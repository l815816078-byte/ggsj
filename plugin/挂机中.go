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

	// 每日任务类型：不判断自动攻击开启中，只判断【任务进行中】【怪物】【省电_金币】三种
	if ctx.TaskName == "每日任务" {
		if 刷新并超时(ctx, "每日任务", core.Exec("任务进行中").Found || core.Exec("怪物").Found || core.Exec("省电_金币").Found) {
			return "#done", nil
		}
		core.LogWithTimef("[挂机中] 每日任务进行中 → 正在挂机")
		睡眠不计时(ctx)
		跑子任务(ctx)
		return "ok", nil
	}

	// 没开启自动攻击 → 不在挂机：清掉打怪计时（去自动攻击的时间不算，回来重新从 0 计）
	if !core.Exec("自动攻击开启中").Found {
		delete(ctx.Global, "上次打怪时间")
		return "no", nil
	}

	// 检测到【怪物】→ 刷新打怪时间；超过 100 秒没打怪 → 结束整个任务
	if 刷新并超时(ctx, "", core.Exec("怪物").Found) {
		return "#done", nil
	}
	core.LogWithTimef("[挂机中] 检测到自动攻击开启中 → 正在挂机")

	睡眠不计时(ctx)

	// 只有【野外挂机】任务才跑子任务（落花战场不跑）
	if ctx.TaskName == "野外挂机" {
		跑子任务(ctx)
	}

	return "ok", nil
}

// 跑子任务 每轮随机顺序执行子任务（名称 + 冷却区间，秒，取区间随机值）；
// 子任务耗时不算"没打怪"时间（跑完补回上次打怪时间）；有执行过则主动清一次任务轮转弹窗。
func 跑子任务(ctx *Context) {
	// 子任务期间不算"没打怪"时间：跑子任务前记时间，跑完把耗时补回到上次打怪时间上
	子任务开始 := time.Now()
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
		{"画册", 10000, 20000},
		{"加护", 10000, 20000},
		{"成长", 10000, 20000},
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

// 睡眠不计时 挂机中 10-100 秒随机延迟，睡完把耗时补回"上次打怪时间"（休眠期间不算"没打怪"）
func 睡眠不计时(ctx *Context) {
	sleep开始 := time.Now()
	core.RandomSleep(10000, 100000)
	ctx.Global["上次打怪时间"] = ctx.Global["上次打怪时间"].(time.Time).Add(time.Since(sleep开始))
}

// 刷新并超时 命中则刷新"上次打怪时间"，首次进入初始化；超过 100 秒无动静返回 true（应结束任务）。
// 前缀 用于日志区分（每日任务/空）。
func 刷新并超时(ctx *Context, 前缀 string, 命中 bool) bool {
	now := time.Now()
	if 命中 {
		ctx.Global["上次打怪时间"] = now
	}
	上次打怪, ok := ctx.Global["上次打怪时间"].(time.Time)
	if !ok {
		上次打怪 = now
		ctx.Global["上次打怪时间"] = now
	}
	if now.Sub(上次打怪) > 100*time.Second {
		if 前缀 != "" {
			core.LogWithTimef("[挂机中] %s %d 秒无动静 → 结束整个任务", 前缀, int(now.Sub(上次打怪).Seconds()))
		} else {
			core.LogWithTimef("[挂机中] %d 秒没打怪 → 结束整个任务", int(now.Sub(上次打怪).Seconds()))
		}
		return true
	}
	return false
}
