package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&自动攻击{})
}

// 自动攻击 插件：确保"自动攻击开启中"。
//
// 循环（超时 3 秒、间隔 1 秒）：
//   - 检测到"自动攻击开启中" → 已开启，退出循环
//   - 否则 → 点一次"自动攻击"按钮，等 1 秒再检测
//
// 开启成功返回 "ok"，3 秒内没能开启返回 "no"。
type 自动攻击 struct{}

func (p *自动攻击) Name() string { return "自动攻击" }

func (p *自动攻击) Exec(ctx *Context) (string, error) {
	deadline := time.Now().Add(10 * time.Second)
	已清弹窗 := false
	for {
		if IsStopped() || IsTaskCancelled() {
			return "no", nil
		}
		if core.Exec("自动攻击开启中").Found {
			core.LogWithTimef("[自动攻击] 自动攻击已开启")
			return "ok", nil
		}
		if time.Now().After(deadline) {
			core.LogWithTimef("[自动攻击] 10 秒内未开启自动攻击 → 失败")
			return "no", nil
		}
		core.LogWithTimef("[自动攻击] 未开启 → 点击自动攻击")
		// 连按钮都识别不到，多半是被弹窗挡住了 → 清一次任务轮转弹窗再试。
		// 只清一次：清扫本身要几秒，每轮都清会把 10 秒预算吃光，反而开不了自动攻击。
		if !ClickRecog("自动攻击") && !已清弹窗 {
			已清弹窗 = true
			core.LogWithTimef("[自动攻击] 未识别到【自动攻击】→ 清一次任务轮转弹窗再试")
			ClearPopups()
		}
		core.RandomSleep(1000, 1000)
	}
}
