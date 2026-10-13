package task

import (
	"app/core"
	"app/plugin"
	"fmt"
	"sync/atomic"
	"time"
)

// PipelineEngine JSON 管道执行引擎
type PipelineEngine struct {
	config  TaskConfig
	steps   []StepConfig
	stepIdx map[string]int // step name → index
	popups  []StepConfig   // 合并后的弹窗处理步骤（全局 + 任务级）

	started  time.Time // 任务开始时间（仅用于日志显示已用时间，不影响逻辑）
	prevStep string    // 上一次进入 runNode 的步骤名（流转日志显示"上一步"，调试用）

	// cancel 本引擎的取消令牌：调度器结束时置 true，引擎各检查点自查后退出。
	// 每个引擎一个（不是全局），这样全量重启里的登录流程等临时引擎不受影响；
	// 子任务引擎用 SetCancel 共享外层令牌，外层一停子任务跟着停。nil = 永不取消。
	cancel *atomic.Bool
}

// CancelToken 取本引擎的取消令牌（调度器在任务开始时绑给 plugin，供插件自查用）
func (e *PipelineEngine) CancelToken() *atomic.Bool { return e.cancel }

// SetCancel 让本引擎共享另一个引擎的取消令牌（子任务流水线用：外层一停，子任务跟着停）
func (e *PipelineEngine) SetCancel(p *atomic.Bool) { e.cancel = p }

// IsCancelled 本引擎是否已被要求结束（引擎内各检查点自查用）
func (e *PipelineEngine) IsCancelled() bool { return e.cancel != nil && e.cancel.Load() }

// globalPopups 全局弹窗配置，所有引擎实例共享
var globalPopups []StepConfig

// SetGlobalPopups 设置全局弹窗处理步骤（从 detectors.json 加载）
func SetGlobalPopups(p []StepConfig) { globalPopups = p }

// taskRotatePopups 任务轮转弹窗列表（detectors.json「任务轮转」段），所有引擎实例共享
// 与 globalPopups 独立：只用于任务开始前的 ClearAllPopups 清扫，不进流水线。
var taskRotatePopups []StepConfig

// SetTaskRotatePopups 设置任务轮转弹窗列表（从 detectors.json 加载）
func SetTaskRotatePopups(p []StepConfig) { taskRotatePopups = p }

// 任务轮转清扫的三层保底：都不是业务规则，只防异常，达到上限就停、不判失败，调用方照常往下走
const (
	popupSweepMaxRounds  = 8                // 总轮数：单次清扫最多处理多少个弹窗
	popupSweepPerItemMax = 3                // 单项：同一个弹窗项最多处理几次，超了本次清扫内排除它（防常驻元素吃光轮数）
	popupSweepBudget     = 10 * time.Second // 时间：单次清扫的墙钟预算，防 OCR 变慢拖住任务
)

// NewPipelineEngine 创建引擎
func NewPipelineEngine(cfg TaskConfig) *PipelineEngine {
	idx := make(map[string]int, len(cfg.Steps))
	for i, s := range cfg.Steps {
		idx[s.Name] = i
	}
	// 合并：全局弹窗在前，任务级弹窗在后
	popups := make([]StepConfig, 0, len(globalPopups)+len(cfg.Popups))
	popups = append(popups, globalPopups...)
	popups = append(popups, cfg.Popups...)
	/*	for _, p := range popups {
		 core.LogWithTimef("[Engine] %s 弹窗配置: %s 引用=%q 跳过引用=%q 插件=%v", cfg.Name, p.Name, p.RecogRef, p.SkipRecogRefs, p.Plugin)
	}*/
	return &PipelineEngine{
		config:  cfg,
		steps:   cfg.Steps,
		stepIdx: idx,
		popups:  popups,
		cancel:  new(atomic.Bool), // 默认 false：不主动取消，由调度器结束时置 true
	}
}

// ExecResult 执行结果
type ExecResult struct {
	Success       bool
	LastStep      string
	ExecutedSteps int
	Error         error
	RetryNode     bool // 弹窗清掉后要求调用方重试当前节点
}

// Execute 执行管道（MAA 式: next-list 驱动）
// 从第一个步骤开始，每个步骤在自己的 timeout 内循环识别，
// 命中后执行动作并跟随 next 列表跳转。
func (e *PipelineEngine) Execute(rs *RunState) *ExecResult {
	e.started = time.Now()
	deadline := time.Now().Add(time.Duration(e.config.MaxDuration) * time.Second)
	ctx := plugin.NewContext(e.config.Name, "")

	result := &ExecResult{}
	idx := 0
	for idx < len(e.steps) {
		if plugin.IsStopped() || e.IsCancelled() {
			return &ExecResult{}
		}
		// 任务级 deadline：整任务时限已到就不再推进（runNode 进入前也会查一次，
		// 这里再兜一道，防"跳转即 return"的路由把主循环架空）
		if time.Now().After(deadline) {
			return &ExecResult{
				LastStep: e.steps[idx].Name,
				Error:    fmt.Errorf("[%s] 任务超时", e.config.Name),
			}
		}
		step := &e.steps[idx]
		ctx.StepName = step.Name
		rs.CurrentStep = idx

		nr := e.runNode(idx, rs, deadline, ctx, "顺序进入")
		result.ExecutedSteps += nr.ExecutedSteps

		if nr.Error != nil {
			nr.ExecutedSteps = result.ExecutedSteps
			return nr
		}
		if nr.Success {
			nr.ExecutedSteps = result.ExecutedSteps
			return nr
		}
		// 节点完成但未成功 → 继续下一个顺序节点
		idx++
	}

	result.Success = true
	return result
}

