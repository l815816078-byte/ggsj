package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&TaskInProgressPlugin{})
}

// TaskInProgressPlugin 检测打怪进行中
// 流程: 循环检测(间隔10秒):
//
//	在固定区域 [1186,155,1224,178] 检测"打怪进行中"(字库找字)
//	命中(真的在打怪) → 超时秒数续上(deadline顺延20秒), 继续检测, 不让固定超时打断打怪;
//	未命中则维持原超时行为。
//
// 不管怎么续时, 总运行时长固定上限5分钟(hardDeadline) → 到点强制超时结束。
//
// 结束判定:
//   - 期间检测到过"打怪进行中" → 点击其坐标 → 显式跳转"主线任务"步骤
//   - 一直没检测到 → 返回 no(交还 StartUp 继续巡检)
//
// 识别预设: "打怪进行中"(字库 ziku.txt 已有该词)
type TaskInProgressPlugin struct{}

func (p *TaskInProgressPlugin) Name() string { return "检测任务进行中" }

func (p *TaskInProgressPlugin) Exec(ctx *Context) (string, error) {
	deadline := time.Now().Add(10 * time.Second)
	// 固定总上限: 不管中间怎么续时, 1分钟一到必须超时结束
	hardDeadline := time.Now().Add(1 * time.Minute)
	var hit *core.RecogResult

	for {
		if IsStopped() {
			return "no", nil
		}
		// 固定5分钟总上限优先: 到了直接超时, 不做本轮识别
		if time.Now().After(hardDeadline) {
			core.LogWithTimef("[Plugin] %s 达到固定1分钟总上限 → 强制超时", ctx.StepName)
			break
		}

		// 固定区域 [1186,155,1224,178] 检测"打怪进行中"
		k := core.Exec("打怪进行中")
		if k.Found {
			hit = k
			deadline = time.Now().Add(10 * time.Second)
			core.LogWithTimef("[Plugin] %s 检测到'打怪进行中' (%d,%d) → 超时续上, 继续检测", ctx.StepName, k.X, k.Y)
			// 打怪期间顺带做一次背包物品使用（配置化任务 assets/tasks/背包物品使用.json）
			// 第二参数 = 执行完后的冷却秒数，避免每轮打怪检测都跑一遍；
			// 内部还会检查任务是否正在运行，自动调度和这里不会重复执行
			// 穿戴类/召唤类任务冷却按等级分档（高等级收益低，拉长冷却）：
			// ≤30 → 1000 秒；>30 → 10000 秒；>50 → 30000 秒
			wugongCooldown := 1000
			if lv, ok := core.GetLevel(core.DefaultLevelCacheTTL); ok && *lv > 30 {
				wugongCooldown = 10000
				if *lv > 50 {
					wugongCooldown = 30000
				}
			}
			// 扩展背包排第一个: 背包满了后面材料制作/分解/上架都做不了
			RunTask("扩展背包", bagExpandCooldownSec)
			core.RandomSleep(1000, 3000)
			RunTask("成就领取", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("领取邮件", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("福利领取", 1000)
			core.RandomSleep(1000, 3000)
			RunTask("季票", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("背包物品使用", 1000)
			core.RandomSleep(1000, 3000)
			RunTask("图鉴登记", 1000)
			core.RandomSleep(1000, 3000)
			RunTask("领取召唤", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("武功", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("灵物", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("阴阳", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("锻炼", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("秘籍", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("材料制作", wugongCooldown)
			core.RandomSleep(1000, 3000)
			RunTask("自动分解", wugongCooldown)

			} else {
			core.LogWithTimef("[Plugin] %s 未检测到'打怪进行中'", ctx.StepName)
		}

		if time.Now().After(deadline) {
			break
		}
		// 间隔约10秒再检测
		core.RandomSleep(10000, 10000)
	}

	if hit == nil {
		core.LogWithTimef("[Plugin] %s 超时且期间未检测到'打怪进行中' → 返回 no", ctx.StepName)
		return "no", nil
	}
	core.SetStatusColor("#00CCFF", "打怪进行中 → 跳转主线任务")
	core.LogWithTimef("[Plugin] %s → 点击'打怪进行中' (%d,%d) → 跳转主线任务", ctx.StepName, hit.X, hit.Y)
	core.RandomClickInAreaWithDelay(hit.X, hit.Y, hit.X+5, hit.Y+5)
	return "主线任务", nil
}
