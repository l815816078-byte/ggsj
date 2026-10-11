package plugin

import (
	"math/rand"

	"app/core"
)

func init() {
	Register(&野外挂机{})
	Register(&挂机中{})
}

// 野外挂机 插件：去 英勇的清山，按战力选最符合的地点，走"去地图"那套。
//
// 流程：取【英勇的清山】配置 → 按战力选最符合的地点 → 去地图(配置, 地点)
// 命中返回 "ok"，否则 "no"。
type 野外挂机 struct{}

func (p *野外挂机) Name() string { return "野外挂机" }

func (p *野外挂机) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	// 取【英勇的清山】配置
	配置, ok := 取主城配置("英勇的清山")
	if !ok || len(配置.地点列表) == 0 {
		core.LogWithTimef("[野外挂机] 未找到【英勇的清山】配置 → 失败")
		return "no", nil
	}

	// 按战力选最符合的地点
	地点 := 选地点(配置.地点列表)
	core.LogWithTimef("[野外挂机] 目标：主城【%s】→ 地点【%s】(战力门槛 %d)",
		配置.主城名, 地点.名称, 地点.战力门槛)

	去地图(配置, 地点)
	return "ok", nil
}

// 选地点 按当前战力选最符合的地点：
//   - 门槛 ≤ 战力里取门槛最高的；同门槛有多个 → 随机一个
//   - 检测不到战力 / 一个门槛都不满足 → 取门槛最低的（同最低多个 → 随机一个）
func 选地点(列表 []地点) 地点 {
	pw, ok := core.GetCachedTextInt("战斗力", core.DefaultLevelCacheTTL)
	if !ok {
		core.LogWithTimef("[野外挂机] 未检测到战力 → 取门槛最低的地点")
		return 最低门槛地点(列表)
	}
	core.LogWithTimef("[野外挂机] 当前战力=%d", *pw)

	最高 := -1
	for _, pt := range 列表 {
		if pt.战力门槛 <= *pw && pt.战力门槛 > 最高 {
			最高 = pt.战力门槛
		}
	}
	if 最高 < 0 {
		core.LogWithTimef("[野外挂机] 战力%d 不满足任何门槛 → 取门槛最低的地点", *pw)
		return 最低门槛地点(列表)
	}

	var 候选 []地点
	for _, pt := range 列表 {
		if pt.战力门槛 == 最高 {
			候选 = append(候选, pt)
		}
	}
	return 候选[rand.Intn(len(候选))]
}

// 最低门槛地点 战力门槛最低的那个（同最低有多个 → 随机一个）
func 最低门槛地点(列表 []地点) 地点 {
	最低 := 列表[0].战力门槛
	for _, pt := range 列表 {
		if pt.战力门槛 < 最低 {
			最低 = pt.战力门槛
		}
	}
	var 候选 []地点
	for _, pt := range 列表 {
		if pt.战力门槛 == 最低 {
			候选 = append(候选, pt)
		}
	}
	return 候选[rand.Intn(len(候选))]
}

// 挂机中 插件：检测到"自动攻击开启中"即认为正在挂机中。
// 挂机中顺带跑子任务（按顺序，冷却 1000 秒避免每轮重复执行）；
// 命中返回 "ok"（挂机中），没检测到返回 "no"。
type 挂机中 struct{}

func (p *挂机中) Name() string { return "挂机中" }

func (p *挂机中) Exec(ctx *Context) (string, error) {
	if !core.Exec("自动攻击开启中").Found {
		return "no", nil
	}
	core.LogWithTimef("[挂机中] 检测到自动攻击开启中 → 正在挂机")

	// 这些子任务每轮随机顺序执行（名称 + 冷却配对）
	子任务 := []struct {
		名 string
		冷 int
	}{
		{"领取邮件", 10000},
		{"领取成就", 10000},
		{"福利领取", 10000},
		{"灵力修炼", 300},
		{"气运显现", 10000},
	}
	rand.Shuffle(len(子任务), func(i, j int) { 子任务[i], 子任务[j] = 子任务[j], 子任务[i] })
	for _, t := range 子任务 {
		RunTask(t.名, t.冷)
		core.RandomSleep(1000, 3000)
	}

	RunTask("背包物品", 1000)
	core.RandomSleep(1000, 3000)

	// 背包物品之后，主动做一次任务轮转弹窗清扫
	ClearPopups()

	return "ok", nil
}
