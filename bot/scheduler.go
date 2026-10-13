package bot

import (
	"app/core"
	"app/plugin"
	"app/task"
	"context"
	"fmt"
	"strings"
	"time"
)

// Scheduler 任务调度器
// 按优先级（level）循环调度任务
type Scheduler struct {
	loader *task.Loader
	sm     *StateMachine

	// 调度间隔
	tickInterval time.Duration
	// 任务超时保护
	taskTimeout time.Duration

	// 控制信号
	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}
	paused   bool

	// 回调
	onTaskComplete func(t *task.RuntimeTask)
	onTaskError    func(t *task.RuntimeTask, err error)

	// running 当前正在执行的任务（单飞锁）。
	// executeTask 开头置上，确认引擎 goroutine 真正退出后才清空；
	// 非空时 tick / RunTaskNow 都不再取新任务，保证同一时刻只有一个引擎在操作屏幕。
	running *task.RuntimeTask
}

// NewScheduler 创建调度器
func NewScheduler(loader *task.Loader, sm *StateMachine) *Scheduler {
	return &Scheduler{
		loader:       loader,
		sm:           sm,
		tickInterval: 2 * time.Second,
		taskTimeout:  30 * time.Minute,
		pauseCh:      make(chan struct{}, 1),
		resumeCh:     make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// OnTaskComplete 设置任务完成回调
func (s *Scheduler) OnTaskComplete(fn func(t *task.RuntimeTask)) {
	s.onTaskComplete = fn
}

// OnTaskError 设置任务错误回调
func (s *Scheduler) OnTaskError(fn func(t *task.RuntimeTask, err error)) {
	s.onTaskError = fn
}

// Start 启动调度循环
func (s *Scheduler) Start() {
	go s.loop()
	core.LogWithTimef("[Scheduler] 调度器已启动 (tick=%v)", s.tickInterval)
}

// Stop 停止调度
// 注意：不等待 doneCh —— tick() 是同步阻塞的（等任务结果/超时），
// 等待会卡死整个停止流程，导致任务继续执行。直接返回，进程退出时残留 goroutine 自然终止。
func (s *Scheduler) Stop() {
	close(s.stopCh)
	core.LogWithTimef("[Scheduler] 调度器已停止")
}

// Pause 暂停调度（不杀当前任务，但不再取新任务）
func (s *Scheduler) Pause() {
	select {
	case s.pauseCh <- struct{}{}:
	default:
	}
}

// Resume 恢复调度
func (s *Scheduler) Resume() {
	select {
	case s.resumeCh <- struct{}{}:
	default:
	}
}

// TickInterval 设置调度间隔（调试用）
func (s *Scheduler) TickInterval(d time.Duration) {
	s.tickInterval = d
}

// loop 主调度循环
func (s *Scheduler) loop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return

		case <-s.pauseCh:
			s.paused = true
			core.LogWithTimef("[Scheduler] 已暂停")

		case <-s.resumeCh:
			s.paused = false
			core.LogWithTimef("[Scheduler] 已恢复")

		case <-ticker.C:
			if s.paused {
				continue
			}
			if !s.sm.CanRun() {
				continue
			}
			// 每轮调度都打印所有自动任务状态（冷却/执行中/可执行），持续可见
			s.logStatusSummary()
			s.tick()
		}
	}
}

// tick 单次调度（自动）
func (s *Scheduler) tick() {
	// 上一个任务还没彻底退出 → 不取新任务（单飞：同一时刻只允许一个引擎在跑）
	if s.running != nil {
		return
	}
	// 从加载器取出最高优先级可用任务（只挑 JSON 里"自动调度":true 的）
	t := s.loader.PickNext()
	if t == nil {
		return
	}
	s.executeTask(t)
}

