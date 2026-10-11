package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&福利领取{})
}

// 福利领取 插件
//
// 循环，结束条件：外层红点和内层红点都扫不到 → 结束；另有 60 秒超时兜底：
//
//	for {                                              // 60 秒超时兜底
//	    外层红点(317,116,356,647) + 内层红点(363,134,1160,591)
//	    两者都为空 → 结束
//	    否则：外层红点逐个点 → 内层红点逐个点 → 回到开头重扫
//	}
//
// 预设："红点" 走字库找字 + "寻找个数": 10，
// 靠 core.ExecAll 返回区域内全部红点（core/recog.go:417 的多结果分支）。
type 福利领取 struct{}

func (p *福利领取) Name() string { return "福利领取" }

func (p *福利领取) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	core.LogWithTimef("[福利领取] 已进入福利面板 → 开始领红点")

	// 60 秒超时兜底（挂在外层点这个循环上）
	deadline := time.Now().Add(60 * time.Second)

	// 外层点
	外层点 := core.ExecAll("红点", 317, 116, 356, 647)

	// 外层扫不到红点 → 福利结束
	if len(外层点) == 0 {
		core.LogWithTimef("[福利领取] 外层无红点 → 结束")
		return "ok", nil
	}

	for _, op := range 外层点 {
		if IsStopped() || time.Now().After(deadline) {
			break
		}
		core.LogWithTimef("[福利领取] 外层红点(%d,%d) → 点击", op.X, op.Y)
		core.RandomClickInAreaWithDelay(op.X, op.Y, op.X+3, op.Y+3)

		// 内层：点开这一项后扫内层红点，逐个点
		内层点 := core.ExecAll("红点", 362, 247, 1152, 637)
		// 内层扫不到红点 → 先走"积分奖励"，再跳过本项，继续下一个外层红点
		if len(内层点) == 0 {
			core.LogWithTimef("[福利领取] 内层无红点 → 跳过本项")
			积分奖励(deadline)
			continue
		}
		for _, ip := range 内层点 {
			if IsStopped() || time.Now().After(deadline) {
				break
			}
			core.LogWithTimef("[福利领取] 内层红点(%d,%d) → 点击", ip.X, ip.Y)
			core.RandomClickInAreaWithDelay(ip.X, ip.Y, ip.X+3, ip.Y+3, 3000)
		}

		// 内层红点点击完之后：走"天数奖励"
		天数奖励(deadline)

		积分奖励(deadline)
	}

	core.LogWithTimef("[福利领取] 处理完成")
	return "ok", nil
}

// 天数奖励 是否翻页：翻页共 6 次（前 3 次从下往上，后 3 次从上往下），每次翻完重扫红点点掉
func 天数奖励(deadline time.Time) {
	for i := 1; i <= 6; i++ {
		if IsStopped() || time.Now().After(deadline) {
			break
		}
		if !core.Exec("是否翻页").Found {
			break
		}
		if i <= 3 {
			core.LogWithTimef("[福利领取] 从下往上翻页 %d/3", i)
			core.RandomSwipe(594, 557, 594, 357, 1500)
		} else {
			core.LogWithTimef("[福利领取] 从上往下翻页 %d/3", i-3)
			core.RandomSwipe(594, 357, 594, 557, 1000)
		}

		翻页红点 := core.ExecAll("红点", 373, 311, 950, 641)
		for _, p := range 翻页红点 {
			if IsStopped() || time.Now().After(deadline) {
				break
			}
			core.LogWithTimef("[福利领取] 翻页后红点(%d,%d) → 点击", p.X, p.Y)
			core.RandomClickInAreaWithDelay(p.X, p.Y, p.X+3, p.Y+3, 3000)
		}
	}
}

// 积分奖励 是否下拉：循环下拉并扫红点点击（最多 6 次）
func 积分奖励(deadline time.Time) {
	for i := 1; i <= 6; i++ {
		if IsStopped() || time.Now().After(deadline) {
			break
		}
		if !core.Exec("是否下拉").Found {
			break
		}
		core.LogWithTimef("[福利领取] 从下往上翻页 %d/6", i)
		core.RandomSwipe(1062, 547, 1064, 407, 1500)
		翻页红点 := core.ExecAll("红点", 968, 353, 1066, 639)
		for _, p := range 翻页红点 {
			if IsStopped() || time.Now().After(deadline) {
				break
			}
			core.LogWithTimef("[福利领取] 翻页后红点(%d,%d) → 点击", p.X, p.Y)
			core.RandomClickInAreaWithDelay(p.X, p.Y, p.X+3, p.Y+3, 3000)
		}
	}
}
