package plugin

import (
	"app/core"
)

func init() {
	Register(&成长{})
}

// 成长子分类方法：无参无返回（每个子分类配一个方法；nil = 留空，暂不执行）
type 成长子分类方法 func()

// 成长子分类：名称 + 识别引用（点击切到该子分类用；第一个子分类默认已选中，不点击）+ 方法
type 成长子分类 struct {
	名称 string
	识别 string
	方法 成长子分类方法
}

// 成长类型：名称 + 入口识别引用（空 = 默认就在该类型，不点击切换）+ 子分类列表
type 成长类型 struct {
	名称   string
	入口   string
	子分类 []成长子分类
}

// 成长类型列表（默认一进来就在【装备-强化】界面；切换成长分类后默认选中第一个子分类）
var 成长类型列表 = []成长类型{
	{
		名称: "装备",
		入口: "", // 默认就在装备，无需点击
		子分类: []成长子分类{
			{名称: "强化", 识别: "强化", 方法: 装备强化}, // 默认选中
			{名称: "觉醒", 识别: "觉醒", 方法: nil},      // 留空
			{名称: "突破", 识别: "突破", 方法: nil},      // 留空
		},
	},
	{
		名称: "加护",
		入口: "成长加护",
		子分类: []成长子分类{
			{名称: "强化", 识别: "强化", 方法: nil}, // 默认选中，留空
			{名称: "觉醒", 识别: "觉醒", 方法: nil}, // 留空
		},
	},
	{
		名称: "道术",
		入口: "成长道术",
		子分类: []成长子分类{
			{名称: "强化", 识别: "强化", 方法: nil}, // 默认选中，留空
			{名称: "觉醒", 识别: "觉醒", 方法: nil}, // 留空
		},
	},
}

// 成长 插件：遍历所有成长类型的所有子分类，执行对应方法
type 成长 struct{}

func (p *成长) Name() string { return "成长" }

func (p *成长) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	for _, 类型 := range 成长类型列表 {
		if IsStopped() {
			return "no", nil
		}
		// 每个成长类型之间间隔 2-3 秒
		core.RandomSleep(2000, 3000)
		// 切到该成长类型（入口为空 = 默认就在该类型，不点击）
		if 类型.入口 != "" {
			if !ClickRecog(类型.入口) {
				core.LogWithTimef("[成长] 未找到成长类型【%s】→ 跳过", 类型.名称)
				continue
			}
			core.LogWithTimef("[成长] 已切到成长类型【%s】", 类型.名称)
		}

		for i, 子 := range 类型.子分类 {
			if IsStopped() {
				return "no", nil
			}
			// 切换成长分类后默认选中第一个子分类 → 第一个不点击，其余点击切换
			if i > 0 {
				if !ClickRecog(子.识别) {
					core.LogWithTimef("[成长] 未找到子分类【%s-%s】→ 跳过", 类型.名称, 子.名称)
					continue
				}
			}
			if 子.方法 == nil {
				core.LogWithTimef("[成长] 【%s-%s】方法未配置 → 跳过", 类型.名称, 子.名称)
				continue
			}
			core.LogWithTimef("[成长] 执行【%s-%s】", 类型.名称, 子.名称)
			子.方法()
		}
	}

	core.LogWithTimef("[成长] 全部完成")
	return "ok", nil
}

// 装备强化 装备-强化 子分类方法：
//
//	for 已装备(920,126,1209,720) 多个数量12 {
//	    循环 3 次（间隔1秒）：检测到【确定花钱】→ 点【棕色按钮】→ 再点【确定花钱】
//	}
func 装备强化() {
	已装备 := core.ExecAll("已装备", 920, 126, 1209, 720)
	for _, pt := range 已装备 {
		if IsStopped() {
			break
		}
		core.LogWithTimef("[成长] 已装备(%d,%d) → 点击", pt.X, pt.Y)
		core.RandomClickInAreaWithDelay(pt.X, pt.Y, pt.X+3, pt.Y+3)

		// 最多循环 3 次，间隔 1 秒
		for i := 1; i <= 3; i++ {
			if IsStopped() {
				break
			}
			// 检测到【确定花钱】→ 先点【棕色按钮】(297,619,353,700)，再点【确定花钱】
			if r := core.Exec("确定花钱"); r.Found {
				core.LogWithTimef("[成长] 检测到【确定花钱】→ 点棕色按钮 297,619,353,700")
				core.RandomClickInAreaWithDelay(297, 619, 353, 700)
				core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+3, r.Y+3)
			}
			core.RandomSleep(1000, 1000)
		}
	}
}