// ClearAllPopups 任务开始前无条件清扫弹窗：反复扫描直到没有弹窗命中，返回处理次数。
//
// 无条件 = 不看上次清扫时间、不看任务类型、不跳过任何弹窗项，每次任务执行前都跑一遍。
//
// 扫的是 detectors.json「任务轮转」段（taskRotatePopups），与流水线内嵌的弹窗列表独立。
//
// skip 本次不清的弹窗名单（按"名称"或"识别引用"匹配，留空=全清）。
// 用于"这个场景下某某弹窗是正常界面元素，不能点"的场合，
// 例如落花战场在副本里挂机，屏幕上常驻【退出副本】，清掉会把人从副本踢出去。
//
// 为什么要循环：sweepOnce 命中第一个就返回（一次只处理一个），而弹窗是连锁出现的 ——
// 点掉"获得物品"结果弹窗 → 底下露出"等级提升" → 再点掉 → 可能还有，
// 所以要一直扫到某轮全部未命中（说明清干净了）为止。屏幕干净时只跑 1 轮就结束。
func (e *PipelineEngine) ClearAllPopups(maxRounds int, skip ...string) int {
	if maxRounds <= 0 {
		maxRounds = popupSweepMaxRounds
	}
	if len(taskRotatePopups) == 0 {
		core.LogWithTimef("[Engine] 任务轮转弹窗列表为空 → 跳过清扫")
		return 0
	}
	ban := make(map[string]bool, len(skip))
	for _, s := range skip {
		ban[s] = true
	}
	if len(ban) > 0 {
		core.LogWithTimef("[Engine] 本次清扫跳过: %v", skip)
	}
	exclude := make(map[string]bool)
	hitCount := make(map[string]int)
	deadline := time.Now().Add(popupSweepBudget)
	handled := 0

	for i := 0; i < maxRounds; i++ {
		if plugin.IsStopped() || time.Now().After(deadline) {
			break
		}
		hit := e.sweepOnce(exclude, ban)
		if hit == "" {
			break // 本轮无一命中 → 已清干净
		}
		handled++
		hitCount[hit]++
		if hitCount[hit] >= popupSweepPerItemMax {
			// 处理了这么多次仍在命中，多半是常驻界面元素而非弹窗 →
			// 本次清扫内不再处理它，把剩余轮数留给真正挡路的弹窗
			exclude[hit] = true
			core.LogWithTimef("[Engine] %s 弹窗清扫: 【%s】已处理%d次仍命中 → 本次清扫内不再处理(疑似常驻元素)",
				e.config.Name, hit, hitCount[hit])
		}
	}
	return handled
}

// sweepOnce 扫一遍任务轮转列表，命中第一个就执行动作并返回其名称；全部未命中返回空串
// exclude: 本次清扫内需要跳过的弹窗项（ClearAllPopups 的单项保底用）
// ban: 调用方指定要跳过的弹窗项（按名称或识别引用匹配，本次清扫全程有效）
func (e *PipelineEngine) sweepOnce(exclude map[string]bool, ban map[string]bool) string {
	for i := range taskRotatePopups {
		if plugin.IsStopped() {
			return ""
		}
		p := &taskRotatePopups[i]
		if exclude[p.Name] || ban[p.Name] || ban[p.RecogRef] {
			continue
		}
		var r *core.RecogResult
		if p.Region != [4]int{} {
			r = core.Exec(p.RecogRef, p.Region[0], p.Region[1], p.Region[2], p.Region[3])
		} else {
			r = core.Exec(p.RecogRef) // 区域留空 → 用 recogs.json 预设的区域
		}
		if !r.Found {
			core.LogWithTimef("[Engine] 弹窗清扫: 场景【%s】引用=%s → 未命中", p.Name, p.RecogRef)
			continue
		}
		core.LogWithTimef("[Engine] 弹窗清扫: 场景【%s】引用=%s → 命中 (%d,%d)", p.Name, p.RecogRef, r.X, r.Y)
		e.doAction(p, r.X, r.Y) // 复用引擎动作，点击/滑动都支持
		if p.WaitAfter > 0 && p.Action != ActionSleep {
			core.RandomSleep(p.WaitAfter/2, p.WaitAfter)
		}
		return p.Name
	}
	return ""
}

