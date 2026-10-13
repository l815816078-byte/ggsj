package bot

import (
	"app/assets"
	"app/core"
	imageHandler "app/imageHandler"
	"app/plugin"
	"app/task"
	"io/fs"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Bot 主控制器
// 组装状态机、调度器、监控器、异常管理器，统一管理生命周期
type Bot struct {
	sm        *StateMachine
	scheduler *Scheduler
	monitor   *Monitor
	loader    *task.Loader
	exception *ExceptionManager
	recovCtx  *RecoveryContext

	config BotConfig

	// 信号
	stopCh chan struct{}
	doneCh chan struct{}

	// 统计
	startTime time.Time
}

// BotConfig Bot 配置
type BotConfig struct {
	// 任务配置目录
	TaskDir string

	// 心跳间隔
	HeartbeatInterval time.Duration
	// 心跳超时（超时触发全量重启）
	HeartbeatTimeout time.Duration

	// 任务默认超时
	TaskTimeout time.Duration

	// 调度间隔
	TickInterval time.Duration
}

// DefaultBotConfig 默认配置
func DefaultBotConfig() BotConfig {
	return BotConfig{
		TaskDir:           "assets/tasks",
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  90 * time.Second,
		TaskTimeout:       5 * time.Minute,
		TickInterval:      2 * time.Second,
	}
}

// NewBot 创建 Bot
func NewBot(cfg BotConfig) *Bot {
	sm := NewStateMachine()
	taskFS, err := fs.Sub(assets.Tasks, "tasks")
	if err != nil {
		core.LogWithTimef("[Bot] 嵌入任务目录读取失败: %v", err)
	}
	loader := task.NewEmbeddedLoader(cfg.TaskDir, taskFS)

	b := &Bot{
		sm:        sm,
		loader:    loader,
		config:    cfg,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		startTime: time.Now(),
	}

	// 创建调度器
	b.scheduler = NewScheduler(loader, sm)
	b.scheduler.OnTaskComplete(b.onTaskComplete)
	b.scheduler.OnTaskError(b.onTaskError)

	return b
}

// Init 初始化 Bot
func (b *Bot) Init() error {
	core.LogWithTimef("[Bot] ===== 初始化 =====")

	// 加载字库（OCR 必须）
	if !imageHandler.SetDict("ziku.txt", "library") {
		core.LogWithTimef("[Bot] 警告: 字库加载失败，OCR 功能可能不可用")
	}

	// 加载任务
	_, err := b.loader.LoadAll()
	if err != nil {
		core.LogWithTimef("[Bot] 任务加载失败: %v", err)
		return err
	}

	// 创建监控器
	b.monitor = NewMonitor(
		b.config.HeartbeatInterval,
		b.config.HeartbeatTimeout,
		b.onHeartbeatTimeout,
	)

	// 创建恢复上下文
	b.recovCtx = NewRecoveryContext(b.loader)

	// 创建异常管理器
	b.exception = NewExceptionManager(b.sm, b.scheduler, b.monitor, b.recovCtx)

	// 注入"插件内执行配置化任务"的实现（plugin 不能 import task，只能反向注入）
	// 这样插件里 plugin.RunTask("背包物品使用") 就能直接跑 assets/tasks/*.json
	plugin.SetRunTaskFunc(b.scheduler.RunTaskNow)
	// 注入"插件内直接跑一遍任务流水线（子任务）"的实现。
	// 与 RunTask 的区别：不走调度器、不写任务状态/冷却/HUD，只在调用方(外层任务)的
	// goroutine 里同步跑完；并共享外层任务的取消令牌，外层一结束子任务立刻跟着停。
	// 只是复用 JSON 的步骤轮转 + 流水线内嵌弹窗能力，不把它当"任务"。
	plugin.SetRunSubTaskFunc(func(name string) bool {
		t := b.loader.Get(name)
		if t == nil {
			core.LogWithTimef("[Bot] 子任务'%s' 不存在（检查 assets/tasks/*.json 的\"名称\"）", name)
			return false
		}
		engine := task.NewPipelineEngine(t.Config())
		engine.SetCancel(plugin.CurrentCancel()) // 共享外层任务的取消令牌
		r := engine.Execute(&task.RunState{ExecutedCounts: make(map[string]int)})
		return r.Error == nil
	})
	// 注入"插件内清除指定任务冷却"的实现（plugin 不能 import task，只能反向注入）
	plugin.SetResetTaskCooldownFunc(b.scheduler.ResetTaskCooldown)
	// 注入"插件内清一次任务轮转弹窗"的实现（plugin 不能 import task，只能反向注入）
	// 临时建一个空引擎，ClearAllPopups 只用全局 taskRotatePopups + 动作执行，不依赖引擎配置
	plugin.SetClearPopupsFunc(func() int {
		engine := task.NewPipelineEngine(task.TaskConfig{Name: "弹窗清扫"})
		return engine.ClearAllPopups(0)
	})
	// 注入"插件内触发全量重启"的实现（如"连线失败"弹窗 → 重启 kit+游戏）
	plugin.SetFullRestartFunc(b.exception.TriggerFullRestart)

	// ── 中控挂钩 ──
	// 1) 上报回调：识别到等级/战力等值有变化时，client/wsclient 负责发送上报
	core.SetReportHook(func(field string) {
		if wsReportHook != nil {
			wsReportHook(field)
		}
	})

	core.LogWithTimef("[Bot] 初始化完成, 已加载 %d 个任务", len(b.loader.All()))
	return nil
}

// ── 中控命令注入 ──
// bot 不能反向 import client（会形成环），所以 client 在启动时把自己的函数注册进来。

var (
	wsReportHook func(field string)
)

// SetWSReportHook 注入中控上报实现（由 client/wsclient 调用）
func SetWSReportHook(f func(field string)) {
	wsReportHook = f
}

// ResetScript 重置脚本：清除所有任务的完成状态与冷却，让任务重新可以调度。
// 由中控"重置脚本"命令触发。
func (b *Bot) ResetScript() {
	core.LogWithTimef("[Bot] 中控指令：重置脚本")
	for _, t := range b.loader.All() {
		t.Reset()
	}
	// 清掉停止标志，让任务可继续跑
	plugin.SetStopped(false)
	core.SetStopped(false)
	b.sm.Resume()
	b.scheduler.Resume()
}

// Run 启动 Bot 主循环（阻塞直到 Stop）
func (b *Bot) Run() {
	b.sm.GoRunning()
	b.startTime = time.Now()

	core.LogWithTimef("[Bot] ===== 启动 =====")

	// 启动监控（心跳看门狗）——已停用：只保留卡密心跳，本地看门狗无喂食会 90s 误触发全量重启
	// b.monitor.Start()

	// 启动异常检测 + 恢复
	b.exception.Start()

	// 启动时先登录（临时注释：真机 system.GetPid 触发 Illegal instruction，先跳过启动登录/起kit流程排查）
	// b.doStartupLogin()

	// 启动调度
	b.scheduler.Start()

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		core.LogWithTimef("[Bot] 收到信号: %v, 开始停止...", sig)
	case <-b.stopCh:
		core.LogWithTimef("[Bot] 收到停止指令...")
	}

	b.shutdown()
}

