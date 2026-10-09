package plugin

import (
	"app/core"
)

func init() {
	Register(&刷新信息{})
}

// 刷新信息 插件：任务启动第一步调用，强制真识别一次"战斗力"和"等级"并刷新缓存，
// 让后续门槛判断（如野外挂机选点的战力门槛）使用最新数值。
// 识别失败不阻塞（保留旧缓存），返回 ok 继续流程。
type 刷新信息 struct{}

func (p *刷新信息) Name() string { return "刷新信息" }

func (p *刷新信息) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	// 1. 刷新战斗力（真识别并更新缓存，识别失败保留旧缓存）
	if pw, ok := core.RefreshTextInt("战斗力"); ok {
		core.LogWithTimef("[刷新信息] 战斗力刷新成功: %d", *pw)
	} else {
		core.LogWithTimef("[刷新信息] 战斗力识别失败 → 保留旧缓存")
	}
	return "ok", nil
}
