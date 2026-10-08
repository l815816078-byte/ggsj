package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&打怪中杂事{})
}

// 打怪中杂事 插件（供"无法副本"任务【StartUp】巡检链调用）
//
// 需求: 用户希望"打副本的时候顺手把每日杂事做掉, 不耽误"——
// 杂事不放在任务开头(副本前), 而是副本战斗中顺带执行, 仿 任务进行中.go(打怪进行中时 RunTask 同款)。
//
// 每轮被 StartUp 巡检调用一次:
//  1. 先确认是否真的在副本战斗中: 识别'副本时间'(recog 预设, 区域546,57,717,157)。
//     未命中 = 没在打本(加载中/已打完/副本列表界面) → 返回 no, 交还 StartUp 继续巡检下一项【检测副本时间】;
//  2. 命中 = 在副本战斗 → 依次 RunTask 11 个杂事(带独立冷却, 同 副本前杂事/任务进行中 的清单):
//     扩展背包/领取邮件/福利领取/季票/背包物品使用/图鉴登记/领取召唤/武功/灵物/阴阳/锻炼/秘籍/材料制作/自动分解;
//     RunTask 内部有冷却/运行态拦截, 冷却中的直接返回 false(不切界面不执行), 只有真执行的
//     任务之间才 RandomSleep 缓冲(全被拦截时零等待零开销);
//  3. 无论本轮有没有真跑杂事, 一律返回 no(用户明确要求: 杂事做完继续打怪, 不重导航):
//     RunTask 杂事收尾会回到副本战斗界面, 杂事不打断当前副本 → 巡检继续【检测副本时间】
//     照常等60秒, 本局副本照常打到结束; 本局打完后的收尾/下一个副本由 StartUp 正常流转,
//     不做"主菜单→地域→无法"重导航(旧版做完杂事就返回'主菜单'放弃本局, 已废弃)。
//  4. 整批冷却: 单任务级冷却(RunTask 第二参)只管"该任务上次执行后 N 秒", 挡不住"每局副本
//     都从头试一遍清单"; 这里给整批再加一道 zsFullCooldown——真跑完一轮(ran=true)后 N 秒内,
//     StartUp 再巡检进来直接返回 no(连清单都不试), 避免打完一局换下一局又整批重跑一遍。
//
// 依赖: recog 预设'副本时间'(recogs.json 已有)。
type 打怪中杂事 struct{}

func (p *打怪中杂事) Name() string { return "打怪中杂事" }

// 整批冷却状态(包级, 进程内跨 StartUp 巡检轮/跨副本局持久)
var (
	zsLastFull     time.Time            // 上次整批真执行完的时间(零值 = 从未跑过)
	zsFullCooldown = 2000 * time.Second // 整批跑完后冷却时长: 3000s=50分钟(每日杂事做一次就够, 覆盖几十局副本)
)

func (p *打怪中杂事) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	// 1. 只有真的在副本战斗(识别到'副本时间')才做杂事; 没在打本 → no 交还 StartUp 继续巡检
	k := core.Exec("副本时间")
	if !k.Found {
		core.LogWithTimef("[Plugin] %s 未检测到'副本时间'(不在副本战斗) → 返回 no", ctx.StepName)
		return "no", nil
	}

	// 1.5 整批冷却: 上次整批跑完后 zsFullCooldown 内不再从头试清单(防"下次再进来又跑一遍")
	if !zsLastFull.IsZero() && time.Since(zsLastFull) < zsFullCooldown {
		core.LogWithTimef("[Plugin] %s 整批杂事冷却中(上次 %s 跑完, 距现在 %s < %s) → 返回 no", ctx.StepName,
			zsLastFull.Format("15:04:05"), time.Since(zsLastFull).Round(time.Second), zsFullCooldown)
		return "no", nil
	}
	core.LogWithTimef("[Plugin] %s 检测到'副本时间'(副本战斗中) → 顺带执行每日杂事", ctx.StepName)

	// 穿戴类/召唤类任务冷却按等级分档: ≤30 → 1000 秒; >30 → 3000 秒
	// (无法副本门槛30级起, 实际基本恒走 3000 档)
	cooldown := 1000
	if lv, ok := core.GetLevel(core.DefaultLevelCacheTTL); ok && *lv > 30 {
		cooldown = 3000
	}

	// 2. 依次跑杂事; 只收集"真有任务执行"(冷却中/运行中会被 RunTask 拦截返回 false);
	//    真执行的任务之间才 RandomSleep 缓冲界面, 全被拦截时不做无谓等待
	ran := false
	exec := func(name string, cd int) {
		if RunTask(name, cd) {
			ran = true
			core.RandomSleep(1000, 3000)
		}
	}
	// 扩展背包排第一个: 背包满了后面材料制作/分解/上架都做不了
	exec("扩展背包", bagExpandCooldownSec)
	exec("领取邮件", cooldown)
	exec("福利领取", 1000)
	exec("季票", cooldown)
	exec("背包物品使用", 1000)
	exec("图鉴登记", 1000)
	exec("领取召唤", cooldown)
	exec("武功", cooldown)
	exec("灵物", cooldown)
	exec("阴阳", cooldown)
	exec("锻炼", cooldown)
	exec("秘籍", cooldown)
	exec("材料制作", cooldown)
	exec("自动分解", cooldown)

	// 3. 无论有没有真跑杂事都返回 no, 不重导航:
	//    全被冷却拦截 → 没切界面, 本局副本照常打;
	//    有真执行 → 杂事收尾回副本战斗界面(同 任务进行中 机制), 本局副本照常打到结束,
	//    由 StartUp 巡检【检测副本时间】继续等/收尾, 不做"主菜单"重导航(旧版行为, 已废弃)
	if !ran {
		core.LogWithTimef("[Plugin] %s 本轮杂事全部冷却中 → 未切界面 → 返回 no, 继续巡检", ctx.StepName)
		return "no", nil
	}
	// 真跑完一轮 → 记下整批冷却起点, zsFullCooldown 内不再整批重试
	zsLastFull = time.Now()
	core.LogWithTimef("[Plugin] %s 本轮杂事执行完成 → 返回 no, 继续巡检打怪(本局副本照常打完, 不重导航; 整批冷却 %s)", ctx.StepName, zsFullCooldown)
	return "no", nil
}
