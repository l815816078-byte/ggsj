package core

import (
	"sync"
	"time"
)

// ---------- 任务强制冷却 ----------
// 插件在特殊分支（如悬赏任务"一个任务都没接到"）可调用 ExtendCooldown，
// 把某任务在 until 之前都置为不可调度，实现"这次执行后冷却更久"。
// 调度器侧 task.RuntimeTask.CanRun / BlockReason 会检查强制冷却。

var (
	cooldownMu      sync.Mutex
	cooldownExtends = map[string]time.Time{} // taskName → 冷却截止时间
	cooldownReasons = map[string]string{}    // taskName → 冷却原因（仅供日志显示，不参与判定）
)

// ExtendCooldown 强制延长任务冷却：任务名匹配的任务在 until 之前不可调度。
// until 早于当前时间时视为取消强制冷却。
func ExtendCooldown(taskName string, until time.Time) {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	if until.After(time.Now()) {
		cooldownExtends[taskName] = until
	} else {
		delete(cooldownExtends, taskName)
		delete(cooldownReasons, taskName)
	}
}

// ExtendCooldownWithReason 同 ExtendCooldown，额外记一条原因（仅用于日志显示）。
// 强制冷却的来源不止一处（插件主动压 / 调度器手动执行的冷却 / 战力门槛不足），
// 只显示"强制冷却中"分不清是谁压的，带上原因便于排查。
func ExtendCooldownWithReason(taskName string, until time.Time, reason string) {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	if until.After(time.Now()) {
		cooldownExtends[taskName] = until
		cooldownReasons[taskName] = reason
	} else {
		delete(cooldownExtends, taskName)
		delete(cooldownReasons, taskName)
	}
}

// CooldownBlocked 查询任务是否处于强制延长的冷却期内（只读，供调度器 CanRun/BlockReason 用）
func CooldownBlocked(taskName string, now time.Time) bool {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	until, ok := cooldownExtends[taskName]
	return ok && now.Before(until)
}

// CooldownUntil 查询强制冷却的截止时间与原因（只读，供日志显示剩余时间）。
// 返回 ok=true 表示当前处于强制冷却期内；reason 为空表示压冷却时没带原因。
func CooldownUntil(taskName string) (until time.Time, reason string, ok bool) {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	t, exists := cooldownExtends[taskName]
	if !exists || !time.Now().Before(t) {
		return time.Time{}, "", false
	}
	return t, cooldownReasons[taskName], true
}
