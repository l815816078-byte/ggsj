package plugin

import (
	"math/rand"
	"time"

	"app/core"
)

func init() {
	Register(&野外挂机{})
	Register(&挂机中{})
}

// 挂机点 一个挂机点：名称（recogs/字库预设名）+ 战力门槛
type 挂机点 struct {
	名称     string
	战力门槛 int
}

// 挂机点表 可选挂机点（名称 + 战力门槛）
var 挂机点表 = []挂机点{
	{名称: "倒塌的据点", 战力门槛: 50000},
	{名称: "废弃通道", 战力门槛: 50000},
	{名称: "基土小路", 战力门槛: 38000},
	{名称: "不详的湖", 战力门槛: 38000},
	{名称: "高处岔路", 战力门槛: 25000},
}

// 找挂机点名称的检测范围（地图界面挂机列表）[x1,y1,x2,y2]
var 找点区域 = [4]int{74, 100, 340, 720}

// 找不到时的滑动起终点 [x,y]（从下往上）
var (
	找点滑动起点 = [2]int{187, 669}
	找点滑动终点 = [2]int{185, 123}
)

const (
	找点滑动时长 = 2000
	// 最多滑动次数
	找点最多滑动 = 6
)

// 野外挂机 插件：选一个最符合当前战力的挂机点 → 循环查找并点击（地图界面由前置步骤打开）。
//
// 流程：
//  1. 检测战力，选出目标挂机点（见 选挂机点）
//  2. 循环（最多 找点最多滑动 次）在区域 74,100,340,720 找目标挂机点名称：
//     找到 → 点它 → 咒术传送(没找到则前往) → 循环检测"立即传送"(超时15秒, 命中刷新超时) → ok
//     找不到 → 检测【收藏】，命中才滑动（187,669 → 185,123）；没命中 → 结束查找
//
// 命中挂机点返回 "ok"，没找到返回 "no"。
type 野外挂机 struct{}

func (p *野外挂机) Name() string { return "野外挂机" }

func (p *野外挂机) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	// 1) 检测战力 → 选目标挂机点
	target := 选挂机点()
	core.LogWithTimef("[野外挂机] 目标挂机点【%s】(战力门槛%d)", target.名称, target.战力门槛)

	// 2) 循环查找挂机点名称；找不到 → 检测【收藏】，命中才滑动
	for i := 1; i <= 找点最多滑动; i++ {
		if IsStopped() {
			return "no", nil
		}

		r := core.Exec(target.名称, 找点区域[:]...)
		if r.Found {
			core.LogWithTimef("[野外挂机] 第%d/%d次: 找到【%s】(%d,%d) → 点击",
				i, 找点最多滑动, target.名称, r.X, r.Y)
			core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+3, r.Y+3)

			// 点击挂机点后：优先【咒术传送】；没找到才【前往】，并循环检测【立即传送】
			if !ClickRecog("咒术传送") {
				core.LogWithTimef("[野外挂机] 未找到【咒术传送】→ 点【前往】")
				ClickRecog("前往")

				// 循环检测【立即传送】(超时15秒)：检测到就点它并刷新超时时间
				deadline := time.Now().Add(15 * time.Second)
				for time.Now().Before(deadline) {
					if IsStopped() {
						break
					}
					if ClickRecog("立即传送") {
						// 立即传送成功 → 查找点击打开【咒术传送】，然后断开循环
						ClickRecog("咒术传送")
						break
					}
					core.RandomSleep(500, 1000)
				}
			}
			return "ok", nil
		}
		core.LogWithTimef("[野外挂机] 第%d/%d次: 未找到【%s】→ 检测【收藏】",
			i, 找点最多滑动, target.名称)

		if !core.Exec("收藏").Found {
			core.LogWithTimef("[野外挂机] 未检测到【收藏】→ 不能滑动，结束查找")
			break
		}
		core.LogWithTimef("[野外挂机] 检测到【收藏】→ 滑动 %d,%d → %d,%d",
			找点滑动起点[0], 找点滑动起点[1], 找点滑动终点[0], 找点滑动终点[1])
		core.SwipeStraight(找点滑动起点[0], 找点滑动起点[1], 找点滑动终点[0], 找点滑动终点[1], 找点滑动时长)
		core.RandomSleep(1500, 2000)
	}

	core.LogWithTimef("[野外挂机] 未找到【%s】→ 失败", target.名称)
	return "no", nil
}

// 选挂机点 选一个最符合当前战力的挂机点：
//   - 战力满足的点里取"门槛最高"的；同门槛有多个 → 随机一个
//   - 检测不到战力 / 战力一个门槛都不满足 → 取门槛最低的（同最低多个 → 随机一个）
func 选挂机点() 挂机点 {
	pw, ok := core.GetCachedTextInt("战斗力", core.DefaultLevelCacheTTL)
	if !ok {
		core.LogWithTimef("[野外挂机] 未检测到战力 → 取门槛最低的挂机点")
		return 最低门槛挂机点()
	}
	core.LogWithTimef("[野外挂机] 当前战力=%d", *pw)

	// 满足条件（门槛 ≤ 战力）里门槛最高的
	最高 := -1
	for _, pt := range 挂机点表 {
		if pt.战力门槛 <= *pw && pt.战力门槛 > 最高 {
			最高 = pt.战力门槛
		}
	}
	if 最高 < 0 {
		core.LogWithTimef("[野外挂机] 战力%d 不满足任何门槛 → 取门槛最低的挂机点", *pw)
		return 最低门槛挂机点()
	}

	var 候选 []挂机点
	for _, pt := range 挂机点表 {
		if pt.战力门槛 == 最高 {
			候选 = append(候选, pt)
		}
	}
	return 候选[rand.Intn(len(候选))]
}

// 最低门槛挂机点 战力门槛最低的那个（同最低有多个 → 随机一个）
func 最低门槛挂机点() 挂机点 {
	最低 := 挂机点表[0].战力门槛
	for _, pt := range 挂机点表 {
		if pt.战力门槛 < 最低 {
			最低 = pt.战力门槛
		}
	}
	var 候选 []挂机点
	for _, pt := range 挂机点表 {
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

	RunTask("领取邮件", 3000)
	core.RandomSleep(1000, 3000)
	RunTask("领取成就", 3000)
	core.RandomSleep(1000, 3000)
	RunTask("福利领取", 3000)
	core.RandomSleep(1000, 3000)
	RunTask("灵力修炼", 300)
	core.RandomSleep(1000, 3000)
	RunTask("气运显现", 3000)
	core.RandomSleep(1000, 3000)
	RunTask("背包物品", 1000)
	core.RandomSleep(1000, 3000)

	// 背包物品之后，主动做一次任务轮转弹窗清扫
	ClearPopups()

	return "ok", nil
}
