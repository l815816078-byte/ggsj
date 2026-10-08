package plugin

import (
	"app/core"
	"sync"
	"time"
)

func init() {
	Register(&掉线{})
}

// 掉线重启节流 两次重启之间的最小间隔。
// 弹窗在屏幕上会一直被弹窗扫描扫到（重启杀掉游戏前它不会消失），
// 不节流会连续触发多次全量重启（多个重启流程并发跑）。
const 掉线重启节流 = 60 * time.Second

var (
	掉线mu        sync.Mutex
	上次掉线重启At time.Time
)

// 掉线 全局弹窗：字库识别到"掉线"(区域521,196,748,420) → 重启 kit(代理) + 重启游戏。
//
// 处理动作是"全量重启"(kill 代理+游戏 → 启动代理点开关 → 启动游戏 → 登录)，
// 由 bot 注入的实现异步执行（期间状态机进 Recovering，调度器不再取新任务），不阻塞当前流水线。
// 命中 → 触发一次重启并返回 "ok"；节流期内/没识别到 → 返回 "no"，交给后面的弹窗/流程继续。
type 掉线 struct{}

func (p *掉线) Name() string { return "掉线" }

func (p *掉线) Exec(ctx *Context) (string, error) {
	r := core.Exec("掉线")
	if !r.Found {
		return "no", nil
	}

	掉线mu.Lock()
	if time.Since(上次掉线重启At) < 掉线重启节流 {
		掉线mu.Unlock()
		return "no", nil // 刚触发过重启，别重复触发
	}
	上次掉线重启At = time.Now()
	掉线mu.Unlock()

	core.LogWithTimef("[掉线] 检测到'掉线' (%d,%d) sim=%.2f → 触发全量重启(重启kit+游戏)",
		r.X, r.Y, r.Sim)
	TriggerFullRestart("掉线")
	return "ok", nil
}
