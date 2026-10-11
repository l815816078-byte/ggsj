package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&支线任务{})
}

// 支线任务 插件：点【支线任务】后，用 ClickAdaptive 自适应处理
// 接受 / 确认 / 立即传送 / 绿色按钮（= 原来的【咒术传送】）/ 引导手势 / 引导提示；
// 完事后循环直接调用【躲避技能】插件（超时 10 秒，间隔 2 秒）。
type 支线任务 struct{}

func (p *支线任务) Name() string { return "支线任务" }

func (p *支线任务) Exec(ctx *Context) (string, error) {
	ClickRecog("支线任务")
	ClickAdaptive(2000, 1000, nil,
		btn("接受", 3),
		btn("确认", 3),
		btn("立即传送", 3),
		btn("绿色按钮", 3), // 绿色按钮 = 原来的【咒术传送】
		btn("引导提示"),
		btn("引导手势"),
		btn("引导提示"),
	)

	// 立即传送成功后 → 查找点击打开【绿色按钮】
	ClickRecog("")

	// 完事后循环直接调用【躲避技能】插件（超时 10 秒，间隔 2 秒）
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if IsStopped() {
			break
		}
		if bp := Get("躲避技能"); bp != nil {
			bp.Exec(ctx)
		}
		core.RandomSleep(2000, 2000)
	}

	return "ok", nil
}
