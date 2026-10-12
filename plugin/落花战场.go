package plugin

import (
	"math/rand"
	"time"

	"app/core"
)

func init() {
	Register(&落花战场{})
}

// 落花战场点位：名称 + 战力门槛 + 多个区域（点击时按顺序找可用的区域）
type 落花点位 struct {
	名称     string
	战力门槛 int
	区域列表 [][4]int
}

// 落花战场挂机点位数组
var 落花战场挂机点位数组 = []落花点位{
	{
		名称:     "点位1",
		战力门槛: 50000,
		区域列表: [][4]int{
			{208, 174, 213, 178},
			{220, 238, 224, 242},
		},
	},
	{
		名称:     "点位2",
		战力门槛: 65000,
		区域列表: [][4]int{
			{229, 238, 235, 242},
			{210, 162, 216, 170},
		},
	},
}

// 落花频道：名称 + 点击固定坐标 + 是否开启（已开启的不点，在未开启里随机选一个点）
type 落花频道 struct {
	名称     string
	X        int
	Y        int
	是否开启 bool
}

// 频道名列表（在未开启的频道里随机选一个点击）；点击直接用它对应的固定坐标
var 落花频道列表 = []落花频道{
	{"频道1", 600, 249, false},
	{"频道2", 623, 311, true},
	{"频道3", 625, 379, true},
}

// 落花战场 插件：选最高战力点位 → 点落花战场 → 黄色按钮 → 随机频道 → 花钱入场
// → 切换战场地图/取消选中 → 循环点配置区域(点不到寻路中就换下一个区域) → 怪物 → 结束
type 落花战场 struct{}

func (p *落花战场) Name() string { return "落花战场" }

func (p *落花战场) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}
	// 1) 先选点位：符合条件的最高战力；不满足任何门槛 → 直接结束
	目标点位, ok := 选落花点位()
	if !ok {
		core.LogWithTimef("[落花战场] 战力不满足任何点位门槛 → 直接结束")
		return "no", nil
	}
	core.LogWithTimef("[落花战场] 目标点位【%s】(门槛 %d)", 目标点位.名称, 目标点位.战力门槛)
	// 入口：主菜单 → 滑动 → 进入落花战场
	if !CheckLoopClick("主菜单", 6*time.Second, 2*time.Second) {
		core.LogWithTimef("[落花战场] 30秒内未找到【主菜单】→ 失败")
		return "no", nil
	}
	core.RandomSwipe(1027, 579, 990, 127, 1000, 1000)
	if !CheckLoopClick("落花战场", 6*time.Second, 2*time.Second) {
		core.LogWithTimef("[落花战场] 未找到【落花战场】→ 失败")
		return "no", nil
	}

	// 3) 点【黄色按钮】
	if !ClickRecog("黄色按钮") {
		core.LogWithTimef("[落花战场] 未找到【黄色按钮】→ 失败")
		return "no", nil
	}

	// 4) 在已开启的频道里随机选一个，直接点它的固定坐标（不需要识别后点击）
	频道, ok := 随机开启频道()
	if !ok {
		core.LogWithTimef("[落花战场] 所有频道都未开启 → 失败")
		return "no", nil
	}
	core.LogWithTimef("[落花战场] 选择频道【%s】→ 点击 (%d,%d)", 频道.名称, 频道.X, 频道.Y)
	core.RandomClickInAreaWithDelay(频道.X, 频道.Y, 频道.X+3, 频道.Y+3)

	// 5) 点【花钱入场】
	if !ClickRecog("花钱入场") {
		core.LogWithTimef("[落花战场] 未找到【花钱入场】→ 失败")
		return "no", nil
	}

	// 6) 循环查找【切换战场地图】(30秒/3秒)，找到点击退出
	if !CheckLoopClick("切换战场地图", 30*time.Second, 3*time.Second) {
		core.LogWithTimef("[落花战场] 30秒内未找到【切换战场地图】→ 失败")
		return "no", nil
	}

	// 7) 循环查找【取消选中】(6秒/3秒)，找到点击退出；没找到继续往下走
	CheckLoopClick("取消选中", 6*time.Second, 3*time.Second)

	// 8) 主循环：只检测【战场地图】→ 用配置的区域点击（点不到寻路中就换下一个区域）
	deadline := time.Now().Add(30 * time.Second)
	寻路中 := false
	for time.Now().Before(deadline) && !寻路中 {
		if IsStopped() {
			return "no", nil
		}
		core.LogWithTimef("[落花战场] 主循环 检测【战场地图】(区域 %d,%d,%d,%d)", 398, 212, 423, 234)
		if !CheckLoop("战场地图", 30*time.Second, 3*time.Second) {
			core.LogWithTimef("[落花战场] 30秒内未检测到【战场地图】→ 下一轮")
			continue
		}
		core.LogWithTimef("[落花战场] 检测到【战场地图】→ 开始按点位区域点击")
		// 依次用配置的多个区域：点一个 → 检测寻路中(10秒/2秒)；没检测到就换下一个区域
		for _, 区 := range 目标点位.区域列表 {
			if IsStopped() {
				return "no", nil
			}
			cx := (区[0] + 区[2]) / 2
			cy := (区[1] + 区[3]) / 2
			core.LogWithTimef("[落花战场] 点位【%s】点区域 %d,%d,%d,%d → 中心(%d,%d)",
				目标点位.名称, 区[0], 区[1], 区[2], 区[3], cx, cy)
			core.RandomClickInAreaWithDelay(cx, cy, cx, cy)
			if CheckLoop("寻路中", 10*time.Second, 2*time.Second) {
				core.LogWithTimef("[落花战场] 检测到【寻路中】→ 点击成功，断开所有循环")
				寻路中 = true
				break
			}
			core.LogWithTimef("[落花战场] 未检测到【寻路中】→ 换下一个区域")
		}
	}

	// 9) 循环检测【怪物】(60秒/2秒)，只找不点
	if !CheckLoop("怪物", 60*time.Second, 2*time.Second) {
		core.LogWithTimef("[落花战场] 60秒内未检测到【怪物】→ 失败")
		return "no", nil
	}

	core.LogWithTimef("[落花战场] 完成")
	return "ok", nil
}

// 选落花点位 按当前战力选符合条件的最高门槛点位；不满足任何门槛 → ok=false
func 选落花点位() (落花点位, bool) {
	pw, ok := core.GetCachedTextInt("战斗力", core.DefaultLevelCacheTTL)
	if !ok {
		core.LogWithTimef("[落花战场] 未检测到战力 → 不满足任何点位")
		return 落花点位{}, false
	}
	core.LogWithTimef("[落花战场] 当前战力=%d", *pw)

	最高 := -1
	for _, pt := range 落花战场挂机点位数组 {
		if pt.战力门槛 <= *pw && pt.战力门槛 > 最高 {
			最高 = pt.战力门槛
		}
	}
	if 最高 < 0 {
		return 落花点位{}, false
	}
	var 候选 []落花点位
	for _, pt := range 落花战场挂机点位数组 {
		if pt.战力门槛 == 最高 {
			候选 = append(候选, pt)
		}
	}
	return 候选[rand.Intn(len(候选))], true
}

// 随机开启频道 在已开启的频道里随机选一个；没有已开启的 → ok=false
func 随机开启频道() (落花频道, bool) {
	var 已开启 []落花频道
	for _, c := range 落花频道列表 {
		if c.是否开启 {
			已开启 = append(已开启, c)
		}
	}
	if len(已开启) == 0 {
		return 落花频道列表[0], false
	}
	return 已开启[rand.Intn(len(已开启))], true
}
