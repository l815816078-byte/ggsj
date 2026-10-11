package plugin

import (
	"app/core"
)

func init() {
	Register(&每日任务{})
}

// 是否接取无需战力任务 是否接取"无需战力"的任务（默认 true）
var 是否接取无需战力任务 = true

// 任务类型列表：依次切到每个 tab 处理（每日、周常）
var 任务类型列表 = []string{"每日", "周常"}

// 第一个循环最多滑动次数（滑满就结束这个循环）
const 最多滑动次数 = 5

// 每日任务 插件：进任务界面后，遍历【每日】【周常】两个 tab，
// 循环接取"符合战力"的任务；可选接取"无需战力"的任务；
// 找不到就滑动列表，最多滑 5 次；检测到【任务接取已满】就停。
type 每日任务 struct{}

func (p *每日任务) Name() string { return "每日任务" }

func (p *每日任务) Exec(ctx *Context) (string, error) {
	for _, 类型 := range 任务类型列表 {
		if IsStopped() {
			return "no", nil
		}
		core.LogWithTimef("[每日任务] 切到任务类型【%s】", 类型)
		ClickRecog(类型)
		core.RandomSleep(1500, 2000)
		// 最多滑动 最多滑动次数 次
		for i := 1; i <= 最多滑动次数; i++ {
			if IsStopped() {
				return "no", nil
			}

			// 任务接取已满 → 结束该 tab
			if core.Exec("任务接取已满").Found {
				core.LogWithTimef("[每日任务]【%s】任务接取已满 → 结束该类型", 类型)
				break
			}

			// 找到符合战力 → 点它 → 点蓝色按钮
			if ClickRecog("符合战力") {
				ClickRecog("蓝色按钮", 834, 626, 912, 705)
				continue
			}

			// 可选：接取无需战力任务
			if 是否接取无需战力任务 && ClickRecog("无需战力") {
				ClickRecog("蓝色按钮", 834, 626, 912, 705)
				continue
			}

			// 两个都找不到 → 滑动列表（447,296,518,528 从下往上），滑后延时2秒
			cx := (447 + 518) / 2
			core.LogWithTimef("[每日任务]【%s】第%d次未找到任务 → 滑动 (%d,%d)→(%d,%d)",
				类型, i, cx, 528, cx, 296)
			core.RandomSwipe(cx, 528, cx, 296, 1500, 2000)
		}
	}
	
	core.LogWithTimef("[每日任务] 全部完成")
	return "ok", nil
}
