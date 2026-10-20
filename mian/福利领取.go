package mian

import (
	"time"

	"app/core"
)

// ══════════════════════════════════════════════════════════════════════════
// 福利领取 —— 子任务（照 plugin/福利领取.go：一轮之内全部做完）
//
// 流程：
//
//	点【福利】→ 认【福利列表】→ 外层红点多个(317,116,356,647) 逐个点
//	→ 每个点完：内层红点多个(362,247,1152,637) 逐个点（点完延时 3 秒）
//	→ 天数奖励（是否翻页 → 翻 6 页：前 3 页从下往上、后 3 页从上往下，每次翻完扫红点(373,311,950,641)点掉）
//	→ 积分奖励（是否下拉 → 下拉 6 次，每次下拉完扫红点(968,353,1066,639)点掉）
//	→ 外层红点全点完 → 点【退出】→ 完成
//
// 结束条件：外层红点扫不到（=0 也成立 → 直接退）；另有 60 秒 deadline 兜底。
// ══════════════════════════════════════════════════════════════════════════

// ── 福利领取自己的状态 ────────────────────────────────────────────────────
var 福利领取已完成 = false // 外层红点全点完 + 退出点了

// 福利领取重置 由 跑子任务() 在【完成后】调（超时不清 → 下次回来接着干）
func 福利领取重置() {
	福利领取已完成 = false
}

// 福利领取完成 = 福利红点全领完（状态走到尽头）
func 福利领取完成() bool { return 福利领取已完成 }

// 福利领取跑 子任务流程（每轮由 跑子任务() 调度，一轮之内全部做完）
// 顺序：点【福利】→ 认【福利列表】→ 一次点完所有红点（带翻页/下拉）→ 点【退出】
func 福利领取跑() {
	查找点击延时("福利")
	if 是否存在("福利列表") {
		// 60 秒 deadline 兜底（照 plugin）
		deadline := time.Now().Add(60 * time.Second)

		// 外层红点多个：一次全部点完（点开一个就把它内层的也点掉）
		外层点 := 识别多个("红点", 317, 116, 356, 647)
		core.LogWithTimef("[福利领取] 外层红点区域(317,116,356,647) 识别到 %d 个", len(外层点))

		for _, 点 := range 外层点 {
			if 已停止() || time.Now().After(deadline) {
				break
			}
			core.LogWithTimef("[福利领取] 外层红点(%d,%d) → 点击", 点.X, 点.Y)
			core.RandomClickInAreaWithDelay(点.X, 点.Y, 点.X+5, 点.Y+5)

			// 内层：点开这一项后扫内层红点，逐个点（点完延时 3 秒）
			内层点 := 识别多个("红点", 362, 247, 1152, 637)
			// 内层扫不到红点 → 先走"积分奖励"，再跳过本项，继续下一个外层红点
			if len(内层点) == 0 {
				core.LogWithTimef("[福利领取] 内层无红点 → 跳过本项")
				积分奖励(deadline)
				continue
			}
			for _, 内点 := range 内层点 {
				if 已停止() || time.Now().After(deadline) {
					break
				}
				core.LogWithTimef("[福利领取] 内层红点(%d,%d) → 点击", 内点.X, 内点.Y)
				core.RandomClickInAreaWithDelay(内点.X, 内点.Y, 内点.X+3, 内点.Y+3)
				查找点击延时("点击关闭")
			}

			// 内层红点点击完之后：走"天数奖励" + "积分奖励"
			天数奖励(deadline)
			积分奖励(deadline)
			core.RandomSleep(1000, 1500)
		}

		//   但必须点中才算完成：点不中 = 面板还开着，下轮接着来
		core.LogWithTimef("[福利领取] 福利红点已全部领取（%d 个）→ 点【退出】", len(外层点))
		if 查找点击延时("关闭") {
			福利领取已完成 = true
			core.LogWithTimef("[福利领取] ✅ 完成")
		}
	}
}

// 天数奖励 是否翻页：翻页共 6 次（前 3 次从下往上、后 3 页从上往下），每次翻完重扫红点点掉
func 天数奖励(deadline time.Time) {
	core.LogWithTimef("[福利领取] 天数奖励：进入")
	m := 滑动表["福利天数"]
	for i := 1; i <= m.次数; i++ {
		if 已停止() {
			core.LogWithTimef("[福利领取] 天数奖励：已停止 → 结束")
			break
		}
		if time.Now().After(deadline) {
			core.LogWithTimef("[福利领取] 天数奖励：超 60 秒 deadline → 结束")
			break
		}
		if !是否存在("是否翻页") {
			core.LogWithTimef("[福利领取] 天数奖励：没有【是否翻页】→ 结束")
			break
		}
		if m.前置验证 == "" || 是否存在(m.前置验证) {
			if i <= 3 {
				core.LogWithTimef("[福利领取] 从下往上翻页 %d/3", i)
				core.RandomSwipe(m.起点X, m.起点Y, m.终点X, m.终点Y, m.时长)
			} else {
				core.LogWithTimef("[福利领取] 从上往下翻页 %d/3", i-3)
				core.RandomSwipe(m.起点X, m.终点Y, m.终点X, m.起点Y, 1000)
			}
		}

		翻页红点 := 识别多个("红点", 373, 311, 950, 641)
		for _, 点 := range 翻页红点 {
			if 已停止() || time.Now().After(deadline) {
				break
			}
			core.LogWithTimef("[福利领取] 翻页后红点(%d,%d) → 点击", 点.X, 点.Y)
			core.RandomClickInAreaWithDelay(点.X, 点.Y, 点.X+3, 点.Y+3, 3000)
			查找点击延时("点击关闭")
		}
	}
}

// 积分奖励 是否下拉：循环下拉并扫红点点击（最多 6 次）
func 积分奖励(deadline time.Time) {
	core.LogWithTimef("[福利领取] 积分奖励：进入")
	m := 滑动表["福利积分"]
	for i := 1; i <= m.次数; i++ {
		if 已停止() {
			core.LogWithTimef("[福利领取] 积分奖励：已停止 → 结束")
			break
		}
		if time.Now().After(deadline) {
			core.LogWithTimef("[福利领取] 积分奖励：超 60 秒 deadline → 结束")
			break
		}
		if !是否存在("是否下拉") {
			core.LogWithTimef("[福利领取] 积分奖励：没有【是否下拉】→ 结束")
			break
		}
		if m.前置验证 == "" || 是否存在(m.前置验证) {
			core.LogWithTimef("[福利领取] 下拉 %d/%d", i, m.次数)
			core.RandomSwipe(m.起点X, m.起点Y, m.终点X, m.终点Y, m.时长)
		}

		翻页红点 := 识别多个("红点", 968, 353, 1066, 639)
		for _, 点 := range 翻页红点 {
			if 已停止() || time.Now().After(deadline) {
				break
			}
			core.LogWithTimef("[福利领取] 翻页后红点(%d,%d) → 点击", 点.X, 点.Y)
			core.RandomClickInAreaWithDelay(点.X, 点.Y, 点.X+3, 点.Y+3)
			查找点击延时("点击关闭")
		}
	}
}