// RunTaskNow 手动立即执行一次指定任务（供插件通过 plugin.RunTask 调用）
//
//	name             任务名（对应 assets/tasks/*.json 的"名称"）
//	cooldownAfterSec 执行完后的冷却秒数（会覆盖 JSON 的"冷却秒"，两值取其一不是取更久）:
//	    >0 → 冷却这么多秒（内部会清零 lastRunAt，让 JSON 的"冷却秒"失效）
//	    =0 → 不冷却，下次可立即再跑
//	    <0 或不传 → 沿用 JSON 配的"冷却秒"（默认）
//
// 返回是否执行成功。任务不存在 / 正在运行 / 冷却中时返回 false。
// 冷却中会被拦截：调用方（如打怪循环）每轮都调也没关系，冷却期内自动跳过。
// 与自动调度互不干扰: PickNext 只挑 auto 任务, 这里按名直接取, 所以
// "自动调度":false 的任务也能手动触发; 配 true 则自动+手动都可用（手动会检查
// 该任务是否正在运行, 避免两个引擎同时操作同一个 RunState）。
func (s *Scheduler) RunTaskNow(name string, cooldownAfterSec int) bool {
	t := s.loader.Get(name)
	if t == nil {
		core.LogWithTimef("[Scheduler] 手动执行失败: 任务'%s' 不存在", name)
		return false
	}
	if t.State() == task.TaskRunning {
		core.LogWithTimef("[Scheduler] 手动执行跳过: 任务'%s' 正在运行中", name)
		return false
	}
	// 单飞：已经有别的任务在执行（哪怕它已超时、正在等引擎退出）→ 一律不接受新任务
	if s.running != nil && s.running != t {
		core.LogWithTimef("[Scheduler] 手动执行跳过: 任务'%s' 正在执行中, 此时不接受'%s'", s.running.Name(), name)
		return false
	}
	// 可运行性检查（冷却/时间窗口/等级/战力）：core.ExtendCooldown 设的强制冷却只有
	// CanRun() 会查，而 CanRun 只有 PickNext 自动调度在用，手动路径必须自己查一次，
	// 否则上次的冷却对手动调用无效，导致打怪循环每轮都重复跑一遍（还会同步阻塞打怪检测）。
	// 注意别用 BlockReason 判空 —— 它全部通过时返回的是字符串"可运行"(非空)，
	// 不是空串，按"非空即阻塞"判断会把可执行状态误判为阻塞。
	if !t.CanRun() {
		core.LogWithTimef("[Scheduler] 手动执行跳过: 任务'%s' 当前不可运行 (%s)", name, t.BlockReason(time.Now()))
		return false
	}

	core.LogWithTimef("[Scheduler] 手动执行任务: %s", name)
	ok := s.executeTask(t)

	// 手动执行后设置冷却
	//
	// 注意: executeTask 内部会 SetLastRun(now)，使 JSON 的"冷却秒"开始计时；
	// 而 BlockReason 先查 config.Cooldown 再查强制冷却，两者取"更久的那个"。
	// 所以显式传了冷却值时必须把 lastRunAt 清零，否则手动指定的短冷却会被
	// JSON 配置的长冷却盖掉（如传 300 但 JSON 配 600，实际要等 600 秒）。
	switch {
	case cooldownAfterSec > 0:
		t.SetLastRun(time.Time{}) // 让 JSON 的"冷却秒"失效，以手动指定的为准
		core.ExtendCooldownWithReason(name, time.Now().Add(time.Duration(cooldownAfterSec)*time.Second), "手动指定")
		core.LogWithTimef("[Scheduler] 手动执行完成: %s → 冷却 %d 秒（覆盖配置 %v）", name, cooldownAfterSec, t.Cooldown())
	case cooldownAfterSec == 0:
		// 清零 = 冷却已过，下次可立即再跑
		t.SetLastRun(time.Time{})
		core.ExtendCooldown(name, time.Time{}) // 顺带清掉可能存在的强制冷却
		core.LogWithTimef("[Scheduler] 手动执行完成: %s → 不额外冷却", name)
	default:
		core.LogWithTimef("[Scheduler] 手动执行完成: %s → 沿用配置冷却 %v", name, t.Cooldown())
	}
	return ok
}