// HandlePopupsOnce 执行一次本任务的弹窗处理（识别弹窗/插件弹窗），不执行任何主流程步骤。
// 用途：CanRun 等级/战力识别失败时，先走一遍弹窗，把挡住识别界面的弹窗（关闭/菜单收起等）处理掉。
// 弹窗逻辑"识别到才点、识别不到无动作"，安全。
func (e *PipelineEngine) HandlePopupsOnce() {
	rs := &RunState{ExecutedCounts: make(map[string]int)}
	ctx := plugin.NewContext(e.config.Name, "")
	e.checkPopups(rs, time.Now().Add(30*time.Second), ctx)
}

// runNode 执行单个步骤的识别-动作-跳转循环
// 返回 nil error + !success → 调用方应继续下一个顺序节点
// entry 标注本次进入方式（顺序进入/失败跳转/插件跳转等），只用于日志显示
func (e *PipelineEngine) runNode(idx int, rs *RunState, deadline time.Time, ctx *plugin.Context, entry string) *ExecResult {
	step := &e.steps[idx]

	// 进入节点前先看任务级 deadline：整任务时限已过就直接收工，不再进入也不走失败跳转。
	// 没有这一句，"StartUp ↔ 挂机中" 这类"跳转即 return"的路由每进一个新节点都会把
	// tried 重置成 false，循环内那个 "tried && 超时" 永远进不去 → 任务实际永远不会超时。
	// 放在进入处而不是循环里，也避免"超时失败跳转"在节点间来回跳形成递归。
	if time.Now().After(deadline) {
		core.LogWithTimef("[Engine] %s →【%s】任务最大执行秒已到 → 收工", e.config.Name, step.Name)
		return &ExecResult{LastStep: step.Name, Error: fmt.Errorf("[%s] 任务超时", step.Name)}
	}

	// 节点超时截止时间
	nodeDl := deadline
	if step.Timeout > 0 {
		nd := time.Now().Add(time.Duration(step.Timeout) * time.Millisecond)
		if nd.Before(deadline) {
			nodeDl = nd
		}
	}

	tried := false
	// 本次进入该步骤的滑动重试计数（独立于全局 ExecutedCounts，保证每次进入都重新生效，真正滑 RetrySwipe 次）
	swipeCount := 0
	// 步骤流转日志：每次进入（含跳转递归进入）打一条，带上一步/进入方式/已用/剩余时间和下一步去向
	core.LogWithTimef("[Engine] %s →【%s】| 上一步:【%s】| 进入:%s | 已用%v/上限%v | Next:%v",
		e.config.Name, step.Name, e.prevStep, entry,
		time.Since(e.started).Round(time.Second),
		time.Until(deadline).Round(time.Second),
		step.Next)
	e.prevStep = step.Name
	for {
		if rs.Interrupted || plugin.IsStopped() || e.IsCancelled() {
			return &ExecResult{LastStep: step.Name}
		}

		// 重试超时检查
		if tried && time.Now().After(nodeDl) {
			core.LogWithTimef("[Engine] %s →【%s】超时", e.config.Name, step.Name)
			if step.OnErrorNext == NextDone {
				return &ExecResult{Success: true} // 超时跳转 #done → 任务完成 ✅
			}
			if step.OnErrorNext != "" {
				if errIdx, ok := e.stepIdx[step.OnErrorNext]; ok {
					return e.runNode(errIdx, rs, deadline, ctx, "超时失败跳转")
				}
			}
			// 没写失败跳转 → 回 StartUp 继续巡检（不判任务失败）
			if idx, ok := e.stepIdx["StartUp"]; ok && step.Name != "StartUp" {
				return e.runNode(idx, rs, deadline, ctx, "超时回StartUp")
			}
			return &ExecResult{Error: fmt.Errorf("[%s] 超时", step.Name), LastStep: step.Name}
		}

		// 重试间隔
		if tried {
			if step.Timeout <= 0 {
				break // 不重试，跳出
			}
			rl := step.RateLimit
			if rl <= 0 {
				rl = 1000
			}
			core.RandomSleep(rl/2, rl)
		}
		tried = true

		// MaxTimes 检查
		rs.ExecutedCounts[step.Name]++
		if step.MaxTimes > 0 && rs.ExecutedCounts[step.Name] > step.MaxTimes {
			core.LogWithTimef("[Engine] %s →【%s】超过最大次数 %d", e.config.Name, step.Name, step.MaxTimes)
			// 失败跳转：写哪跳哪（#done 完成任务 / 任意节点名跳转）
			if step.OnErrorNext != "" {
				if step.OnErrorNext == NextDone {
					return &ExecResult{Success: true}
				}
				if errIdx, ok := e.stepIdx[step.OnErrorNext]; ok {
					return e.runNode(errIdx, rs, deadline, ctx, "超过次数失败跳转")
				}
				core.LogWithTimef("[Engine] %s →【%s】失败跳转【%s】未找到", e.config.Name, step.Name, step.OnErrorNext)
			}
			break
		}

		// ── 插件步骤 ──
		if step.Plugin {
			nextName := e.execPluginStep(step, ctx)
			if nextName == "" || nextName == NextDone {
				return &ExecResult{Success: true, LastStep: step.Name}
			}
			// 插件返回 "no" = 未命中/无事可做 → 与识别步骤未命中一致：走失败跳转（如回 StartUp 路由器）
			if nextName == "no" {
				if step.OnErrorNext != "" {
					if step.OnErrorNext == NextDone {
						return &ExecResult{Success: true, LastStep: step.Name}
					}
					if errIdx, ok := e.stepIdx[step.OnErrorNext]; ok {
						return e.runNode(errIdx, rs, deadline, ctx, "插件失败跳转")
					}
				}
				return &ExecResult{LastStep: step.Name}
			}
			if jumpIdx, ok := e.stepIdx[nextName]; ok {
				// 插件步骤同样应用"等待秒"（与识别类步骤一致）：跳转前先停
				if step.WaitAfter > 0 {
					core.RandomSleep(step.WaitAfter/2, step.WaitAfter)
				}
				return e.runNode(jumpIdx, rs, deadline, ctx, "插件成功跳转")
			}
			return &ExecResult{Error: fmt.Errorf("[%s] 插件跳转【%s】未找到", step.Name, nextName), LastStep: step.Name}
		}

		// ── 跳过条件：命中任一"跳过识别引用" → 本步骤直接跳过（不识别不点击）──
		if e.matchSkipRecogRefs(step) {
			core.LogWithTimef("[Engine] %s →【%s】命中跳过识别引用 → 跳过本步骤", e.config.Name, step.Name)
			nr := e.processNext(step.Next, rs, deadline, ctx, step.Name)
			nr.ExecutedSteps++
			nr.LastStep = step.Name
			if nr.Success || nr.Error != nil {
				return nr
			}
			if nr.RetryNode { // 弹窗清掉 → 重试当前节点
				continue
			}
			// 无成功跳转命中 → 跳过视为完成，继续下一顺序节点
			return &ExecResult{LastStep: step.Name}
		}

		// ── 识别 ──
		var x, y int
		var matched bool
		if e.hasRecognition(step) {
			x, y, matched = e.doRecognition(step, false)
		} else {
			matched = true // 纯动作步骤
		}

		if !matched {
			// 识别失败 → 先扫一遍弹窗（识别不到通常是弹窗挡路），清掉就重试当前节点
			core.LogWithTimef("[Engine] %s →【%s】未识别到 → 扫弹窗", e.config.Name, step.Name)
			if e.checkPopups(rs, deadline, ctx) != nil {
				// 弹窗插件可能已经置了取消令牌（如"复活"判定当前任务该收工）→ 直接退出，
				// 不再打"弹窗已清 → 重试当前节点"（那条日志会让人误以为还在正常重试）
				if plugin.IsStopped() || e.IsCancelled() || time.Now().After(deadline) {
					return &ExecResult{}
				}
				core.LogWithTimef("[Engine] %s →【%s】弹窗已清 → 重试当前节点", e.config.Name, step.Name)
				continue // 弹窗清掉 → 重试当前节点识别
			}
			core.LogWithTimef("[Engine] %s →【%s】无弹窗可清", e.config.Name, step.Name)
			// 未命中滑动重试：没找到就转圈滑动一次再找，本轮最多滑 RetrySwipe 次
			// 滑动模仿找光柱转圈（alignLightPillar）：以滑动范围中心为基准，水平左右各100px，时长800ms
			if e.swipeRetry(step, &swipeCount) {
				continue
			}
			if step.Timeout <= 0 {
				core.LogWithTimef("[Engine] %s →【%s】未识别到", e.config.Name, step.Name)
				if step.OnErrorNext == NextDone {
					return &ExecResult{Success: true} // 失败跳转 #done → 任务完成 ✅
				}
				if step.OnErrorNext != "" {
					if errIdx, ok := e.stepIdx[step.OnErrorNext]; ok {
						return e.runNode(errIdx, rs, deadline, ctx, "未识别失败跳转")
					}
				}
				// 没写失败跳转 → 回 StartUp 继续巡检（不判任务失败）
				if idx, ok := e.stepIdx["StartUp"]; ok && step.Name != "StartUp" {
					return e.runNode(idx, rs, deadline, ctx, "未识别回StartUp")
				}
				return &ExecResult{Error: fmt.Errorf("[%s] 未识别", step.Name), LastStep: step.Name}
			}
			continue // 有 timeout → 重试
		}

		// ── 执行动作 ──
		e.doAction(step, x, y)
		if step.WaitAfter > 0 && step.Action != ActionSleep {
			core.RandomSleep(step.WaitAfter/2, step.WaitAfter)
		}

		// ── 跟随 next 列表 ──
		nr := e.processNext(step.Next, rs, deadline, ctx, step.Name)
		nr.ExecutedSteps++
		nr.LastStep = step.Name

		if nr.Success || nr.Error != nil {
			return nr
		}
		if nr.RetryNode { // 弹窗清掉 → 重试当前节点
			continue
		}
		// next 列表无命中 → 继续当前步骤的 retry 循环
		continue
	}

	// 跳出循环 → 顺序下一个
	return &ExecResult{LastStep: step.Name}
}

