package plugin

import (
	"app/core"
	"sync"
	"sync/atomic"
)

// ---------- 全局停止标志 ----------
// 脚本停止(shutdown)时置位，所有长循环(引擎/插件)检查后立即退出，
// 避免"脚本已停但任务 goroutine 还在滑动/跳跃/转圈"

var stoppedFlag atomic.Bool

// SetStopped 设置全局停止标志
func SetStopped(v bool) { stoppedFlag.Store(v) }

// IsStopped 查询是否已停止
func IsStopped() bool { return stoppedFlag.Load() }

// ---------- 当前任务的取消令牌 ----------
// 调度器在任务开始时绑定该任务的取消令牌，任务彻底结束（引擎 goroutine 已退出）后解绑。
// 令牌由 task.PipelineEngine 持有（每个引擎一个），子任务引擎会共享外层的令牌，
// 于是"外层任务结束 → 子任务流水线 + 插件长循环"在各自的检查点一起退出。
//
// 与 stoppedFlag 的区别：stoppedFlag 是脚本级（停止脚本，所有任务都停）；
// 这里是任务级（只停当前这一个任务，脚本继续跑下一个）。

var 当前取消令牌 atomic.Pointer[atomic.Bool]

// SetCurrentCancel 绑定当前任务的取消令牌（由 bot 调度器在 executeTask 里调用），传 nil = 解绑
func SetCurrentCancel(p *atomic.Bool) { 当前取消令牌.Store(p) }

// CurrentCancel 取当前任务的取消令牌（bot 建子任务引擎时用，让子任务共享外层令牌）
func CurrentCancel() *atomic.Bool { return 当前取消令牌.Load() }

// IsTaskCancelled 当前任务是否已被要求结束（引擎/插件的检查点自查用）。
// 未绑定令牌（全量重启的登录流程、弹窗清扫临时引擎、单元测试）时恒为 false，不影响既有流程。
func IsTaskCancelled() bool {
	if p := 当前取消令牌.Load(); p != nil {
		return p.Load()
	}
	return false
}

// Plugin 插件接口
// JSON 步骤中 action="plugin"，name 匹配插件名时触发
type Plugin interface {
	// Name 返回插件名，对应 JSON step 的 name 字段
	Name() string
	// Exec 执行插件逻辑。返回值 nextStep 与 err 的语义如下：
	//
	// ① 成功：nextStep == "" 或 "ok"
	//    含义：要做的事已经做完，当前结果就是该步骤想要的。
	//    引擎动作：走"成功跳转"——跳到 JSON 里"成功跳转"列表的第一个步骤；
	//              若没配"成功跳转"，则把整个任务判为完成（#done）。
	//    注意："" 和 "ok" 完全等价，推荐统一写 "ok"，别混用。
	//
	// ② 未命中：nextStep == "no"
	//    含义：没找到要做的事 / 当前状态不适用，无事可做。
	//    引擎动作：走"失败跳转"——跳到 JSON 里的"失败跳转"步骤
	//              （各插件步骤通常配的是回 StartUp 路由器，继续尝试其他场景）；
	//              若没配"失败跳转"，引擎按未命中继续执行下一个步骤，不报错。
	//    典型：剧情对话没检测到对话 → 返回 "no" 交给路由器；
	//          装备提升/药品购买在弹窗处理里 → 返回 "no" 表示没命中该弹窗。
	//
	// ③ 显式跳转：nextStep 为其他任意字符串
	//    含义：直接跳转到名为该字符串的步骤。
	//    引擎动作：去找同名步骤执行；找不到时报"跳转未找到"并使任务失败。
	//    一般用不上，能用 ①/② 表达的别用它。
	//
	// ④ 报错：err != nil
	//    含义：插件执行出错。
	//    引擎动作：走 JSON 里的"失败跳转"(onErrorNext)。
	//    注意：err != nil 时 nextStep 返回什么都会被忽略，引擎只认 err。
	Exec(ctx *Context) (nextStep string, err error)
}

// Context 插件上下文，提供运行时环境
type Context struct {
	// TaskName 当前任务名
	TaskName string
	// StepName 当前步骤名
	StepName string
	// Data 插件间共享数据（跨步骤）
	Data map[string]interface{}
	// Global 全局存储（跨任务）
	Global map[string]interface{}
}

// NewContext 创建上下文
func NewContext(taskName, stepName string) *Context {
	return &Context{
		TaskName: taskName,
		StepName: stepName,
		Data:     make(map[string]interface{}),
		Global:   make(map[string]interface{}),
	}
}

