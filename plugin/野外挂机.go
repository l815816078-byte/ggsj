package plugin

import (
	"math/rand"

	"app/core"
)

func init() {
	Register(&野外挂机{})
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

	if !去地图(配置, 地点) {
		core.LogWithTimef("[野外挂机] 去地图失败 → 返回失败")
		return "no", nil
	}
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