// processNext 处理 next 列表: 逐一尝试，首个命中即跳转
// 返回 nil error + !success → 无命中，调用方继续 retry
// parentName 仅用于巡检日志显示（如 StartUp），标明子节点属于哪个父节点
func (e *PipelineEngine) processNext(next NextList, rs *RunState, deadline time.Time, ctx *plugin.Context, parentName string) *ExecResult {
	// 注意：弹窗不再在此处插队扫描。改为"识别失败时兜底"——
	// runNode 未命中、或本函数子节点未命中时，才扫一次弹窗（见对应分支）。
	if len(next) == 0 {
		return &ExecResult{} // 空列表 → 回到父循环
	}

	for _, name := range next {
		if name == NextDone {
			return &ExecResult{Success: true} // 任务完成 ✅
		}
		if name == NextBack || name == NextSelf {
			return &ExecResult{} // 回到父循环
		}

		childIdx, ok := e.stepIdx[name]
		if !ok {
			core.LogWithTimef("[Engine] %s next 步骤未找到: 【%s】", e.config.Name, name)
			continue
		}

		child := &e.steps[childIdx]

		// ── 插件子节点：走插件执行，不走识别-动作 ──
		if child.Plugin {
			nextName := e.execPluginStep(child, ctx)
			if nextName == "" || nextName == NextDone {
				core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| ok | 任务完成", e.config.Name, parentName, child.Name)
				return &ExecResult{Success: true, LastStep: child.Name}
			}
			// 插件返回 "no" = 未命中/无事可做。与 runNode 插件分支一致: 配了"失败跳转"就走它
			// (如 按名使用物品 no → 关闭背包 立即收尾, 不等巡检轮数耗尽空转); 没配才继续尝试下一个子节点
			if nextName == "no" {
				if child.OnErrorNext != "" {
					if child.OnErrorNext == NextDone {
						return &ExecResult{Success: true, LastStep: child.Name}
					}
					if errIdx, ok := e.stepIdx[child.OnErrorNext]; ok {
						core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| no | 失败跳转:【%s】", e.config.Name, parentName, child.Name, child.OnErrorNext)
						return e.runNode(errIdx, rs, deadline, ctx, "子节点插件失败跳转")
					}
				}
				core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| no | 继续下一个", e.config.Name, parentName, child.Name)
				continue
			}
			if jumpIdx, ok := e.stepIdx[nextName]; ok {
				core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| ok | 跳转:【%s】", e.config.Name, parentName, child.Name, nextName)
				// 插件子节点同样应用"等待秒"（与识别类子节点一致）：跳转前先停
				if child.WaitAfter > 0 {
					core.RandomSleep(child.WaitAfter/2, child.WaitAfter)
				}
				return e.runNode(jumpIdx, rs, deadline, ctx, "子节点插件成功跳转")
			}
			core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| ok | 跳转:【%s】未找到", e.config.Name, parentName, child.Name, nextName)
			continue
		}

		// 跳过条件：命中任一"跳过识别引用" → 跳过该子节点，尝试 next 列表下一项
		if e.matchSkipRecogRefs(child) {
			core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| 被跳过 | 尝试下一个", e.config.Name, parentName, child.Name)
			continue
		}

		var cx, cy int
		var cMatched bool

		if e.hasRecognition(child) {
			cx, cy, cMatched = e.doRecognition(child, false)
		} else {
			cMatched = true
		}

		if cMatched {
			core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| 命中 | %s", e.config.Name, parentName, child.Name, actionDesc(child))
			e.doAction(child, cx, cy)
			if child.WaitAfter > 0 && child.Action != ActionSleep {
				core.RandomSleep(child.WaitAfter/2, child.WaitAfter)
			}
			rs.ExecutedCounts[child.Name]++

			// 递归跟随 child 的 next
			childResult := e.processNext(child.Next, rs, deadline, ctx, child.Name)
			if childResult.Success || childResult.Error != nil {
				return childResult
			}
			if childResult.RetryNode { // 弹窗清掉 → 向上传递，让父节点重试
				return childResult
			}
			// child 完成但未 #done → 继续尝试 next 列表中后续项
			continue
		}
		// 子节点未命中 → 先扫一遍弹窗（可能是弹窗挡路导致识别不到），清掉就让父节点重巡检
		core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】未识别到 → 扫弹窗", e.config.Name, parentName, child.Name)
		if e.checkPopups(rs, deadline, ctx) != nil {
			if plugin.IsStopped() || time.Now().After(deadline) {
				return &ExecResult{}
			}
			core.LogWithTimef("[Engine] %s 弹窗已清 → 父节点重巡检", e.config.Name)
			return &ExecResult{RetryNode: true}
		}
		core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】无弹窗可清", e.config.Name, parentName, child.Name)
		// 未命中：配了未命中滑动次数 → 转圈滑动重试后重新识别
		if child.RetrySwipe > 0 {
			swipeCount := 0
			for e.swipeRetry(child, &swipeCount) {
				cx, cy, cMatched = e.doRecognition(child, false)
				if cMatched {
					break
				}
			}
			if cMatched {
				core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| 滑动后命中 | %s", e.config.Name, parentName, child.Name, actionDesc(child))
				e.doAction(child, cx, cy)
				if child.WaitAfter > 0 && child.Action != ActionSleep {
					core.RandomSleep(child.WaitAfter/2, child.WaitAfter)
				}
				rs.ExecutedCounts[child.Name]++
				childResult := e.processNext(child.Next, rs, deadline, ctx, child.Name)
				if childResult.Success || childResult.Error != nil {
					return childResult
				}
				if childResult.RetryNode { // 弹窗清掉 → 向上传递
					return childResult
				}
				continue
			}
		}
		// 未命中：配了失败跳转 → 写哪跳哪（#done 完成任务 / 任意节点名跳转）
		if child.OnErrorNext != "" {
			if child.OnErrorNext == NextDone {
				return &ExecResult{Success: true} // 失败跳转 #done → 任务完成 ✅
			}
			if errIdx, ok := e.stepIdx[child.OnErrorNext]; ok {
				core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| 未命中 | 失败跳转:【%s】", e.config.Name, parentName, child.Name, child.OnErrorNext)
				return e.runNode(errIdx, rs, deadline, ctx, "子节点失败跳转")
			}
			core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| 未命中 | 失败跳转【%s】未找到", e.config.Name, parentName, child.Name, child.OnErrorNext)
		}
		// 无失败跳转 → 尝试 next 列表中下一个
		core.LogWithTimef("[Engine] %s 巡检【%s】→【%s】| 未命中 | 继续下一个", e.config.Name, parentName, child.Name)
	}

	return &ExecResult{} // 全未命中 → 回到父循环
}

