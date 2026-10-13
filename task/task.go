package task

import (
	"encoding/json"
	"fmt"
	"time"

	"app/core"
)

// ───── JSON 配置结构 ─────

// TaskConfig 单个任务 JSON 配置
type TaskConfig struct {
	Name        string       `json:"名称"`    // 任务唯一标识
	Auto        bool         `json:"自动调度"`  // 是否自动调度
	Level       int          `json:"优先级"`   // 优先级, 数字越小越优先
	MinLevel    int          `json:"最低等级"`  // 角色等级门槛, 0=不限; 等级 >= 该值才可调度
	MaxLevel    int          `json:"最高等级"`  // 角色等级上限, 0=不限; 等级 <= 该值才可调度
	MinPower    int          `json:"最低战力"`  // 角色战力门槛, 0=不限; 战力 >= 该值才可调度（识别不到战力 → 放行）
	PowerGates  []PowerGate  `json:"战力门槛"`  // 等级→最低战力门槛表（可配多段: 如28-35级需16000战力, 36-40级需20000战力）
	Cooldown    int          `json:"冷却秒"`   // 冷却时间, 秒
	MaxDuration int          `json:"最大执行秒"` // 最长执行时间, 秒
	TimeWindows []TimeWindow `json:"时间窗口"`  // 时间窗口
	Steps       []StepConfig `json:"步骤列表"`  // 步骤列表
	Popups      []StepConfig `json:"弹窗处理"`  // 弹窗处理步骤（MAA式：注入到每个步骤Next前，命中后#back）
}

// NextList 下一步的跳转列表
// JSON 兼容: 可以是字符串 "step2" 或数组 ["stepA","stepB"]
type NextList []string

// UnmarshalJSON 支持字符串和数组两种格式
func (n *NextList) UnmarshalJSON(data []byte) error {
	// 尝试数组
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*n = arr
		return nil
	}
	// 尝试单字符串（向后兼容旧配置）
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s != "" {
			*n = []string{s}
		}
		return nil
	}
	return fmt.Errorf("next: 需要字符串或字符串数组")
}

// TimeWindow 时间窗口
type TimeWindow struct {
	Start string `json:"开始"` // HH:MM
	End   string `json:"结束"` // HH:MM
}

// PowerGate 等级→最低战力门槛段（"战力门槛"配置表的一条）
// 当前角色等级落在 [最低等级, 最高等级] 区间内时，要求战力 >= 最低战力 才可调度；
// 等级不在任何区间 → 无门槛放行（如低级号主线照做）。可配多条实现"35级16000、40级20000、以此类推"。
type PowerGate struct {
	MinLevel int `json:"最低等级"` // 区间下界（含），0=不限下界
	MaxLevel int `json:"最高等级"` // 区间上界（含），0=不限上界
	MinPower int `json:"最低战力"` // 该区间要求的最低战力
}

