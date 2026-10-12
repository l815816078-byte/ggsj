package plugin

import (
	"math/rand"
	"time"

	"app/core"
)

func init() {
	Register(&挂机中{})
}

// 挂机中 插件：检测到"自动攻击开启中"即认为正在挂机中。
// 挂机中顺带跑子任务（按顺序，冷却区间随机，避免每轮重复执行）；
// 命中返回 "ok"（挂机中），没检测到返回 "no"。
type 挂机中 struct{}

func (p *挂机中) Name() string { return "挂机中" }

func (p *挂机中) Exec(ctx *Context) (string, error) {
	// 先检测【复活】：命中就点它（复活），本轮不跑子任务，交回路由器
	if ClickRecog("复活") {
		core.LogWithTimef("[挂机中] 检测到【复活】→ 已点击")
		return "no", nil
	}

	// 没开启自动攻击 → 不在挂机：清掉打怪计时（去自动攻击的时间不算，回来重新从 0 计）
	if !core.Exec("自动攻击开启中").Found {
		delete(ctx.Global, "上次打怪时间")
		return "no", nil
	}

	now := time.Now()

	// 只要检测到【怪物】就刷新打怪时间
	if core.Exec("怪物").Found {
		ctx.Global["上次打怪时间"] = now
	}
	// 首次进入初始化（本次连续挂机从 0 开始计）
	上次打怪, ok := ctx.Global["上次打怪时间"].(time.Time)
	if !ok {
		上次打怪 = now
		ctx.Global["上次打怪时间"] = now
	}

	// 超过 100 秒没打怪 → 直接结束整个任务
	if now.Sub(上次打怪) > 100*time.Second {
		core.LogWithTimef("[挂机中] %d 秒没打怪 → 结束整个任务", int(now.Sub(上次打怪).Seconds()))
		return "#done", nil
	}
	core.LogWithTimef("[挂机中] 检测到自动攻击开启中 → 正在挂机")

	// sleep 期间不算"没打怪"时间：睡完把耗时补回打怪时间
	sleep开始 := time.Now()
	core.RandomSleep(10000, 100000) // 挂机中 10-100 秒随机延迟 这样每个 账号操作任务的间隔会不一样
	ctx.Global["上次打怪时间"] = ctx.Global["上次打怪时间"].(time.Time).Add(time.Since(sleep开始))

	// 只有【野外挂机】任务才跑子任务（落花战场不跑）
	if ctx.TaskName == "野外挂机" {
		// 子任务期间不算"没打怪"时间：跑子任务前记时间，跑完把耗时补回到上次打怪时间上
		子任务开始 := time.Now()
		// 子任务每轮随机顺序执行（名称 + 冷却区间，秒）；冷却取区间内随机值
		子任务 := []struct {
			名     string
			冷最小 int
			冷最大 int
		}{
			{"领取邮件", 10000, 20000},
			{"领取成就", 10000, 20000},
			{"福利领取", 10000, 20000},
			{"灵力修炼", 2500, 5000},
			{"气运显现", 10000, 20000},
			{"背包物品", 5000, 10000},
		}
		rand.Shuffle(len(子任务), func(i, j int) { 子任务[i], 子任务[j] = 子任务[j], 子任务[i] })
		执行过 := false
		for _, t := range 子任务 {
			if RunTask(t.名, t.冷最小+rand.Intn(t.冷最大-t.冷最小+1)) {
				执行过 = true
				core.RandomSleep(1000, 3000)
			}
		}
		ctx.Global["上次打怪时间"] = ctx.Global["上次打怪时间"].(time.Time).Add(time.Since(子任务开始))

		// 有子任务真的执行过 → 主动做一次任务轮转弹窗清扫
		if 执行过 {
			ClearPopups()
		}
	}

	return "ok", nil
}