// swipeRetry 未命中时按"未命中滑动次数"转圈滑动一次再找（模仿找光柱转圈）
// 返回 true 表示已滑动一次（调用方应重新识别）；返回 false 表示已滑够或未配置
func (e *PipelineEngine) swipeRetry(step *StepConfig, swipeCount *int) bool {
	if step.RetrySwipe <= 0 || *swipeCount >= step.RetrySwipe {
		return false
	}
	// 滑动范围，未配置则默认光柱的 [318,74,1067,257]
	region := step.RetrySwipeRange
	if region[0] == 0 && region[1] == 0 && region[2] == 0 && region[3] == 0 {
		region = [4]int{318, 74, 1067, 257}
	}
	swipeCX := (region[0] + region[2]) / 2
	swipeCY := (region[1] + region[3]) / 2
	*swipeCount++
	core.LogWithTimef("[Engine] %s →【%s】未识别到 → 转圈滑动一次(%d,%d)→(%d,%d) 本轮已滑%d/%d次", e.config.Name, step.Name, swipeCX+100, swipeCY, swipeCX-100, swipeCY, *swipeCount, step.RetrySwipe)
	core.SwipeStraight(swipeCX+100, swipeCY, swipeCX-100, swipeCY, 300)
	return true
}

// checkPopups 扫描弹窗列表，命中后执行动作并返回非 nil；全部未命中返回 nil
// 调用方决定后续：HandlePopupsOnce 等只关心"点掉弹窗"这件事；
// processNext 里则是点掉后继续走 next（弹窗只插队，不丢弃跳转链）
// 先扫插件弹窗、再扫识别弹窗：插件弹窗（如"越过障碍_处理"）多为时效性提示，
// 提示话只在屏幕上停留约 2 秒，必须先于全局识别弹窗执行；否则会被全局全屏 OCR
// 弹窗挡在后面，等扫到时话已消失 → 死循环点任务。此改动只调整扫描顺序，不改任何弹窗逻辑。
func (e *PipelineEngine) checkPopups(rs *RunState, deadline time.Time, ctx *plugin.Context) *ExecResult {
	core.LogWithTimef("[Engine] %s 开始扫弹窗（共 %d 项）", e.config.Name, len(e.popups))

	// 第一遍：插件模式弹窗（时效性提示优先处理）
	for i := range e.popups {
		if plugin.IsStopped() {
			return nil
		}
		p := &e.popups[i]
		if !p.Plugin {
			continue
		}
		plug := plugin.Get(p.Name)
		if plug == nil {
			core.LogWithTimef("[Engine] %s 弹窗插件未注册: %s", e.config.Name, p.Name)
			continue
		}
		// 同步 StepName，弹窗插件日志显示正确步骤名
		ctx.StepName = p.Name
		next, err := plug.Exec(ctx)
		if err != nil {
			core.LogWithTimef("[Engine] %s 弹窗插件 %s 报错: %v", e.config.Name, p.Name, err)
			continue
		}
		if next == "" || next == "no" {
			continue // 未命中，尝试下一个弹窗
		}
		core.LogWithTimef("[Engine] %s 弹窗命中(插件): %s → %s", e.config.Name, p.Name, next)
		rs.ExecutedCounts[p.Name]++
		return &ExecResult{} // 回到父循环
	}

	// 第二遍：识别模式弹窗
	for i := range e.popups {
		if plugin.IsStopped() {
			return nil
		}
		p := &e.popups[i]
		if p.Plugin {
			continue
		}

		// 扫描日志：每个识别弹窗都打（含跳过引用配置，便于排查"跳过识别引用没生效"）
		// core.LogWithTimef("[Engine] %s 弹窗扫描: %s 引用=%q 跳过引用=%q", e.config.Name, p.Name, p.RecogRef, p.SkipRecogRefs)

		var px, py int
		var pMatched bool

		if e.hasRecognition(p) {
			px, py, pMatched = e.doRecognition(p, true) // 弹窗识别静默，不打日志
		} else {
			pMatched = true // 纯动作弹窗（如固定点击区域）
		}

		if pMatched {
			core.LogWithTimef("[Engine] %s 弹窗匹配: %s (引用:%s) 跳过引用:%v", e.config.Name, p.Name, p.RecogRef, p.SkipRecogRefs)
			// 排除条件：命中任一"跳过识别引用"（如剧情中）→ 跳过本弹窗不点
			if e.matchSkipRecogRefs(p) {
				core.LogWithTimef("[Engine] %s 弹窗被排除跳过: %s", e.config.Name, p.Name)
				continue
			}
			// core.LogWithTimef("[Engine] %s 弹窗命中: %s", e.config.Name, p.Name)
			e.doAction(p, px, py)
			if p.WaitAfter > 0 && p.Action != ActionSleep {
				core.RandomSleep(p.WaitAfter/2, p.WaitAfter)
			}
			rs.ExecutedCounts[p.Name]++
			// 弹窗处理完回到父循环（MAA #back），不中断主流程
			return &ExecResult{}
		}
	}
	return nil
}

