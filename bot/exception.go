package bot

import (
	"app/bot/detector"
	"app/core"
	"app/plugin"
	"app/task"
	"sync"
	"time"
)

// ExceptionManager 异常管理器
// 串联检测器 → 统一触发全量重启（L3）：kill 代理+游戏 → 重新拉起 → 登录
type ExceptionManager struct {
	sm         *StateMachine
	detMgr     *detector.Manager
	recoverCtx *RecoveryContext
	scheduler  *Scheduler
	monitor    *Monitor

	mu sync.Mutex

	// 统计
	recoverCount  int
	lastRecoverAt time.Time

	// 冷却：连续恢复之间有间隔
	recoverCooldown time.Duration
}

// NewExceptionManager 创建异常管理器
func NewExceptionManager(sm *StateMachine, scheduler *Scheduler, monitor *Monitor, recovCtx *RecoveryContext) *ExceptionManager {
	return &ExceptionManager{
		sm:              sm,
		scheduler:       scheduler,
		monitor:         monitor,
		recoverCtx:      recovCtx,
		recoverCooldown: 30 * time.Second,
	}
}

// Start 启动异常检测
func (em *ExceptionManager) Start() {
	// 加载检测器 JSON 配置
	cfg := detector.LoadConfig()

	// 弹窗配置 → 转为全局 StepConfig，注入到所有任务流水线
	task.SetGlobalPopups(convertPopupScenesToSteps(&cfg.Popups))

	// 任务轮转弹窗 → 独立注入，只用于任务开始前的清扫，不进流水线
	task.SetTaskRotatePopups(convertPopupScenesToSteps(&cfg.TaskRotate))

	// 创建检测管理器
	em.detMgr = detector.NewManager(em.onExceptionDetected)

	// 注册检测器（弹窗不再走全局异常，改为流水线内嵌处理）
	// 每个检测器都按 JSON 的"启用"字段决定是否真正参与调度
	em.detMgr.Register(detector.NewProcessDetector(
		cfg.Process.Enabled,
		time.Duration(cfg.Process.IntervalSec)*time.Second,
	))
	em.detMgr.Register(detector.NewIdleDetector(
		cfg.Idle.Enabled,
		time.Duration(cfg.Idle.IntervalSec)*time.Second,
		time.Duration(cfg.Idle.ThresholdSec)*time.Second,
	))
	em.detMgr.Register(detector.NewFreezeDetector(&cfg.Freeze))

	em.detMgr.Start()
	core.LogWithTimef("[Exception] 异常检测已启动 (进程=%v 空闲=%v 冻结=%v)",
		cfg.Process.Enabled, cfg.Idle.Enabled, cfg.Freeze.Enabled)
}

// Stop 停止异常检测
func (em *ExceptionManager) Stop() {
	if em.detMgr != nil {
		em.detMgr.Stop()
	}
}

// Beat 通知空闲检测器任务正在执行
func (em *ExceptionManager) Beat(taskName string) {
	// 通知 idle detector
	// 通过 manager 的 detectors 找到 IdleDetector
	for _, d := range em.detMgr.AllDetectors() {
		if idle, ok := d.(*detector.IdleDetector); ok {
			idle.Beat(taskName)
		}
	}
}

// onExceptionDetected 检测器回调：一律全量重启
func (em *ExceptionManager) onExceptionDetected(d detector.Detected) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 冷却检查
	if time.Since(em.lastRecoverAt) < em.recoverCooldown {
		core.LogWithTimef("[Exception] %s 触发过于频繁，跳过", d.DetectorName)
		return
	}

	core.LogWithTimef("[Exception] %s → 全量重启", d.DetectorName)
	core.SetStatusColor("#FFAA00", "检测到异常: %s → 全量重启", d.DetectorName)

	// 执行恢复
	go em.executeRecovery(d.DetectorName)
}

