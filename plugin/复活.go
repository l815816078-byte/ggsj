package plugin

import (
	"sync/atomic"
	"time"

	"app/core"
)

func init() {
	Register(&复活{})
}

// 复活次数达到该值后做一次"死在 BOSS 手上"的收尾（压冷却 + 结束当前任务），计数清零重新累计。
const 复活重置阈值 = 1

// 全局复活次数（跨任务累计；每满 复活重置阈值 次后清零，重新累计）
var 复活次数 atomic.Int32

// 复活 插件：识别到"复活"弹窗 → 点击复活 + 累计一次复活次数；
// 累计满 复活重置阈值 次后：若死前 5 分钟内在打 BOSS，就给"上次打 BOSS 的那个任务"压 3000 秒冷却，
// 同时置当前任务的取消令牌让它立即收工（死了再死磕只会空转），并把复活计数清零重新累计。
//
// 命中返回 "ok"，没识别到返回 "no"。
// 注意：弹窗插件的返回值只区分"命中/未命中"，不参与跳转 —— 引擎命中后一律"清掉→重试当前节点"，
// 所以"结束任务"是靠取消令牌实现的，不是靠返回 #done。
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
			// 死了 → 当前任务没必要再死磕（继续跑只会空转），立刻收工：
			// 置当前任务的取消令牌，引擎在下一个检查点退出，调度器记为"失败"并按冷却走。
			// 不走 #done 是故意的：死亡属于异常情况，判失败才能触发 onTaskError。
			if p := CurrentCancel(); p != nil {
				p.Store(true)
				core.LogWithTimef("[复活] 当前任务【%s】立即结束", ctx.TaskName)
			}
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
