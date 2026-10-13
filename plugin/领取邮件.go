package plugin

import "app/core"

func init() {
	Register(&领取邮件{})
}

// 领取邮件 插件：点【邮件】后，逐个点【红点】，再点【完成对钩】。
type 领取邮件 struct{}

func (p *领取邮件) Name() string { return "领取邮件" }

func (p *领取邮件) Exec(ctx *Context) (string, error) {
	pts := core.ExecAll("红点", 14, 66, 598, 134)
	for _, pt := range pts {
		core.RandomClickInAreaWithDelay(pt.X, pt.Y, pt.X+10, pt.Y+10)
		ClickAdaptive(2000, 1000, nil,
			btn("完成对钩", 3), // 连续 3 轮同位置命中（点不掉）→ 后续跳过
			btn("点击关闭"),
		)
	}

	return "ok", nil
}
