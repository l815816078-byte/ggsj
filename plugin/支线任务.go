package plugin

import "app/core"

func init() {
	Register(&支线任务{})
}

// 支线任务 插件：点【支线任务】后，用 ClickAdaptive 自适应处理
// 接受 / 确认 / 立即传送 / 引导手势 / 引导提示。
type 支线任务 struct{}

func (p *支线任务) Name() string { return "支线任务" }

func (p *支线任务) Exec(ctx *Context) (string, error) {
	ClickRecog("支线任务")
	ClickAdaptive(2000, 1000, nil,
		btn("接受"),
		btn("确认"),
		btn("立即传送"),
		btn("引导手势"),
		btn("引导提示"),
	)
	return "ok", nil
}
