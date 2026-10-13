package plugin

import (
	"math/rand"
	"strconv"
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

	// 2.5) 落花商店：点【棕色按钮】进商店 → 按脚本配置逐个买物品 → 回退
	落花商店()

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
	core.RandomClickInAreaWithDelay(频道.X, 频道.Y, 频道.X+10, 频道.Y+10)

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

// 落花商店 点【棕色按钮】(区域 12,637,98,703) 打开商店 → 按脚本配置【落花商店】逐个买 → 最后点【回退】。
//
// 每个物品：
//  1. 找它并点击（点不到就从下往上滑动 177,521 → 177,233，最多 3 次；3 次都找不到 → 换下一个物品）
//  2. 购买数量默认 1，配置 N 就再点 N-1 次【加1】
//  3. 用【购买数量】取文字库读出数量文本，跟配置对上 → 点【确认】；对不上 → 点关闭(760,201,807,249)
func 落花商店() {
	物品列表 := scriptFlowerShop()
	core.LogWithTimef("[落花商店] 开始，脚本配置共 %d 个物品", len(物品列表))
	if !ClickRecog("棕色按钮", 12, 637, 98, 703) {
		core.LogWithTimef("[落花商店] 未识别到【棕色按钮】(区域 12,637,98,703) → 结束")
		return
	}
	core.LogWithTimef("[落花商店] 已点击【棕色按钮】→ 进入商店")
	for n, 物品 := range 物品列表 {
		if IsStopped() {
			core.LogWithTimef("[落花商店] 脚本已停止 → 中断")
			return
		}
		core.LogWithTimef("[落花商店] 第%d/%d 个物品【%s】，配置数量 %d", n+1, len(物品列表), 物品.Name, 物品.Qty)
		// 数量 0 → 不买：跳过检测，连物品名都不识别
		if 物品.Qty <= 0 {
			core.LogWithTimef("[落花商店] 【%s】配置数量为 0 → 不买，跳过", 物品.Name)
			continue
		}
		找到 := false
		for i := 1; i <= 3; i++ {
			if IsStopped() {
				core.LogWithTimef("[落花商店] 脚本已停止 → 中断")
				return
			}
			if ClickRecog(物品.Name) { // 偏移 118,2 在预设的"坐标偏移"里，Exec 已叠加，这里不要再传
				core.LogWithTimef("[落花商店] 第%d次找到并点击【%s】", i, 物品.Name)
				找到 = true
				break
			}
			core.LogWithTimef("[落花商店] 第%d次未找到【%s】→ 从下往上滑动 177,521 → 177,233", i, 物品.Name)
			core.RandomSwipe(177, 521, 177, 233, 500, 2000)
		}
		if !找到 {
			core.LogWithTimef("[落花商店] 滑动3次仍未找到【%s】→ 下一个物品", 物品.Name)
			continue
		}
		core.LogWithTimef("[落花商店] 【%s】数量默认 1，需要再点 %d 次【加1】", 物品.Name, 物品.Qty-1)
		for i := 1; i < 物品.Qty; i++ {
			core.LogWithTimef("[落花商店] 【%s】点第%d/%d次【加1】", 物品.Name, i, 物品.Qty-1)
			if !ClickRecog("加1") {
				core.LogWithTimef("[落花商店] 【%s】第%d次未识别到【加1】(区域 709,368,724,383)", 物品.Name, i)
			}
		}
		r := core.Exec("购买数量")
		对上 := r.Found && r.Word == strconv.Itoa(物品.Qty)
		core.LogWithTimef("[落花商店] 【%s】购买数量读到(%s) 命中=%t，配置 %d → 对上=%t", 物品.Name, r.Word, r.Found, 物品.Qty, 对上)
		if 对上 {
			core.LogWithTimef("[落花商店] 【%s】数量对上 → 点【确认】", 物品.Name)
			if !ClickRecog("确认") {
				core.LogWithTimef("[落花商店] 【%s】未识别到【确认】", 物品.Name)
			}
		} else {
			core.LogWithTimef("[落花商店] 【%s】数量对不上 → 点击关闭 760,201,807,249", 物品.Name)
			core.RandomClickInAreaWithDelay(760, 201, 807, 249)
		}
	}

	ClickRecog("购买")
	core.LogWithTimef("[落花商店] 全部物品处理完 → 点【回退】")
	if !ClickRecog("回退") {
		core.LogWithTimef("[落花商店] 未识别到【回退】(区域 29,20,56,53)")
	}
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