// matchSkipRecogRefs 命中任一"跳过识别引用"则返回 true（弹窗需跳过）
// 用于"剧情中不点关闭"这类排除场景：识别引用命中后，若排除项也命中则跳过
func (e *PipelineEngine) matchSkipRecogRefs(step *StepConfig) bool {
	for _, ref := range step.SkipRecogRefs {
		r := core.Exec(ref)
		core.LogWithTimef("[Engine] %s 节点 %s 跳过引用检查: %s → Found=%v(%d,%d)", e.config.Name, step.Name, ref, r.Found, r.X, r.Y)
		if r.Found {
			return true
		}
	}
	return false
}

// ---- 插件执行 ----
func (e *PipelineEngine) execPluginStep(step *StepConfig, ctx *plugin.Context) string {
	// 插件执行前同步 StepName：
	// runNode 递归跳转（如 药品购买→StartUp）和 processNext 子节点插件执行时，
	// ctx.StepName 还残留顺序列表外层步骤名，会导致插件内部日志打印错误的步骤名
	ctx.StepName = step.Name
	p := plugin.Get(step.Name)
	if p == nil {
		core.LogWithTimef("[Engine] %s →【%s】插件未注册", e.config.Name, step.Name)
		return step.OnErrorNext
	}
	next, err := p.Exec(ctx)
	if err != nil {
		core.LogWithTimef("[Engine] %s →【%s】插件报错: %v", e.config.Name, step.Name, err)
		return step.OnErrorNext
	}
	if next == "" || next == "ok" {
		if len(step.Next) > 0 {
			return step.Next[0]
		}
		return NextDone
	}
	return next
}

