package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&成长{})
}

// 成长子分类方法：无参无返回（参数用闭包固定；nil = 留空，暂不执行）
type 成长子分类方法 func()

// 成长子分类：名称 + 识别引用（点击切到该子分类用；第一个子分类默认已选中，不点击）+ 方法
type 成长子分类 struct {
	名称 string
	识别 string
	方法 成长子分类方法
}

// 成长类型：名称 + 入口识别引用（空 = 默认就在该类型，不点击切换）+ 子分类列表
type 成长类型 struct {
	名称  string
	入口  string
	子分类 []成长子分类
}

// 成长类型列表（默认一进来就在【装备-强化】界面；切换成长分类后默认选中第一个子分类）
var 成长类型列表 = []成长类型{
	{
		名称: "装备",
		入口: "", // 默认就在装备，无需点击
		子分类: []成长子分类{
			{名称: "强化", 识别: "强化", 方法: func() { 装备处理("装备-强化", "已装备", false) }}, // 默认选中
			{名称: "突破", 识别: "突破", 方法: func() { 装备处理("装备-突破", "已装备", false) }},
			{名称: "觉醒", 识别: "觉醒", 方法: func() { 装备处理("装备-觉醒", "已装备_可觉醒", true) }},
		},
	},
	{
		名称: "加护",
		入口: "成长加护",
		子分类: []成长子分类{
			{名称: "强化", 识别: "强化", 方法: func() { 装备处理("加护-强化", "已装备", false) }}, // 默认选中
			{名称: "觉醒", 识别: "觉醒", 方法: func() { 装备处理("加护-觉醒", "已装备", false) }}, // 共用同一方法
		},
	},
	{
		名称: "道术",
		入口: "成长道术",
		子分类: []成长子分类{
			{名称: "强化", 识别: "强化", 方法: func() { 道术强化("道术-强化") }}, // 默认选中
			{名称: "觉醒", 识别: "觉醒", 方法: nil},                      // 留空
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
			// 切换成长分类后默认选中第一个子分类 → 第一个点击分类(59,139,97,163)【强行切换】；其余先点子分类再点【取消成长选中】
			if i > 0 {
				if !ClickRecog(子.识别) {
					core.LogWithTimef("[成长] 未找到子分类【%s-%s】→ 跳过", 类型.名称, 子.名称)
					continue
				}
				ClickRecog("取消成长选中")
			} else {
				// 强行切换：不管哪次，第一个子分类都点一次分类
				core.LogWithTimef("[成长] 【%s】第一个子分类 → 强行切换，点击分类 59,139,97,163", 类型.名称)
				core.RandomClickInAreaWithDelay(59, 139, 97, 163)
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
//
// 装备处理 通用流程（装备/加护的强化、突破、觉醒共用）：
//
//	取【已装备】(区域在预设里) 多个 → 逐个点开 →（可选：先点【可觉醒】）→ 点【确定花钱】(先点棕色按钮)
//	标签 = 成长类型-子分类（仅用于日志，便于看出当前处理的是哪个分类）
func 装备处理(标签 string, 预设名 string, 需可觉醒 bool) {
	// 循环找【已装备】(超时3秒)，找到就断开
	var 已装备 []*core.RecogResult
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if IsStopped() {
			break
		}
		已装备 = core.ExecAll(预设名)
		if len(已装备) > 0 {
			core.LogWithTimef("[成长] 【%s】预设【%s】命中 %d 个 → 找到，断开查找", 标签, 预设名, len(已装备))
			/*	for n, p := range 已装备 {
				core.LogWithTimef("[成长] 【%s】#%d 坐标 %d,%d 矩形 %d,%d 相似度 %.2f 文字【%s】类型 %s",
					标签, n+1, p.X, p.Y, p.X2, p.Y2, p.Sim, p.Word, p.Type)
			}*/
			break
		}
	}
	if len(已装备) == 0 {
		core.LogWithTimef("[成长] 【%s】3秒内未找到 → 结束", 标签)
		return
	}

	// 逐个点开处理
	for _, pt := range 已装备 {
		if IsStopped() {
			break
		}
		core.LogWithTimef("[成长] 【%s】(需可觉醒=%v) 点击(%d,%d)", 标签, 需可觉醒, pt.X, pt.Y)
		core.RandomClickInAreaWithDelay(pt.X, pt.Y, pt.X+10, pt.Y+10)

		// 点开后再点一下【可觉醒】
		if 需可觉醒 {
			if ClickRecog("可觉醒") {
				core.LogWithTimef("[成长] 【%s】已点【可觉醒】", 标签)
			} else {
				core.LogWithTimef("[成长] 【%s】未找到【可觉醒】→ 跳过", 标签)
			}
		}

		// 最多循环 3 次
		for i := 1; i <= 3; i++ {
			if IsStopped() {
				break
			}
			// 检测到【确定花钱】→ 先点【棕色按钮】，再点【确定花钱】
			if r := core.Exec("确定花钱"); r.Found {
				core.LogWithTimef("[成长] 【%s】第%d次 检测到【确定花钱】(%d,%d) → 点【棕色按钮】(区域 297,619,353,700)", 标签, i, r.X, r.Y)
				ClickRecog("棕色按钮", 297, 619, 353, 700)
				core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+10, r.Y+10)
				core.LogWithTimef("[成长] 【%s】第%d次 已点击【确定花钱】(%d,%d)", 标签, i, r.X, r.Y)
			} else {
				core.LogWithTimef("[成长] 【%s】第%d次未检测到【确定花钱】", 标签, i)
			}
			core.RandomSleep(300, 500)
		}

		// 需可觉醒时：确定花钱点完后 → 点一下【取消成长选中】
		if 需可觉醒 {
			core.LogWithTimef("[成长] 【%s】点【取消成长选中】", 标签)
			ClickRecog("取消成长选中")
		}
	}
	core.LogWithTimef("[成长] 【%s】处理结束", 标签)
}

// 道术强化 道术-强化 子分类方法：for 红点(1198,103,1243,720) 多个数量8 → 每个点最多循环3次检测确定花钱
func 道术强化(标签 string) {
	红点 := core.ExecAll("红点8", 1198, 103, 1243, 720)
	core.LogWithTimef("[成长] 【%s】红点命中 %d 个", 标签, len(红点))
	for _, pt := range 红点 {
		if IsStopped() {
			break
		}
		core.LogWithTimef("[成长] 【%s】点击红点(%d,%d)", 标签, pt.X, pt.Y)
		core.RandomClickInAreaWithDelay(pt.X, pt.Y, pt.X+5, pt.Y+5)

		// 最多循环 3 次
		for i := 1; i <= 3; i++ {
			if IsStopped() {
				break
			}
			// 检测到【确定花钱】→ 先点【棕色按钮】，再点【确定花钱】
			if r := core.Exec("确定花钱"); r.Found {
				core.LogWithTimef("[成长] 【%s】第%d次 检测到【确定花钱】(%d,%d) → 点【棕色按钮】(区域 297,619,353,700)", 标签, i, r.X, r.Y)
				ClickRecog("棕色按钮", 297, 619, 353, 700)
				core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+10, r.Y+10)
				core.LogWithTimef("[成长] 【%s】第%d次 已点击【确定花钱】(%d,%d)", 标签, i, r.X, r.Y)
			} else {
				core.LogWithTimef("[成长] 【%s】第%d次未检测到【确定花钱】", 标签, i)
			}
			core.RandomSleep(300, 500)
		}

		// 滑动最多1次 从下往上 1021,263,1082,589
		cx := (1021 + 1082) / 2
		core.LogWithTimef("[成长] 【%s】从下往上滑动 (%d,%d)→(%d,%d)", 标签, cx, 589, cx, 263)
		core.RandomSwipe(cx, 589, cx, 263, 1500)
	}
	core.LogWithTimef("[成长] 【%s】处理结束", 标签)
}
