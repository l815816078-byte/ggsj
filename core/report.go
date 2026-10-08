package core

import (
	"sync"
	"time"
)

// 中控上报挂钩：
//   - 识别类字段（等级/战力）在「识别成功且值有变化」时，触发一次上报
//   - 上报实现由 client/wsclient 注入（core 不能反向 import client，故用回调）
//
// 这样设计的原因（见方案）：等级/战力只在特定界面可读，跑任务时读不到；
// 挂在识别成功分支「变化才报」，零额外识别开销。

// reportHook 上报回调：由 client/wsclient 在启动时注入。
// 参数 field 为发生变化的字段名（如 "等级"/"战力"）。
var (
	reportHook   func(field string)
	reportHookMu sync.RWMutex
)

// SetReportHook 注入中控上报回调（由 client/wsclient 调用）
func SetReportHook(f func(field string)) {
	reportHookMu.Lock()
	reportHook = f
	reportHookMu.Unlock()
}

// notifyReport 通知一次字段变化（内部使用）
func notifyReport(field string) {
	reportHookMu.RLock()
	h := reportHook
	reportHookMu.RUnlock()
	if h != nil {
		h(field)
	}
}

// ── 停止状态（供上报当前 state 使用）──
// 与 plugin.IsStopped 相互独立：这里只表达「脚本是否已被要求停止」，
// 由 bot 在停止时置位。

var (
	stoppedFlag   bool
	stoppedFlagMu sync.RWMutex
)

// SetStopped 设置脚本停止标志（bot 停止时调用）
func SetStopped(v bool) {
	stoppedFlagMu.Lock()
	stoppedFlag = v
	stoppedFlagMu.Unlock()
}

// IsStopped 读取脚本停止标志
func IsStopped() bool {
	stoppedFlagMu.RLock()
	defer stoppedFlagMu.RUnlock()
	return stoppedFlag
}

// ── 缓存读音（不触发识别）──

// PeekCachedInt 读取某「取文」字段的缓存值并解析为 int。
// 只读缓存，不触发截图识别：用于上报时读取「上次识别到的值」。
// 返回 (值, 是否存在)。缓存里存的是 *RecogResult（与 GetCachedTextInt 同一 key）。
func PeekCachedInt(name string) (int, bool) {
	v, ok := CacheGet("recog:" + name)
	if !ok {
		return 0, false
	}
	r, ok := v.(*RecogResult)
	if !ok || r == nil {
		return 0, false
	}
	return parseLeadingInt(r.Word)
}

// ── 上报节流 ──
// 避免识别密集时每条都触发上报；同一字段最短 3 秒报一次。

var (
	lastReportAt   = map[string]time.Time{}
	lastReportAtMu sync.Mutex
)

// reportThrottled 判断某字段是否允许此刻上报（节流）
func reportThrottled(field string) bool {
	lastReportAtMu.Lock()
	defer lastReportAtMu.Unlock()
	if t, ok := lastReportAt[field]; ok && time.Since(t) < 3*time.Second {
		return true
	}
	lastReportAt[field] = time.Now()
	return false
}

// ReportField 供识别成功分支调用：字段变化时通知上报（带 3 秒节流）
func ReportField(field string) {
	if reportThrottled(field) {
		return
	}
	notifyReport(field)
}