// Stop 异步停止
func (b *Bot) Stop() {
	// 同步 core 的停止标志，供中控上报 state=stopped
	core.SetStopped(true)
	close(b.stopCh)
}

// Pause 暂停
func (b *Bot) Pause(reason string) {
	b.sm.Pause(reason)
	b.scheduler.Pause()
}

// Resume 恢复
func (b *Bot) Resume() {
	if b.sm.Resume() {
		b.scheduler.Resume()
	}
}

// shutdown 内部停机流程
func (b *Bot) shutdown() {
	core.LogWithTimef("[Bot] ===== 停止中 =====")

	// 先置全局停止标志：所有正在执行的任务/插件循环立即退出，不再滑动/跳跃/转圈
	plugin.SetStopped(true)

	b.sm.Stop()

	// 先停异常检测（不再触发恢复）
	if b.exception != nil {
		b.exception.Stop()
	}

	// 停调度
	b.scheduler.Stop()

	// 停监控（monitor.Start 已停用，此处 Stop 会等 doneCh 永久阻塞，一并停用）
	// if b.monitor != nil {
	// 	b.monitor.Stop()
	// }

	elapsed := time.Since(b.startTime).Round(time.Second)
	core.LogWithTimef("[Bot] ===== 已停止 (运行时间: %v) =====", elapsed)
	// 打印恢复统计
	core.LogWithTimef("[Bot] 恢复统计: 全量重启 %d 次", b.exception.Stats())

	close(b.doneCh)
}

// Wait 等待 Bot 完全停止
func (b *Bot) Wait() {
	<-b.doneCh
}

// doStartupLogin 启动时自动登录: 先按 kit 那套启动代理(与 L3 一样), 再跑登录任务
func (b *Bot) doStartupLogin() {
	b.sm.StartRecover("startup login")

	ok := b.recovCtx.StartupLogin()
	if ok {
		core.LogWithTimef("[Bot] 启动登录成功")
		b.sm.RecoverOK()
	} else {
		core.LogWithTimef("[Bot] 启动登录失败，继续运行（等待异常恢复重试）")
		b.sm.RecoverOK()
	}
}

// ───── 回调 ─────

// onTaskComplete 任务完成回调
func (b *Bot) onTaskComplete(t *task.RuntimeTask) {
	// 通知异常管理器（空闲检测器记录心跳）
	b.exception.Beat(t.Name())
	// 重置任务，下次可以再跑
	t.Reset()
}

// onTaskError 任务错误回调
func (b *Bot) onTaskError(t *task.RuntimeTask, err error) {
	core.LogWithTimef("[Bot] 任务 %s 出错: %v", t.Name(), err)
	// 通知异常管理器（可用来自动决定是否需要恢复）
	b.exception.Beat(t.Name())
	// 普通错误，重置重试
	t.Reset()
}

// onHeartbeatTimeout 心跳超时 → 全量重启
func (b *Bot) onHeartbeatTimeout() {
	core.LogWithTimef("[Bot] 心跳超时! 触发全量重启")

	if b.exception != nil {
		b.exception.TriggerFullRestart("心跳超时")
	}
}

// ───── 查询接口 ─────

// StateMachine 获取状态机（外部监控用）
func (b *Bot) StateMachine() *StateMachine { return b.sm }

// Scheduler 获取调度器
func (b *Bot) Scheduler() *Scheduler { return b.scheduler }

// MonitorHelper 获取监控器
func (b *Bot) MonitorHelper() *Monitor { return b.monitor }

// Exception 获取异常管理器
func (b *Bot) Exception() *ExceptionManager { return b.exception }

// Beat 记录心跳（供外部定时调用）
func (b *Bot) Beat() {
	if b.monitor != nil {
		b.monitor.Beat()
	}
}

// Uptime 运行时长
func (b *Bot) Uptime() time.Duration {
	return time.Since(b.startTime)
}