// ---------- 插件注册表 ----------

var (
	registry     = map[string]Plugin{}
	registryLock sync.RWMutex
)

// Register 注册一个插件（通常在 init() 中调用）
func Register(p Plugin) {
	registryLock.Lock()
	defer registryLock.Unlock()
	name := p.Name()
	if name == "" {
		panic("plugin.Register: 插件名不能为空")
	}
	if _, exists := registry[name]; exists {
		panic("plugin.Register: 插件名冲突: " + name)
	}
	registry[name] = p
}

// Get 获取已注册的插件
func Get(name string) Plugin {
	registryLock.RLock()
	defer registryLock.RUnlock()
	return registry[name]
}

// MustRegister 批量注册
func MustRegister(plugins ...Plugin) {
	for _, p := range plugins {
		Register(p)
	}
}

// ---------- 任务运行入口（依赖反转） ----------
//
// 插件里想执行某个配置化任务(assets/tasks/*.json)时调用 RunTask(name)。
// 但 task 包导入了 plugin（引擎要执行插件步骤），插件不能反向 import task，
// 否则循环依赖编译不过。所以这里只声明一个函数变量，由外层(bot)在初始化时
// 注入真正的实现（bot 同时导入 plugin 和 task，可以做这件事）。

var (
	runTaskMu sync.RWMutex
	runTaskFn func(name string, cooldownAfterSec int) bool
)

// SetRunTaskFunc 注入"按任务名执行一次"的实现（由 bot 在 Init 时调用）
func SetRunTaskFunc(fn func(name string, cooldownAfterSec int) bool) {
	runTaskMu.Lock()
	runTaskFn = fn
	runTaskMu.Unlock()
}

// RunTask 按名称立即执行一次配置化任务，返回是否执行成功
//
//	RunTask("背包物品使用")       // 只跑一次, 沿用任务 JSON 里配的"冷却秒"
//	RunTask("背包物品使用", 900)  // 跑完后强制冷却 900 秒（覆盖 JSON 配置）
//	RunTask("背包物品使用", 0)    // 跑完后不额外冷却, 下次可立即再跑
//
// 自动/手动互不干扰: 自动调度只挑 JSON 里"自动调度":true 的任务(PickNext),
// 这里按名直接取, 所以任务配 false 也能手动触发, 配 true 则两种都可用。
//
// 未注入实现（如单元测试/未走 bot 启动流程）时返回 false，不会 panic。
func RunTask(name string, cooldownAfterSec ...int) bool {
	runTaskMu.RLock()
	fn := runTaskFn
	runTaskMu.RUnlock()
	if fn == nil {
		return false
	}
	cd := -1 // 默认不覆盖，沿用 JSON 配的冷却
	if len(cooldownAfterSec) > 0 {
		cd = cooldownAfterSec[0]
	}
	return fn(name, cd)
}

// ---------- 子任务：直接跑一遍流水线（依赖反转）----------
//
// 子任务 = 只想复用 assets/tasks/<名字>.json 的步骤轮转 + 流水线内嵌弹窗能力，
// 但**不作为"任务"**参与调度：不写任务状态、不压冷却、不占 HUD、不进 PickNext 自动轮转。
// 它在调用方（外层任务）的同一个 goroutine 里同步跑完 —— 所以外层任务一结束，
// 子任务必然已经结束，不会出现"任务结束了里面还在跑"。
//
// 与 RunTask 的区别：RunTask 走调度器(新建引擎+状态+冷却+HUD)，这里只跑流水线本身。
// 与 RunTask 同理：task 包导入了 plugin，插件不能反向 import task，由外层(bot)注入实现。

var (
	runSubTaskMu sync.RWMutex
	runSubTaskFn func(name string) bool
)

// SetRunSubTaskFunc 注入"按名字直接跑一遍任务流水线"的实现（由 bot 在 Init 时调用）
func SetRunSubTaskFunc(fn func(name string) bool) {
	runSubTaskMu.Lock()
	runSubTaskFn = fn
	runSubTaskMu.Unlock()
}

// RunSubTask 跑一遍指定任务的流水线（不参与调度、不写状态/冷却），返回是否跑成功。
// 未注入实现（如单元测试/未走 bot 启动流程）时返回 false，不会 panic。
func RunSubTask(name string) bool {
	runSubTaskMu.RLock()
	fn := runSubTaskFn
	runSubTaskMu.RUnlock()
	if fn == nil {
		return false
	}
	return fn(name)
}

