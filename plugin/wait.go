package plugin

import (
	"time"

	"app/core"
)

// CheckLoop 场景循环检测（只检测，不点击）：timeout 内每隔 interval 检测一次，命中一次立即返回 true，超时返回 false。
// 可选 region [x1,y1,x2,y2]：传入时覆盖预设区域。
// 用途：检测"当前是什么场景/在哪一页"（缔造师挑战、挑战任务、活动、背包列表…），
// 画面切换需要时间，单次检测容易漏，循环检测更稳。
func CheckLoop(name string, timeout, interval time.Duration, region ...int) bool {
	end := time.Now().Add(timeout)
	for {
		if core.Exec(name, region...).Found {
			return true
		}
		if time.Now().After(end) {
			return false
		}
		core.RandomSleep(int(interval.Milliseconds()), int(interval.Milliseconds()))
	}
}

// CheckLoopClick 场景循环检测（检测到就点）：timeout 内每隔 interval 检测一次，命中就点它一次并立即返回 true，超时返回 false。
// 可选 region [x1,y1,x2,y2]：传入时覆盖预设区域。
// 用途：检测到某场景/按钮就点（回退、商店…），复用 ClickRecog。
func CheckLoopClick(name string, timeout, interval time.Duration, region ...int) bool {
	end := time.Now().Add(timeout)
	for {
		if ClickRecog(name, region...) {
			return true
		}
		if time.Now().After(end) {
			return false
		}
		core.RandomSleep(int(interval.Milliseconds()), int(interval.Milliseconds()))
	}
}

// ClickRecog 识别指定预设，命中则点击它返回的坐标，返回是否命中。
//
// 用途：插件里"要找某个按钮/元素并点它"的通用写法。识别预设配了"返回中间坐标": true 时
// Exec 返回的 X/Y 就是目标中心，直接点即可；未命中则不点、返回 false。
//
// 可选变参 region [x1,y1,x2,y2]：传入时用该范围覆盖预设的"截图区域"（浅拷贝，不影响
// 全局预设），用于"同一个预设在不同场景要找不同区域"（如全屏找"关闭"会误识别，
// 某场景限定在弹窗范围内找）。不传则用预设自带区域。
//
// 注意：点击用 core.RandomClickInAreaWithDelay(x, y, x+3, y+3)（小范围随机，防被识别成
// 固定点），并自带点击延迟；不要在外面再补 core.RandomSleep（除业务确需的步骤间隔外）。
//
//	plugin.ClickRecog("使用")                    // 用预设自带区域识别"使用"并点它
//	plugin.ClickRecog("关闭", 331, 11, 461, 152) // 限定在 (331,11,461,152) 内找"关闭"并点它
func ClickRecog(name string, region ...int) bool {
	return ClickRecogOffset(name, 0, 0, region...)
}

// ClickRecogOffset 同 ClickRecog，但点击坐标额外加偏移 (offX, offY)：
// 实际点击 (命中坐标 + 偏移)。识别区域仍由 region 决定。
//
//	plugin.ClickRecogOffset("红点", -655, -192)             // 用预设区域，点命中坐标+偏移
//	plugin.ClickRecogOffset("红点", 0, 40, 106, 70, 1187, 713) // 指定区域 + 偏移
func ClickRecogOffset(name string, offX, offY int, region ...int) bool {
	r := core.Exec(name, region...)
	if !r.Found {
		core.LogWithTimef("[Plugin] 未识别到【%s】，跳过点击", name)
		return false
	}
	x, y := r.X+offX, r.Y+offY
	core.LogWithTimef("[Plugin] 点击【%s】(%d,%d)", name, x, y)
	core.RandomClickInAreaWithDelay(x, y, x+3, y+3)
	return true
}

// adaptiveSkipTol 稳定命中抑制的"同位置"容差（像素）：两次命中坐标在 ±该值内视为同一位置
// （识别坐标会有抖动，完全相同判不出来，所以用容差）。
const adaptiveSkipTol = 10