// ---- 动作执行 ----
// actionDesc 命中日志里的动作描述：未配动作的纯检测/路由节点不再误报成"点击"
func actionDesc(step *StepConfig) string {
	if step.Plugin {
		return "插件执行后跟随"
	}
	switch step.Action {
	case ActionClick:
		return "点击后跟随"
	case ActionSwipe:
		return "滑动后跟随"
	case ActionSleep:
		return "等待后跟随"
	default: // ActionNone
		return "仅检测,不点击,直接跟随"
	}
}

func (e *PipelineEngine) doAction(step *StepConfig, x, y int) {
	// 空动作（纯路由节点如 StartUp）不打日志，避免与进入日志重复
	if step.Action != "" {
		core.LogWithTimef("[Engine] %s →【%s】→ %s", e.config.Name, step.Name, step.Action)
	}

	// 连点次数，默认 1
	times := step.ClickTimes
	if times <= 0 {
		times = 1
	}
	// 连点间隔，秒 → 毫秒，默认 500ms
	clickInterval := step.ClickInterval
	if clickInterval <= 0 {
		clickInterval = 500
	} else {
		clickInterval *= 1000
	}

	switch step.Action {
	case ActionSleep:
		if step.WaitAfter > 0 {
			time.Sleep(time.Duration(step.WaitAfter) * time.Millisecond)
		}
	case ActionNone:
		// 未配置"动作" → 只识别/路由，不执行点击
		// 用于纯检测步骤（如"主菜单"仅作为"已进游戏"的确认信号，检测命中即跳转，不该点开菜单）
		// 需要点击的步骤请显式配置 "动作": "点击"
	case ActionClick:
		// 识别方式由"识别引用"统一决定，动作只负责点击：
		// 应用点击偏移 → 有点击区域点区域，否则点识别坐标
		if step.ClickOffset[0] != 0 || step.ClickOffset[1] != 0 {
			x += step.ClickOffset[0]
			y += step.ClickOffset[1]
		}
		for i := 0; i < times; i++ {
			if step.ClickArea[2] > 0 && step.ClickArea[3] > 0 {
				core.RandomClickInArea(step.ClickArea[0], step.ClickArea[1], step.ClickArea[2], step.ClickArea[3])
			} else if x > 0 && y > 0 {
				// 点击延迟：配置了"点击延迟秒"（含写0=点击后零延迟）按配置走；没配置则保持原行为——不传第 5 参数，
				// 由 RandomClickInAreaWithDelay 内部默认处理（点击后随机睡 1.5~2 秒）
				if step.ClickDelay != nil {
					core.LogWithTimef("[Engine] %s →【%s】RandomClickInAreaWithDelay(%d, %d, %d, %d, %d)", e.config.Name, step.Name, x, y, x+5, y+5, *step.ClickDelay)
					core.RandomClickInAreaWithDelay(x, y, x+5, y+5, *step.ClickDelay)
				} else {
					core.LogWithTimef("[Engine] %s →【%s】RandomClickInAreaWithDelay(%d, %d, %d, %d)", e.config.Name, step.Name, x, y, x+5, y+5)
					core.RandomClickInAreaWithDelay(x, y, x+5, y+5)
				}
			}
			if times > 1 {
				time.Sleep(time.Duration(clickInterval) * time.Millisecond)
			}
		}
	case ActionSwipe:
		duration := step.SwipeDuration
		if duration <= 0 {
			duration = 300
		}
		// 绝对起终点: [x,y] 写法优先，其次旧的四字段写法
		sx, sy, ex, ey := step.SwipeX1, step.SwipeY1, step.SwipeX2, step.SwipeY2
		if step.SwipeStart != [2]int{} || step.SwipeEnd != [2]int{} {
			sx, sy = step.SwipeStart[0], step.SwipeStart[1]
			ex, ey = step.SwipeEnd[0], step.SwipeEnd[1]
		}
		// 滑动偏移模式: 基于识别位置 + 偏移计算起终点
		if x > 0 && y > 0 && step.SwipeDelta[0] != 0 || step.SwipeDelta[1] != 0 {
			sx = x + step.ClickOffset[0]
			sy = y + step.ClickOffset[1]
			ex = sx + step.SwipeDelta[0]
			ey = sy + step.SwipeDelta[1]
			core.LogWithTimef("[Engine] %s →【%s】滑动偏移: 识别(%d,%d) + 点击偏移(%d,%d) = 起点(%d,%d)，滑动偏移(%d,%d) = 终点(%d,%d)",
				e.config.Name, step.Name, x, y, step.ClickOffset[0], step.ClickOffset[1], sx, sy, step.SwipeDelta[0], step.SwipeDelta[1], ex, ey)
		}
		core.LogWithTimef("[Engine] %s →【%s】滑动执行: 识别坐标(%d,%d) 本次生效起终点 (%d,%d)→(%d,%d) 时长=%dms 配置[起点=%v 终点=%v 偏移=%v 时长=%d]",
			e.config.Name, step.Name, x, y, sx, sy, ex, ey, duration, step.SwipeStart, step.SwipeEnd, step.SwipeDelta, step.SwipeDuration)
		core.RandomSwipe(sx, sy, ex, ey, duration)
		core.LogWithTimef("[Engine] %s →【%s】滑动完成 (%d,%d)→(%d,%d) 耗时=%dms", e.config.Name, step.Name, sx, sy, ex, ey, duration)
	default:
		core.LogWithTimef("[Engine] %s →【%s】未知动作类型: %s", e.config.Name, step.Name, step.Action)
	}
}

