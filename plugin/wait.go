package plugin

import (
	"time"

	"app/core"
)

// WaitFind 循环检测单个识别预设：先等待 intervalSec 秒再检测，
// 直到找到返回结果，或超过 timeoutSec 秒返回 nil。
func WaitFind(timeoutSec, intervalSec int, recog string) *core.RecogResult {
	_, r := WaitFindAny(timeoutSec, intervalSec, recog)
	return r
}

// WaitFindAny 循环检测多个识别预设：任一命中即返回命中名与结果，
// 先等待 intervalSec 秒再检测，直到超过 timeoutSec 秒返回 ("", nil)。
func WaitFindAny(timeoutSec, intervalSec int, recogs ...string) (string, *core.RecogResult) {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for {
		if IsStopped() {
			return "", nil
		}
		core.RandomSleep((intervalSec-1)*1000, (intervalSec+1)*1000) // 卡住写在循环一开始
		for _, recog := range recogs {
			r := core.Exec(recog)
			if r.Found {
				core.LogWithTimef("[Wait] 检测到 %s → (%d,%d)", recog, r.X, r.Y)
				return recog, r
			}
		}
		if time.Now().After(deadline) {
			return "", nil
		}
	}
}

// LoopUntil 带超时的通用动作循环：
// 在 timeoutSec 秒内反复执行 fn，直到 fn 返回 (val, true) 视为完成，返回 (val, true)；
// fn 返回 (_, false) 则继续下一轮；每轮结束后额外等待 intervalSec 秒（<=0 不等待）。
// 超过 timeoutSec 或 IsStopped() 时返回 (零值, false)。
// timeoutSec <= 0 表示不设超时（仅靠 IsStopped() 退出）。
// 适合"每轮有业务动作"的循环（对话推进/越障等），把 deadline+停止检查统一收拢。
func LoopUntil[T any](timeoutSec, intervalSec int, fn func() (T, bool)) (T, bool) {
	var zero T
	var deadline time.Time
	if timeoutSec > 0 {
		deadline = time.Now().Add(time.Duration(timeoutSec) * time.Second)
	}
	for {
		if IsStopped() {
			return zero, false
		}
		val, done := fn()
		if done {
			return val, true
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return zero, false
		}
		if intervalSec > 0 {
			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	}
}