// StepConfig 单个步骤 JSON 配置
type StepConfig struct {
	Name        string `json:"名称"` // 步骤名（跳转锚点）
	Description string `json:"描述"` // 步骤描述
	Action      string `json:"动作"` // 动作类型

	// 识别字段
	RecogRef        string   `json:"识别引用"`   // 识别条件预设引用（assets/recogs.json），加载时自动合并
	SkipRecogRefs   []string `json:"跳过识别引用"` // 命中任一识别引用则跳过本步骤（排除条件）
	Images          []string `json:"模板图片"`   // 模板图片名
	Text            string   `json:"识别文字"`   // OCR 文字
	Colors          []string `json:"颜色数组"`   // 颜色数组
	ColorMulti      []string `json:"多点找色"`   // 多点找色格式（数组，单个也写成一个元素）
	Label           string   `json:"界面标签"`   // 界面标签（assets/scene.json 的 key）
	Region          [4]int   `json:"截图区域"`   // 截图区域 [x1,y1,x2,y2]
	IsStrictMode    bool     `json:"严格模式"`   // 严格模式
	SearchImageType int      `json:"模板匹配算法"` // 模板匹配算法
	FontName        string   `json:"字库名"`    // 字库名
	// 相似度：本步骤所有识别类型（多点找色/模板图/找色/OCR取文/字库找字）统一的阈值字段名，
	// 与 assets/recogs.json 预设里的 "相似度" 完全一致；不写则由各识别类型套默认值
	Sim             float64  `json:"相似度"`
	Count           int      `json:"找色数量"`   // 找色数量
	Direction       int      `json:"找色方向"`   // 找色方向

	// 动作参数
	// 点击目标优先级: 点击区域 > 点击偏移 > 检测坐标。三样都不写就点检测到的位置
	ClickArea     [4]int `json:"点击区域"`  // 绝对点击区域 [x1, y1, x2, y2] → RandomClickInArea
	ClickOffset   [2]int `json:"点击偏移"`  // 相对识别位置的位移 [dx, dy]
	ClickTimes    int    `json:"连点次数"`  // 点击次数（连点），默认1次
	ClickInterval int    `json:"连点间隔秒"` // 连点间隔，秒（默认0.5）
	SwipeStart    [2]int `json:"滑动起点"`  // 绝对滑动起点 [x, y]
	SwipeEnd      [2]int `json:"滑动终点"`  // 绝对滑动终点 [x, y]
	SwipeX1       int    `json:"滑动起点X"` // 滑动起点 X（旧字段，兼容保留）
	SwipeY1       int    `json:"滑动起点Y"` // 滑动起点 Y（旧字段，兼容保留）
	SwipeX2       int    `json:"滑动终点X"` // 滑动终点 X（旧字段，兼容保留）
	SwipeY2       int    `json:"滑动终点Y"` // 滑动终点 Y（旧字段，兼容保留）
	SwipeDelta    [2]int `json:"滑动偏移"`  // 相对 OCR 位置的滑动偏移量 [dx, dy]
	SwipeDuration int    `json:"滑动时长"`  // 滑动持续时间
	RetrySwipe      int    `json:"未命中滑动次数"` // 未识别到时转圈滑动一次再找，最多找该次数
	RetrySwipeRange [4]int `json:"滑动范围"`   // 未命中滑动范围 [x1,y1,x2,y2]（默认光柱 [318,74,1067,257]）；以范围中心为基准水平左右各100px、时长800ms，模仿找光柱转圈滑动
	WaitAfter     int    `json:"等待秒"`    // 执行后等待，JSON 配秒，运行时转毫秒
	ClickDelay    *int   `json:"点击延迟秒"` // 点击动作点击后的固定延迟，JSON 配秒，运行时转毫秒；不写=nil=点击后走默认延迟(随机1.5~2秒)；写0=点击后零延迟

	// 流程控制
	Plugin      bool     `json:"插件模式"`  // 是否走插件
	Next        NextList `json:"成功跳转"`  // 成功跳转 step name 列表（MAA 式: 逐一尝试，首个匹配跳转）
	OnErrorNext string   `json:"失败跳转"`  // 超时/失败跳转 step name
	MaxTimes    int      `json:"最大次数"`  // 最大识别次数
	Timeout     int      `json:"超时秒"`   // 本步骤识别超时，JSON 配秒，运行时转毫秒 (0=仅尝试一次)
	RateLimit   int      `json:"重试间隔秒"` // 识别重试间隔，JSON 配秒，运行时转毫秒 (默认 1)

}

// 动作类型常量（插件步骤由"插件模式"字段触发，无独立动作类型）
const (
	ActionClick = "点击"
	ActionSwipe = "滑动"
	ActionSleep = "等待"
	ActionNone  = ""
)

// 特殊跳转名（用于 next 列表）
const (
	NextDone = "#done" // 任务完成，立即返回成功
	NextBack = "#back" // 回到父节点的 next 列表循环（MAA 核心机制）
	NextSelf = "#self" // 回到当前节点重新开始
)

// ───── 运行时状态 ─────

// TaskState 任务当前状态
type TaskState int

const (
	TaskPending   TaskState = iota // 等待调度
	TaskRunning                    // 执行中
	TaskCompleted                  // 完成
	TaskFailed                     // 失败
	TaskCancelled                  // 取消
)