// absInt 整数绝对值
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Btn 自适应按钮项：预设名 +（可选）查找区域 / 结束标记 / 稳定命中抑制 / 跳过条件 / 点击偏移。
type Btn struct {
	Name   string // 识别预设名
	Region []int  // [x1,y1,x2,y2]，nil = 用预设自带"截图区域"
	Done   bool   // true = 结束按钮：命中就点它并停止循环
	// SkipStreak > 0 = 启用"稳定命中抑制"：该按钮连续这么多轮在**完全相同位置**命中，
	// 则认为它是点不掉的误识别，本次调用内后续所有轮都跳过它。0 = 不启用。
	SkipStreak int
	// SkipIfRecog 非空 = 跳过条件：命中本按钮后，若该预设也命中（如"对钩"=已选中），
	// 则**不点**本按钮，并**本轮视为没命中**继续扫后面的按钮（不再重截重来，避免画面不变
	// 导致死循环）；全部按钮都跳过/未命中 → 走正常空闲判定（超时才结束）。
	SkipIfRecog string
	// ClickOffset = 点击偏移 [dx,dy]：命中后点击"命中坐标 + 偏移"（(x+dx, y+dy)）。
	// 零值 [0,0] = 不偏移（点命中点本身）。
	ClickOffset [2]int
	// Stop = true 表示"命中即停"：命中后**不点击**本按钮，并让 ClickAdaptive
	// **立即返回 true**（不再点其他按钮、不等超时）。用 btnStop(name, ...) 构造。
	// 用途：检测到某标志（如【召唤用光】）即视为完成，且绝不能点它。
	Stop bool
}

// btn 构造自适应按钮项，可选参数按类型/顺序解析（都传基本类型，无需自定义标记）：
//   - 4 个连续的 int（第1~4） → 识别区域 [x1,y1,x2,y2]
//   - 再 1 个 int（第5个）    → 稳定命中抑制轮数（连续多少轮同位置命中就跳过）
//   - 再 2 个 int（第6、7个） → 点击偏移 [dx,dy]（点"命中坐标 + 偏移"）
//   - bool                    → true = 结束按钮
//   - string                  → 跳过条件预设名（命中它就不点本按钮，本轮继续扫后面的按钮）
//
//	btn("今日不再显示", "对钩")                // 命中"今日不再显示"但检测到"对钩"(已选中) → 跳过不点
//	btn("红点")                              // 用预设区域
//	btn("红点", 18,306,368,546)              // 指定区域
//	btn("红点", 18,306,368,546, 3)           // 区域 + 抑制(连续3轮同位置→跳过)
//	btn("红点", 18,306,368,546, 3, -655,-192) // 区域 + 抑制 + 点击偏移
//	btn("确定", true)                         // 结束按钮
func btn(name string, opts ...any) Btn {
	b := Btn{Name: name}
	var ints []int
	for _, o := range opts {
		switch v := o.(type) {
		case bool:
			b.Done = v
		case int:
			ints = append(ints, v)
		case string:
			b.SkipIfRecog = v // string = 跳过条件预设名
		}
	}
	if len(ints) >= 4 {
		b.Region = []int{ints[0], ints[1], ints[2], ints[3]}
		if len(ints) >= 5 {
			b.SkipStreak = ints[4] // 第 5 个 int = 抑制轮数
		}
		if len(ints) >= 7 {
			b.ClickOffset = [2]int{ints[5], ints[6]} // 第 6、7 个 int = 点击偏移 [dx,dy]
		}
	} else if len(ints) == 1 {
		b.SkipStreak = ints[0] // 只传 1 个 int = 抑制轮数（不指定区域）
	}
	return b
}

// btnStop 构造"命中即停"按钮项：命中后**不点击**它，ClickAdaptive **立即返回 true**。
// 参数同 btn（区域 / 跳过条件等均可用）。
//
//	btnStop("召唤用光")                        // 检测到就用（不点）→ ClickAdaptive 立即返回 true
//	btnStop("召唤用光", 453,644,552,682)       // 指定区域
func btnStop(name string, opts ...any) Btn {
	b := btn(name, opts...)
	b.Stop = true
	return b
}