// TriggerFullRestart 手动触发全量重启（如心跳超时）
func (em *ExceptionManager) TriggerFullRestart(reason string) {
	em.onExceptionDetected(detector.Detected{DetectorName: reason})
}

// executeRecovery 执行全量恢复流程：kill 代理+游戏 → 重新拉起 → 登录
func (em *ExceptionManager) executeRecovery(reason string) {
	em.recoverCount++
	em.lastRecoverAt = time.Now()

	em.sm.StartRecover(reason)

	// ① 先断开正在执行的任务：置全局停止标志（引擎/插件在检查点会退出）+ 停调度。
	// 否则会出现"这边全量重启、那边任务还在点旧画面"两条 goroutine 并行操作同一界面的问题。
	plugin.SetStopped(true)
	em.scheduler.Pause()
	core.RandomSleep(300, 600) // 给当前任务走到检查点退出的时间

	// ② 立刻清掉标志：FullRestart 内部的 doLogin 也用引擎，若标志一直为 true 会一进就退出
	plugin.SetStopped(false)

	// ③ 跑恢复（此时无任务在跑）
	ok := em.recoverCtx.FullRestart()

	// ④ 恢复调度
	em.scheduler.Resume()

	if ok {
		em.sm.RecoverOK()
		core.LogWithTimef("[Exception] 全量重启 恢复成功")
		core.SetStatusColor("#00FF00", "全量重启 恢复成功")
	} else {
		core.LogWithTimef("[Exception] 全量重启 恢复失败")
		core.SetStatusColor("#FF5555", "全量重启 恢复失败")
	}
}

// Stats 统计信息
func (em *ExceptionManager) Stats() int {
	em.mu.Lock()
	defer em.mu.Unlock()
	return em.recoverCount
}

// convertPopupScenesToSteps 将 detectors.json 弹窗配置转为流水线 StepConfig
// 合并高优先级 + 普通场景；识别方式由"识别引用"统一决定，动作默认"点击"（配了"滑动"则走滑动）
func convertPopupScenesToSteps(cfg *detector.PopupConfig) []task.StepConfig {
	all := make([]detector.PopupScene, 0, len(cfg.HighPriority)+len(cfg.Scenes))
	all = append(all, cfg.HighPriority...)
	all = append(all, cfg.Scenes...)

	steps := make([]task.StepConfig, 0, len(all))
	for _, s := range all {
		// 插件模式弹窗：名称直接用配置里的名称（即插件注册名，如"弹窗_判断等级复活"），
		// 不加"弹窗_"前缀、不设动作/识别字段，由引擎按名称查插件处理
		if s.Plugin {
			steps = append(steps, task.StepConfig{Name: s.Name, Plugin: true})
			continue
		}

		sc := task.StepConfig{
			Name:        "弹窗_" + s.Name,
			Region:      s.Region,
			Sim:         s.Sim,
			FontName:    s.FontName,
			Count:       s.Count,
			Direction:   s.Direction,
			ClickArea:   s.ClickArea,
			ClickOffset: s.ClickOffset,
			SwipeDelta:  s.SwipeDelta,
			WaitAfter:   s.WaitAfter * 1000, // 秒 → 毫秒（引擎直接用毫秒）
		}

		// 动作：默认点击，显式配"滑动"才走滑动（如省电模式需滑动关闭）
		sc.Action = s.Action
		if sc.Action == "" {
			sc.Action = task.ActionClick
		}
		switch s.Type {
		case "文字":
			sc.Text = s.Text
		case "找色":
			sc.Colors = s.Colors
			sc.ColorMulti = s.ColorMulti
		default: // "图片" 或空
			if s.Name != "" {
				sc.Images = []string{"scene_" + s.Name + ".png"}
			}
		}

		// 识别引用：保留引用，合并推迟到引擎实际执行该弹窗时（避免启动时查预设库）
		if s.RecogRef != "" {
			sc.RecogRef = s.RecogRef
		}
		steps = append(steps, sc)
	}
	return steps
}