func (ts TaskState) String() string {
	switch ts {
	case TaskPending:
		return "PENDING"
	case TaskRunning:
		return "RUNNING"
	case TaskCompleted:
		return "COMPLETED"
	case TaskFailed:
		return "FAILED"
	case TaskCancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

// ───── Task 接口（调度器用）─────

// Task 任务接口
type Task interface {
	// Name 任务名
	Name() string
	// Level 优先级, 越小越优先
	Level() int
	// Cooldown 冷却时间
	Cooldown() time.Duration
	// MaxDuration 最大执行时限
	MaxDuration() time.Duration
	// CanRun 当前是否可以运行（时间窗口、冷却等）
	CanRun() bool
	// State 当前状态
	State() TaskState
	// Run 执行任务, 返回错误和是否需要重试
	Run() error
	// Reset 重置状态, 准备下一次执行
	Reset()
}

// ───── 运行时任务封装 ─────

// RuntimeTask 运行时任务对象
type RuntimeTask struct {
	config    TaskConfig
	state     TaskState
	name      string
	lastRunAt time.Time
	runState  *RunState // 引擎运行时状态
}

// RunState 步骤执行进度
type RunState struct {
	CurrentStep    int            // 当前步骤索引
	ExecutedCounts map[string]int // 步骤名 → 已执行次数
	Interrupted    bool           // 是否被打断
}

// NewRuntimeTask 创建运行时任务
func NewRuntimeTask(cfg TaskConfig) *RuntimeTask {
	return &RuntimeTask{
		config: cfg,
		state:  TaskPending,
		name:   cfg.Name,
		runState: &RunState{
			CurrentStep:    0,
			ExecutedCounts: make(map[string]int),
		},
	}
}

func (rt *RuntimeTask) Name() string { return rt.name }
func (rt *RuntimeTask) Level() int   { return rt.config.Level }
func (rt *RuntimeTask) Cooldown() time.Duration {
	return time.Duration(rt.config.Cooldown) * time.Second
}
func (rt *RuntimeTask) MaxDuration() time.Duration {
	return time.Duration(rt.config.MaxDuration) * time.Second
}
func (rt *RuntimeTask) State() TaskState    { return rt.state }
func (rt *RuntimeTask) Config() TaskConfig  { return rt.config }
func (rt *RuntimeTask) RunState() *RunState { return rt.runState }

func (rt *RuntimeTask) SetState(s TaskState)   { rt.state = s }
func (rt *RuntimeTask) SetLastRun(t time.Time) { rt.lastRunAt = t }
func (rt *RuntimeTask) LastRunAt() time.Time   { return rt.lastRunAt }

// lastPopupAt 识别失败时弹窗处理的节流时间（10 秒最多一次，避免每轮调度都截图点击）
var lastPopupAt time.Time

// clearBlockingPopup 识别失败时走一次弹窗处理：构造引擎只跑弹窗逻辑，把挡界面的弹窗关掉。
// 弹窗逻辑"识别到才点、识别不到无动作"，安全；10 秒节流避免每 tick 都全量扫弹窗。
func (rt *RuntimeTask) clearBlockingPopup() {
	if time.Since(lastPopupAt) < 10*time.Second {
		return
	}
	lastPopupAt = time.Now()
	core.LogWithTimef("[Task] %s 等级/战力识别失败 → 执行弹窗处理", rt.name)
	e := NewPipelineEngine(rt.config)
	e.HandlePopupsOnce()
}

// powerGateCooldown 战力门槛不满足时追加的强制冷却时长。
// 战力短期内不会涨够，没必要每轮调度都重新截图 OCR 一遍等级/战力，直接压 1 小时再看。
const powerGateCooldown = time.Hour

// markPowerGateCooldown 战力门槛不满足时压一段强制冷却（走 core.ExtendCooldown）
//
// 不会每 tick 重复续期：CanRun 的检查顺序是
// 运行态 → JSON冷却 → 强制冷却 → 时间窗口 → 等级 → 最低战力 → 战力门槛表，
// 强制冷却排在战力检查之前，压上之后 CanRun 会先在强制冷却处 return false，
// 根本走不到战力门槛那段代码；等 1 小时到期后若战力仍不足，才会再压 1 小时。
func (rt *RuntimeTask) markPowerGateCooldown(minPower int) {
	until := time.Now().Add(powerGateCooldown)
	core.ExtendCooldownWithReason(rt.name, until, "战力门槛不足")
	core.LogWithTimef("[Task] %s 战力低于门槛 %d → 强制冷却至 %s（%v）",
		rt.name, minPower, until.Format("15:04:05"), powerGateCooldown)
}

// getLevelOrClear 识别等级：失败先走一次弹窗处理，不重试（进冷却轮转兜底）
func (rt *RuntimeTask) getLevelOrClear() (*int, bool) {
	lv, ok := core.GetLevel(core.DefaultLevelCacheTTL)
	if !ok {
		rt.clearBlockingPopup()
	}
	return lv, ok
}

// getPowerOrClear 识别战力：失败先走一次弹窗处理，不重试（进冷却轮转兜底）
// 本游戏没有"战力图标"，识别战斗力不做前置门控，直接 Exec("战斗力")
func (rt *RuntimeTask) getPowerOrClear() (*int, bool) {
	pp, ok := core.GetCachedTextInt("战斗力", core.DefaultLevelCacheTTL)
	if !ok {
		rt.clearBlockingPopup()
	}
	return pp, ok
}

// CanRun 检查是否能运行
//
// 注意: 本函数有副作用 —— 战力门槛不满足时会调用 markPowerGateCooldown() 给任务
// 压一段强制冷却(core.ExtendCooldown)；等级/战力"识别不到"时会调用 markFailCooldown()
// 刷新 lastRunAt 进冷却轮转。调用方（调度器 PickNext、手动 RunTaskNow）都把
// CanRun()==false 当作"这次不跑"，所以副作用方向一致，不会误判。
// 但请勿把它当纯查询在高频循环里反复调用（BlockReason 才是只读的）。
func (rt *RuntimeTask) CanRun() bool {
	if rt.state == TaskRunning {
		return false
	}
	// 冷却检查
	if rt.config.Cooldown > 0 && time.Since(rt.lastRunAt) < rt.Cooldown() {
		return false
	}
	// 强制冷却检查（插件可调用 core.ExtendCooldown 延长，如悬赏任务一个任务都没接到时）
	if core.CooldownBlocked(rt.name, time.Now()) {
		return false
	}
	// 时间窗口检查
	if len(rt.config.TimeWindows) > 0 {
		if !rt.inTimeWindow() {
			return false
		}
	}
	// 等级区间门槛：最低等级 <= 等级 <= 最高等级 才可运行（0=不限；识别不到等级 → 不可运行，即"没确认满足条件就不做"）
	if rt.config.MinLevel > 0 || rt.config.MaxLevel > 0 {
		lvp, ok := rt.getLevelOrClear()
		if !ok {
			rt.markFailCooldown()
			return false
		}
		if rt.config.MinLevel > 0 && *lvp < rt.config.MinLevel {
			return false
		}
		if rt.config.MaxLevel > 0 && *lvp > rt.config.MaxLevel {
			return false
		}
	}
	// 战力门槛：最低战力 > 0 时，战力 >= 该值才可运行（0=不限；识别不到战力 → 不可运行，即"没确认满足条件就不做"）
	if rt.config.MinPower > 0 {
		pp, ok := rt.getPowerOrClear()
		if !ok {
			rt.markFailCooldown()
			return false
		}
		if *pp < rt.config.MinPower {
			rt.markPowerGateCooldown(rt.config.MinPower) // 战力不够 → 压 1 小时强制冷却, 不再每轮重复识别
			return false
		}
	}
	// 等级→战力门槛表（战力门槛）：当前等级落在某区间且战力不足该区间要求 → 不可运行。
	// 等级不在任何区间 → 无门槛放行（如低级号主线照做）；识别不到等级/战力 → 不可运行。
	if len(rt.config.PowerGates) > 0 {
		lvp, ok := rt.getLevelOrClear()
		if !ok {
			rt.markFailCooldown()
			return false
		}
		for _, g := range rt.config.PowerGates {
			if *lvp >= g.MinLevel && *lvp <= g.MaxLevel {
				pp, ok2 := rt.getPowerOrClear()
				if !ok2 {
					rt.markFailCooldown()
					return false
				}
				if *pp < g.MinPower {
					rt.markPowerGateCooldown(g.MinPower) // 战力不够 → 压 1 小时强制冷却, 不再每轮重复识别
					return false
				}
				break
			}
		}
	}
	return true
}

// markFailCooldown 识别失败（等级/战力识别不到）时计入冷却轮转：
// 识别不到不能执行任务，但也不该永远卡在"识别失败"状态不动；
// 把失败时刻记入 lastRunAt，让冷却开始倒计时（BlockReason 会优先显示"冷却剩余"），
// 冷却结束再重新识别 → 失败再冷却，形成轮转。
// 只在"上次失败已满冷却（含从未失败过）"时才刷新，避免每轮都刷新导致冷却永不过期。
func (rt *RuntimeTask) markFailCooldown() {
	if rt.config.Cooldown > 0 && time.Since(rt.lastRunAt) >= rt.Cooldown() {
		rt.lastRunAt = time.Now()
	}
}

// BlockReason 返回当前不可运行的原因（只读查询，供调度器日志打印，不改变任何状态/逻辑）
func (rt *RuntimeTask) BlockReason(now time.Time) string {
	if rt.state == TaskRunning {
		return "执行中"
	}
	// 冷却检查
	if rt.config.Cooldown > 0 && now.Sub(rt.lastRunAt) < rt.Cooldown() {
		left := rt.Cooldown() - now.Sub(rt.lastRunAt)
		return fmt.Sprintf("冷却剩余 %v", left.Round(time.Second))
	}
	// 强制冷却检查（带剩余时间与原因：只显示"强制冷却中"分不清是谁压的、还要等多久）
	if until, reason, ok := core.CooldownUntil(rt.name); ok {
		left := until.Sub(now).Round(time.Second)
		if reason != "" {
			return fmt.Sprintf("强制冷却剩余 %v(%s)", left, reason)
		}
		return fmt.Sprintf("强制冷却剩余 %v", left)
	}
	// 时间窗口检查
	if len(rt.config.TimeWindows) > 0 && !rt.inTimeWindow() {
		tw := rt.config.TimeWindows[0]
		return fmt.Sprintf("时间窗口外 %s~%s", tw.Start, tw.End)
	}
	// 等级区间门槛
	if rt.config.MinLevel > 0 || rt.config.MaxLevel > 0 {
		if lvp, ok := core.GetLevel(core.DefaultLevelCacheTTL); ok {
			if rt.config.MinLevel > 0 && *lvp < rt.config.MinLevel {
				return fmt.Sprintf("等级不足 需≥%d/当前%d", rt.config.MinLevel, *lvp)
			}
			if rt.config.MaxLevel > 0 && *lvp > rt.config.MaxLevel {
				return fmt.Sprintf("等级过高 需≤%d/当前%d", rt.config.MaxLevel, *lvp)
			}
		} else {
			return "等级识别失败"
		}
	}
	// 战力门槛
	if rt.config.MinPower > 0 {
		if pp, ok := core.GetCachedTextInt("战斗力", core.DefaultLevelCacheTTL); ok {
			if *pp < rt.config.MinPower {
				return fmt.Sprintf("战力不足 需≥%d/当前%d", rt.config.MinPower, *pp)
			}
		} else {
			return "战力识别失败"
		}
	}
	// 等级→战力门槛表
	if len(rt.config.PowerGates) > 0 {
		if lvp, ok := core.GetLevel(core.DefaultLevelCacheTTL); ok {
			for _, g := range rt.config.PowerGates {
				if *lvp >= g.MinLevel && *lvp <= g.MaxLevel {
					if pp, ok2 := core.GetCachedTextInt("战斗力", core.DefaultLevelCacheTTL); ok2 {
						if *pp < g.MinPower {
							return fmt.Sprintf("战力不足 需≥%d/当前%d(等级%d)", g.MinPower, *pp, *lvp)
						}
					} else {
						return "战力识别失败"
					}
					break
				}
			}
		} else {
			return "等级识别失败"
		}
	}
	return "可运行"
}

// inTimeWindow 判断当前是否在某个时间窗口内
func (rt *RuntimeTask) inTimeWindow() bool {
	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	for _, tw := range rt.config.TimeWindows {
		start, err := parseTimeMinutes(tw.Start)
		if err != nil {
			continue
		}
		end, err := parseTimeMinutes(tw.End)
		if err != nil {
			continue
		}
		if currentMinutes >= start && currentMinutes <= end {
			return true
		}
	}
	return false
}

// parseTimeMinutes 将 "HH:MM" 转为分钟数
func parseTimeMinutes(t string) (int, error) {
	var h, m int
	_, err := fmt.Sscanf(t, "%d:%d", &h, &m)
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

// Reset 重置任务状态
func (rt *RuntimeTask) Reset() {
	rt.state = TaskPending
	rt.runState.CurrentStep = 0
	rt.runState.ExecutedCounts = make(map[string]int)
	rt.runState.Interrupted = false
}