// ClickAdaptive 自适应找按钮（通用）：循环按顺序给出的一组按钮，**按顺序一个一个找、
// 找到一个就点一个**（从头往后逐个识别，命中就点它，点完继续往后找下一个）；
// **不管是否还在持续命中，非点击延时时间累计满 timeoutMs 就强制结束**。
// onHit 可选（nil = 不回调）：每点击一个按钮，用**被点按钮的预设名**回调一次。
//
// 场景：点某个按钮后，接下来弹出的按钮不固定（例如"点【使用】后可能直接出【打开】，
// 也可能先出【点击关闭】再出【使用】"），不能写死顺序 —— 用这个函数自适应。
//
// 超时是"**硬超时（不含点击延时）**"：从进入函数起计时，识别/等待时间累计满 timeoutMs 就结束；
// **每次点击自带的"点击后延时"不计入超时预算**（点击后把该次点击耗时补回 deadline）；
// 命中结束按钮不提前返回，只做标记。
//
// 参数：
//   - timeoutMs   硬超时（毫秒）：识别/等待时间累计达到它就结束（点击延时不算）
//   - intervalMs  每轮检测之间的等待（毫秒，取值 [intervalMs/2, intervalMs]）
//   - onHit       可选回调（传 nil = 不回调）：func(name string)
//   - btns        按钮项列表（用 btn 构造，**每轮按顺序逐个识别、命中即点**）
//
// 返回：本次自适应是否命中过任一结束按钮（true=命中过；false=没命中/被停止）。
func ClickAdaptive(timeoutMs, intervalMs int, onHit func(name string), btns ...Btn) bool {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	// 每个按钮"连续同位置命中"的追踪 + 已跳过集合（按按钮下标 i 区分，同名不同区域互不影响）
	type track struct{ x, y, streak int }
	tr := make(map[int]*track)
	skipped := make(map[int]bool)

	// 是否命中过结束按钮（只做标记，不提前结束）
	命中结束 := false

	// 超时只看识别/等待时间（点击耗时会在点击后补回 deadline）。
	for time.Now().Before(deadline) {
		if IsStopped() {
			return false
		}

		// 每轮按顺序逐个识别：命中就点它，点完继续往后找下一个（不先全扫再统一点）。
		本轮命中 := false
		for i := range btns {
			if IsStopped() {
				return false
			}
			// 到点立即结束（点击延时已不计入 deadline），不等本轮识别完
			if time.Now().After(deadline) {
				break
			}
			if skipped[i] {
				continue
			}
			r := core.Exec(btns[i].Name, btns[i].Region...)
			if !r.Found {
				continue
			}
			if btns[i].SkipIfRecog != "" && core.Exec(btns[i].SkipIfRecog).Found {
				core.LogWithTimef("[Plugin] 自适应：【%s】命中但检测到【%s】（已选中）→ 跳过该按钮",
					btns[i].Name, btns[i].SkipIfRecog)
				continue
			}

			// 稳定命中抑制：连续 N 轮同位置命中 → 跳过
			if btns[i].SkipStreak > 0 {
				t := tr[i]
				if t == nil {
					t = &track{}
					tr[i] = t
				}
				samePos := absInt(t.x-r.X) <= adaptiveSkipTol && absInt(t.y-r.Y) <= adaptiveSkipTol
				if t.streak > 0 && samePos {
					t.streak++
				} else {
					t.x, t.y, t.streak = r.X, r.Y, 1
				}
				if t.streak >= btns[i].SkipStreak {
					core.LogWithTimef("[Plugin] 自适应：【%s】连续 %d 轮同位置(%d,%d)命中 → 判定点不掉，本次自适应后续所有轮都跳过",
						btns[i].Name, t.streak, r.X, r.Y)
					skipped[i] = true
					continue
				}
			}

			// 命中即停：不点击本按钮，立即返回 true（不再点其他按钮、不等超时）
			if btns[i].Stop {
				core.LogWithTimef("[Plugin] 自适应命中【%s】(%d,%d) → 命中即停（不点击）", btns[i].Name, r.X, r.Y)
				if onHit != nil {
					onHit(btns[i].Name)
				}
				return true
			}

			// 点击坐标 = 命中坐标 + 点击偏移（默认 [0,0] 即不偏移）
			clickX := r.X + btns[i].ClickOffset[0]
			clickY := r.Y + btns[i].ClickOffset[1]
			core.LogWithTimef("[Plugin] 点击【%s】(%d,%d)", btns[i].Name, clickX, clickY)
			// 点击自带的"点击后延时"不计入超时预算：测出本次点击耗时，补回 deadline
			t0 := time.Now()
			core.RandomClickInAreaWithDelay(clickX, clickY, clickX+3, clickY+3)
			deadline = deadline.Add(time.Since(t0))
			if onHit != nil {
				onHit(btns[i].Name)
			}
			本轮命中 = true

			if btns[i].Done {
				core.LogWithTimef("[Plugin] 自适应命中结束按钮【%s】", btns[i].Name)
				命中结束 = true
			}
		}

		// 本轮一个都没命中 → sleep 后重进下一轮（到点由外层 for 条件结束）
		if !本轮命中 {
			core.RandomSleep(intervalMs/2, intervalMs)
		}
	}
	return 命中结束
}

// sleepWithStop 可被全局停止标志打断的睡眠：
// 每 200ms 检查一次 IsStopped()，脚本已停则立即返回，避免"已停还要干等满时长"。
func sleepWithStop(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if IsStopped() {
			return
		}
		remaining := time.Until(deadline)
		step := 200 * time.Millisecond
		if remaining < step {
			step = remaining
		}
		if step <= 0 {
			return
		}
		time.Sleep(step)
	}
}
