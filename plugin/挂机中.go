package plugin

import (
	"math/rand"
	"time"

	"app/core"
)

// 子任务下次可跑 子任务冷却表（进程内）：跑完一个子任务按随机区间记下次可跑时间，没到就跳过。
// 子任务不再走调度器(不写 RuntimeTask 的冷却/状态)，冷却只记在这张表里。
var 子任务下次可跑 = map[string]time.Time{}

func init() {
	Register(&挂机中{})
}

// 挂机中 插件：判断"是不是还在挂机" → "ok"（顺带跑子任务）/ "no"（不在挂机）/ "#done"（长时间无动静，收工）。
// 每日任务：不判断【自动攻击开启中】，【任务进行中】命中、或 先命中【省电模式】再命中【/】即算挂机中。
// 其他任务：【自动攻击开启中】是前提（没开就直接不在挂机）；在此基础上【怪物】命中 或【省电_金币】文字有变化 = 有动静，
// 两者共用同一计时（任一有动静都刷新），100 秒都没动静 → 不算挂机。
type 挂机中 struct{}

func (p *挂机中) Name() string { return "挂机中" }

func (p *挂机中) Exec(ctx *Context) (string, error) {
	// 先做一次任务轮转弹窗清扫，再检测【复活】（复活可能被弹窗挡着识别不到）
	ClearPopups()

	// 先检测【复活】：命中就点它（复活），本轮不跑子任务，交回路由器
	if ClickRecog("复活") {
		core.LogWithTimef("[挂机中] 检测到【复活】→ 已点击")
		return "no", nil
	}

	// 每日任务类型：不判断【自动攻击开启中】。
	// 挂机中 =【任务进行中】命中；或 先检测到【省电模式】、再检测到【/】
	if ctx.TaskName == "每日任务" {
		挂机中 := core.Exec("任务进行中").Found
		core.LogWithTimef("[挂机中] 每日任务：【任务进行中】命中=%t", 挂机中)
		if !挂机中 && core.Exec("省电模式").Found {
			core.LogWithTimef("[挂机中] 每日任务：检测到【省电模式】→ 检测【/】")
			挂机中 = core.Exec("/").Found
			core.LogWithTimef("[挂机中] 每日任务：【/】命中=%t", 挂机中)
		}
		if 刷新并超时(ctx, "每日任务", 挂机中) {
			return "#done", nil
		}
		睡眠不计时(ctx)
		跑子任务(ctx)
		return "ok", nil
	}

	// 没开启自动攻击 → 不在挂机：清掉打怪计时和金币文本（去自动攻击的时间不算，回来重新从 0 计，
	// 金币也重记，否则下次回来第一次读到的金币跟离开前一样 → 判不出"有变化"）
	if !core.Exec("自动攻击开启中").Found {
		core.LogWithTimef("[挂机中] 未检测到【自动攻击开启中】→ 不在挂机；清掉打怪计时与金币文本，回来从 0 重新计 → no")
		delete(ctx.Global, "上次打怪时间")
		delete(ctx.Global, "上次金币文本")
		return "no", nil
	}
	// 有动静 =【怪物】命中 或【省电_金币】读到的文字与上次不同；两者共用同一个计时（任一有动静都刷新），
	// 100 秒都没动静（没打怪、金币也不变）→ 不算挂机，结束整个任务
	有动静 := core.Exec("怪物").Found
	core.LogWithTimef("[挂机中] 【怪物】命中=%t", 有动静)
	if r := core.Exec("省电_金币"); r.Found && r.Word != "" {
		core.LogWithTimef("[挂机中] 【省电_金币】读到(%s)", r.Word)
		if 上次金币, _ := ctx.Global["上次金币文本"].(string); r.Word != 上次金币 {
			ctx.Global["上次金币文本"] = r.Word
			有动静 = true
			core.LogWithTimef("[挂机中] 【省电_金币】有变化(%s → %s) → 刷新计时", 上次金币, r.Word)
		}
	}
	core.LogWithTimef("[挂机中] 本轮有动静=%t", 有动静)
	if 刷新并超时(ctx, "", 有动静) {
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
//
// 子任务只是"跑一遍该任务的流水线"（RunSubTask）：不参与调度、不写任务状态/冷却，
// 就在本插件所在的 goroutine 里同步跑完；外层任务一结束（含超时），
// 取消令牌置位后这里和子任务流水线内部都会在下一个检查点一起退出。
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
		// 脚本停止 / 外层任务被取消 → 立刻停，不再往后跑子任务
		if IsStopped() || IsTaskCancelled() {
			break
		}
		if time.Now().Before(子任务下次可跑[t.名]) {
			continue // 冷却未到 → 跳过
		}
		if RunSubTask(t.名) {
			执行过 = true
			子任务下次可跑[t.名] = time.Now().Add(time.Duration(t.冷最小+rand.Intn(t.冷最大-t.冷最小+1)) * time.Second)
			core.RandomSleep(3000, 20000)
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
