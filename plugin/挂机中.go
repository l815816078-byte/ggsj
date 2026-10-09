package plugin

import "app/core"

func init() {
	Register(&挂机中{})
}

// 挂机中 插件：检测到"自动攻击开启中"即认为正在挂机中。
// 命中返回 "ok"（挂机中），没检测到返回 "no"。
type 挂机中 struct{}

func (p *挂机中) Name() string { return "挂机中" }

func (p *挂机中) Exec(ctx *Context) (string, error) {
	if core.Exec("自动攻击开启中").Found {
		core.LogWithTimef("[挂机中] 检测到自动攻击开启中 → 正在挂机")
		return "ok", nil
	}
	return "no", nil
}
