package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&气运显现{})
}

// 气运类型 一个气运类型：名称 + 点击区域（生命之气是默认第一个，不需要点击）
type 气运类型 struct {
	名称 string
	区域 [4]int // 点击区域 x1,y1,x2,y2（空 = 不需要点击，默认选中）
}

// 气运类型表 依次切换的类型（生命之气是默认第一个，不需要点击）
var 气运类型表 = []气运类型{
	{名称: "生命之气", 区域: [4]int{}}, // 默认第一个，不需要点击
	{名称: "黎明气息", 区域: [4]int{80, 275, 120, 315}},
	{名称: "梦幻气息", 区域: [4]int{80, 415, 121, 456}},
	{名称: "红炎气息", 区域: [4]int{82, 559, 117, 594}},
}

// 气运显现 插件：循环切换气运类型，每个类型里循环点【气运】，找不到超过 3 秒就切下一个类型。
type 气运显现 struct{}

func (p *气运显现) Name() string { return "气运显现" }

func (p *气运显现) Exec(ctx *Context) (string, error) {
	// 进入即"点了第一个类型"（生命之气默认选中），无需点击
	for _, 类型 := range 气运类型表 {
		if IsStopped() {
			return "no", nil
		}
		// 非默认类型 → 点击切换
		if 类型.区域 != [4]int{} {
			core.LogWithTimef("[气运显现] 点击气运类型【%s】(%d,%d,%d,%d)",
				类型.名称, 类型.区域[0], 类型.区域[1], 类型.区域[2], 类型.区域[3])
			core.RandomClickInAreaWithDelay(类型.区域[0], 类型.区域[1], 类型.区域[2], 类型.区域[3])
			core.RandomSleep(500, 1000)
		} else {
			core.LogWithTimef("[气运显现] 气运类型【%s】（默认第一个，无需点击）", 类型.名称)
		}

		// 循环点【气运】：找不到超过 3 秒就停止当前类型的循环，切下一个类型
		上次找到 := time.Now()
		for {
			if IsStopped() {
				return "no", nil
			}
			if time.Since(上次找到) > 6*time.Second {
				core.LogWithTimef("[气运显现] 【%s】超过6秒没找到【气运】→ 切下一个类型", 类型.名称)
				break
			}
			// 识别后点击，点击延时 600ms
			r := core.Exec("气运")
			if r.Found {
				core.LogWithTimef("[气运显现] 点击【气运】(%d,%d)", r.X, r.Y)
				core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+3, r.Y+3, 600)
				上次找到 = time.Now()
			}
			core.RandomSleep(500, 1000)
		}
	}
	return "ok", nil
}