// ---------- 清除任务冷却（依赖反转） ----------
//
// 插件想"让某个配置化任务解除冷却、下一轮调度即可执行"（不立即跑）时调用 ResetTaskCooldown(name)。
// 与 RunTask 同理：task 包导入了 plugin，插件不能反向 import task，由外层(bot)初始化时注入实现。

var (
	resetCooldownMu sync.RWMutex
	resetCooldownFn func(taskName string) bool
)

// SetResetTaskCooldownFunc 注入"清除指定任务冷却"的实现（由 bot 在 Init 时调用）
func SetResetTaskCooldownFunc(fn func(taskName string) bool) {
	resetCooldownMu.Lock()
	resetCooldownFn = fn
	resetCooldownMu.Unlock()
}

// ResetTaskCooldown 清除指定任务的冷却（JSON"冷却秒" + 强制冷却都清），返回是否清除成功。
// 只清冷却、不立即执行任务；目标任务需配"自动调度":true 才会被调度器下一轮自动选中。
// 未注入实现（如单元测试/未走 bot 启动流程）时返回 false，不会 panic。
func ResetTaskCooldown(taskName string) bool {
	resetCooldownMu.RLock()
	fn := resetCooldownFn
	resetCooldownMu.RUnlock()
	if fn == nil {
		return false
	}
	return fn(taskName)
}

// ---------- 任务轮转弹窗清扫（依赖反转）----------
//
// 插件想主动做一次"任务轮转弹窗清扫"（detectors.json「任务轮转」段那种，扫到没有弹窗为止）时
// 调用 ClearPopups()。与 RunTask 同理：task 包导入了 plugin，插件不能反向 import task，
// 由外层(bot)初始化时注入实现。返回本次处理掉的弹窗次数。

var (
	clearPopupsMu sync.RWMutex
	clearPopupsFn func(skip ...string) int
)

// SetClearPopupsFunc 注入"清一次任务轮转弹窗"的实现（由 bot 在 Init 时调用）
func SetClearPopupsFunc(fn func(skip ...string) int) {
	clearPopupsMu.Lock()
	clearPopupsFn = fn
	clearPopupsMu.Unlock()
}

// ClearPopups 主动做一次任务轮转弹窗清扫（扫到没有弹窗命中为止），返回处理次数。
// skip: 本次不清的弹窗名单（按"名称"或"识别引用"匹配，留空=全清）。
// 空串会被过滤掉，调用方可以无条件把"可选的跳过项"传进来。
// 未注入实现（如单元测试/未走 bot 启动流程）时返回 0，不会 panic。
func ClearPopups(skip ...string) int {
	实跳 := skip[:0]
	for _, s := range skip {
		if s != "" {
			实跳 = append(实跳, s)
		}
	}
	clearPopupsMu.RLock()
	fn := clearPopupsFn
	clearPopupsMu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn(实跳...)
}

// ---------- 触发全量重启（依赖反转）----------
//
// 与 RunTask 同理：全量重启（kill 代理+游戏 → 启动代理 → 启动游戏 → 登录）在 bot 里，
// 而 bot 依赖 task/plugin，插件不能反向 import bot，由外层(bot)初始化时注入实现。
// 注入的实现内部自己起 goroutine 执行（恢复期间状态机进 Recovering，调度器不再取新任务），
// 所以插件调用后立即返回，不会卡住当前任务流水线。

var (
	fullRestartMu sync.RWMutex
	fullRestartFn func(reason string)
)

// SetFullRestartFunc 注入"触发一次全量重启"的实现（由 bot 在 Init 时调用）
func SetFullRestartFunc(fn func(reason string)) {
	fullRestartMu.Lock()
	defer fullRestartMu.Unlock()
	fullRestartFn = fn
}

// TriggerFullRestart 触发一次全量重启（重启 kit(代理) + 游戏），reason 只用于日志/状态显示。
// 未注入实现（如单元测试/未走 bot 启动流程）时只记日志，不会 panic。
func TriggerFullRestart(reason string) {
	fullRestartMu.RLock()
	fn := fullRestartFn
	fullRestartMu.RUnlock()
	if fn == nil {
		core.LogWithTimef("[Plugin] 未注入全量重启实现 → 忽略重启请求: %s", reason)
		return
	}
	fn(reason)
}