// ---- 识别匹配 ----
// hasRecognition 该步骤是否需要识别（统一走识别预设）
func (e *PipelineEngine) hasRecognition(step *StepConfig) bool {
	return step.RecogRef != ""
}

// doRecognition 执行识别，返回 (x, y, matched)
// 所有识别统一走 core.Exec，不再分发到个别 FindXxx
// silent=true 时不打识别日志（弹窗扫描调用，避免刷屏）
func (e *PipelineEngine) doRecognition(step *StepConfig, silent bool) (int, int, bool) {
	if step.RecogRef == "" {
		return 0, 0, false
	}
	var r *core.RecogResult
	if step.Region != [4]int{} {
		// 步骤"截图区域"非零 → 覆盖预设区域（如寻路图标需换固定范围再检测）
		r = core.Exec(step.RecogRef, step.Region[0], step.Region[1], step.Region[2], step.Region[3])
	} else {
		r = core.Exec(step.RecogRef)
	}
	if !silent {
		if r.Found {
			core.LogWithTimef("[Engine] %s →【%s】→ 预设:%s(%s) → 命中", e.config.Name, step.Name, step.RecogRef, r.Type)
		} else {
			core.LogWithTimef("[Engine] %s →【%s】→ 预设:%s → 未命中", e.config.Name, step.Name, step.RecogRef)
		}
	}
	return r.X, r.Y, r.Found
}