// ResetTaskCooldown 清除指定任务的冷却，让调度器下一轮即可选中它（供插件"检测类型"调用）
//
// 只清冷却、不立即执行：任务是否跑起来交给自动调度(PickNext)下一轮 tick；
// 目标任务没配"自动调度":true 的话，清冷却只对手动 RunTask 有意义。
// 清的是两部分：JSON"冷却秒"的计时起点(lastRunAt) + 强制冷却(ExtendCooldown)，两处都清才真正可执行。
//
// 返回 false = 任务名不存在（配置写错），调用方应记日志排查，别每轮重试刷屏。
func (s *Scheduler) ResetTaskCooldown(name string) bool {
	t := s.loader.Get(name)
	if t == nil {
		core.LogWithTimef("[Scheduler] 重置冷却失败: 任务'%s' 不存在（检查 assets/tasks/*.json 的\"名称\"）", name)
		return false
	}
	t.SetLastRun(time.Time{})              // 清 JSON"冷却秒"计时起点 → 冷却已过
	core.ExtendCooldown(name, time.Time{}) // 清强制冷却
	core.LogWithTimef("[Scheduler] 重置冷却: 任务'%s' 冷却已清除 → 下一轮调度即可执行", name)
	return true
}

// executeTask 执行单个任务: 单飞标记 → 引擎执行 → 超时保护 → 等引擎真正退出 → 回调
//
// 自动调度(tick) 与手动触发(RunTaskNow) 共用这一段，返回是否成功。
// 关键约束：本函数 return 之前，engine.Execute 的那个 goroutine 一定已经结束，
// 绝不会留下还在操作屏幕的残留代码（s.running 也只有到那时才清空）。
func (s *Scheduler) executeTask(t *task.RuntimeTask) bool {
	// 单飞锁：置上"当前执行中"，只有确认引擎 goroutine 真正退出后才清空
	s.running = t

	// 标记为运行中 + 刷新最后运行时间（冷却从执行开始算起）
	t.SetState(task.TaskRunning)
	t.SetLastRun(time.Now())
	startTime := time.Now()

	// 更新 HUD 顶部状态：任务开始
	core.SetStatusColor("#00CCFF", "正在执行: %s", t.Name())

	// 构建引擎执行
	engine := task.NewPipelineEngine(t.Config())

	// 绑定本任务的取消令牌：引擎各检查点、插件长循环（plugin.IsTaskCancelled）、
	// 以及共享该令牌的子任务流水线，都看它 → 任务一结束"里面所有的程序"跟着停
	plugin.SetCurrentCancel(engine.CancelToken())

	// 任务开始前无条件清扫弹窗：反复扫到没有弹窗命中为止（弹窗连锁出现，单次扫不干净）。
	// 自动调度(tick) 和手动触发(RunTaskNow) 都走 executeTask，所以两条路径都会清扫。
	// 传 0 = 用 engine 里的 popupSweepMaxRounds 默认轮数，阈值集中在一个地方管。
	sweepStart := time.Now()
	swept := engine.ClearAllPopups(0)
	core.LogWithTimef("[Scheduler] %s 弹窗清扫 %d 次, 耗时 %v",
		t.Name(), swept, time.Since(sweepStart).Round(time.Millisecond))

	// 带超时执行：优先用任务 JSON 的"最大执行秒"，没配才用调度器默认超时
	timeout := s.taskTimeout
	if d := t.MaxDuration(); d > 0 {
		timeout = d
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	core.LogWithTimef("[Scheduler] 开始执行 %s (超时上限%v)", t.Name(), timeout)

	// HUD 显示任务超时进度（已运行/超时上限 秒），每秒刷新一次；任务结束自动清除进度行
	progressStop := make(chan struct{})
	go func() {
		pt := time.NewTicker(1 * time.Second)
		defer pt.Stop()
		for {
			select {
			case <-progressStop:
				core.SetTaskProgress("", 0, 0) // 退出前清除进度行
				return
			case <-pt.C:
				core.SetTaskProgress(t.Name(), time.Since(startTime), timeout)
			}
		}
	}()
	defer close(progressStop)

	doneCh := make(chan *task.ExecResult, 1)
	go func() {
		doneCh <- engine.Execute(t.RunState())
	}()

	// 先判定走哪条结束路径，但**不在这里 return** —— 必须等引擎 goroutine 真正退出
	var result *task.ExecResult
	超时 := false
	select {
	case <-ctx.Done():
		超时 = true
		// 通知引擎里的所有循环（引擎检查点 + 插件长循环 + 子任务流水线）尽快退出
		engine.CancelToken().Store(true)
		core.LogWithTimef("[Scheduler] %s 超时 (elapsed=%v) → 已发取消信号, 等待引擎退出",
			t.Name(), time.Since(startTime))
	case result = <-doneCh:
	}

	// ── 等引擎 goroutine 真正退出 ──
	// 不管是完成/失败/超时哪种结束方式，都必须确认 engine.Execute 已经 return，
	// 否则残留 goroutine 会继续操作屏幕，跟下一个任务的引擎互相干扰（就是"串台"）。
	// doneCh 缓冲为 1，引擎跑完一定发得出来，不会因为这里没人收而卡住泄漏。
	if result == nil {
		等待开始 := time.Now()
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		已告警 := false
		for result == nil {
			select {
			case result = <-doneCh:
			case <-tk.C:
				core.LogWithTimef("[Scheduler] %s 引擎仍未退出（已等%v）, 继续等待",
					t.Name(), time.Since(等待开始).Round(time.Second))
				if !已告警 && time.Since(等待开始) > 120*time.Second {
					已告警 = true
					core.LogWithTimef("[Scheduler] %s 引擎等待超时(120秒) 仍未退出 → 持续等待, 需人工处理", t.Name())
					core.SetStatusColor("#FF5555", "%s 引擎未退出 → 等待人工处理", t.Name())
				}
			}
		}
		core.LogWithTimef("[Scheduler] %s 引擎已退出（等待%v）",
			t.Name(), time.Since(等待开始).Round(time.Second))
	}

	// 引擎已彻底退出 → 解绑取消令牌 + 清空单飞锁。到此才允许跑下一个任务。
	plugin.SetCurrentCancel(nil)
	s.running = nil

	// 任务间隔延时：上一个任务的收尾动画/弹窗还没落定，先等界面回稳再让下一轮调度取新任务
	core.LogWithTimef("[Scheduler] %s 已结束，等待任务间隔延时（3~5 秒）", t.Name())
	core.RandomSleep(3000, 5000)
	core.LogWithTimef("[Scheduler] 任务间隔延时结束 → 可调度下一个任务")

	if 超时 {
		t.SetState(task.TaskFailed)
		core.SetStatusColor("#FF5555", "%s 超时", t.Name())
		if s.onTaskError != nil {
			s.onTaskError(t, ctx.Err())
		}
		return false
	}

	elapsed := time.Since(startTime).Round(time.Millisecond)
	if result.Success {
		core.LogWithTimef("[Scheduler] %s 完成 (elapsed=%v, steps=%d)",
			t.Name(), elapsed, result.ExecutedSteps)
		t.SetState(task.TaskCompleted)
		core.SetStatusColor("#00FF00", "%s 完成 (%v)", t.Name(), elapsed)
		if s.onTaskComplete != nil {
			s.onTaskComplete(t)
		}
		return true
	}
	core.LogWithTimef("[Scheduler] %s 失败 (elapsed=%v, step=%s, err=%v)",
		t.Name(), elapsed, result.LastStep, result.Error)
	t.SetState(task.TaskFailed)
	core.SetStatusColor("#FF5555", "%s 失败: %v", t.Name(), result.Error)
	if s.onTaskError != nil {
		s.onTaskError(t, result.Error)
	}
	return false
}

// logStatusSummary 每轮调度打印所有自动任务状态（执行中/冷却剩余/可执行），只打日志不影响调度
func (s *Scheduler) logStatusSummary() {
	parts := []string{}
	for _, rt := range s.loader.All() {
		if !rt.Config().Auto {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", rt.Name(), rt.BlockReason(time.Now())))
	}
	core.LogWithTimef("[Scheduler] 轮转状态: %s", strings.Join(parts, " | "))
}
