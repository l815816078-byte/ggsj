package plugin

import (
	"app/core"
)

func init() {
	Register(&加护{})
}

// 加护 插件：
//
//  1. btn循环：绿色按钮 / 确认 / 点击关闭 / 自动装备
//  2. 直接点击 184,78,280,105 区域
//  3. btn循环：召唤加护 / 跳过 / 绿色按钮 / 棕色按钮
//  4. 检测到【召唤用光】→ 直接完成整个任务（#done）
type 加护 struct{}

func (p *加护) Name() string { return "加护" }

func (p *加护) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	// 1) btn循环：绿色按钮 / 确认 / 点击关闭 / 自动装备
	ClickAdaptive(2000, 1000, nil,
		btn("绿色按钮"),
		btn("确认"),
		btn("点击关闭"),
		btn("自动装备"),
	)

	// 2) 直接点击 184,78,280,105 区域
	core.LogWithTimef("[加护] 直接点击区域 184,78,280,105")
	core.RandomClickInAreaWithDelay(184, 78, 280, 105)

	// 3) btn循环：召唤加护 / 跳过 / 绿色按钮 / 棕色按钮；【召唤用光】命中即停（不点击）
	if ClickAdaptive(2000, 1000, nil,
		btnStop("召唤用光"),
		btn("召唤加护"),
		btn("跳过"),
		btn("绿色按钮"),
		btn("棕色按钮"),
	) {
		core.LogWithTimef("[加护] 命中【召唤用光】→ 直接完成整个任务")
		return "#done", nil
	}
	core.LogWithTimef("[加护] 处理完成")
	return "ok", nil
}
