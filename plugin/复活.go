package plugin

import (
	"sync/atomic"
	"time"

	"app/core"
)

func init() {
	Register(&复活{})
}

// 复活次数达到该值后，重置"野外挂机"的冷却，让调度器下一轮就能重新挂机。
const 复活重置阈值 = 1

// 全局复活次数（跨任务累计；每满 复活重置阈值 次后清零，重新累计）
var 复活次数 atomic.Int32

// 复活 插件：识别到"复活"弹窗 → 点击复活 + 累计一次复活次数；
// 累计满 复活重置阈值(3) 次后，清除"野外挂机"任务的冷却（不立即执行，交给调度器下一轮），
// 并把复活计数清零重新累计。
//
// 命中返回 "ok"，没识别到返回 "no"。
type 复活 struct{}

func (p *复活) Name() string { return "复活" }

func (p *复活) Exec(ctx *Context) (string, error) {
	if !ClickRecog("复活") {
		return "no", nil
	}

	n := 复活次数.Add(1)
	core.LogWithTimef("[复活] 点击复活，累计复活次数 %d/%d", n, 复活重置阈值)

	if int(n) >= 复活重置阈值 {
		core.RandomSleep(3000, 3000)
		// 死前 5 分钟内在打 BOSS → 给"上次打 BOSS 的那个任务"压 3000 秒冷却
		if time.Since(上次打BOSS时间) < 5*time.Minute {
			core.ExtendCooldown(上次打BOSS任务名, time.Now().Add(3000*time.Second))
			core.LogWithTimef("[复活] 上次打BOSS任务=【%s】(%.0f秒前) → 【%s】冷却 3000 秒",
				上次打BOSS任务名, time.Since(上次打BOSS时间).Seconds(), 上次打BOSS任务名)
			复活次数.Store(0) // 压了冷却才清零计数，重新累计
		} else {
			core.LogWithTimef("[复活] 上次打BOSS任务=【%s】(%.0f秒前) → 超过5分钟，不压冷却",
				上次打BOSS任务名, time.Since(上次打BOSS时间).Seconds())
		}
	}
	return "ok", nil
}
