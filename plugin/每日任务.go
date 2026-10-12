package plugin

import (
	"time"

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

			// 找到【符合战力】或（可接取时）【无需战力】→ 点它 → 点蓝色按钮
			if ClickRecog("符合战力") || (是否接取无需战力任务 && ClickRecog("无需战力")) {

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

	// 查找点击【每日】
	ClickRecog("每日")
	core.LogWithTimef("[每日任务] 已点击【每日】")

	// 多点找色【绿色战力】取多个（区域 552,171,834,622，寻找个数5 在预设里）→ 逐个点击
	pts := core.ExecAll("绿色战力", 552, 171, 834, 622)
	core.LogWithTimef("[每日任务] 【绿色战力】区域 552,171,834,622 命中 %d 个", len(pts))
	点过蓝色按钮 := false
	for i, pt := range pts {
		if IsStopped() {
			return "no", nil
		}
		core.LogWithTimef("[每日任务] 第%d个战力 命中坐标 %d,%d", i+1, pt.X, pt.Y)
		// 点击该战力
		core.RandomClickInAreaWithDelay(pt.X, pt.Y, pt.X+3, pt.Y+3)
		// 点完再检测一下【/】(区域 857,142,1231,587)，只判断不做操作
		if !core.Exec("/", 857, 142, 1231, 587).Found {
			core.LogWithTimef("[每日任务] 第%d个战力 未检测到【/】(区域 857,142,1231,587) → 继续下一个", i+1)
			continue
		}
		core.LogWithTimef("[每日任务] 第%d个战力 检测到【/】(区域 857,142,1231,587)", i+1)
		// 检测到【自动进行】(区域 773,657,827,681) → 点击它
		ClickRecog("自动进行")
		// 点【蓝色按钮】(区域 955,631,1102,702)
		if ClickRecog("蓝色按钮", 955, 631, 1102, 702) {
			core.LogWithTimef("[每日任务] 第%d个战力 已点击【蓝色按钮】(区域 955,631,1102,702)", i+1)
			点过蓝色按钮 = true
		} else {
			core.LogWithTimef("[每日任务] 第%d个战力 未找到【蓝色按钮】(区域 955,631,1102,702) → 继续下一个", i+1)
		}
	}
	// 完事 → 退出（循环6秒/间隔2秒）
	CheckLoopClick("退出", 6*time.Second, 2*time.Second)
	// 一次都没点过【蓝色按钮】→ 直接完成整个任务
	if !点过蓝色按钮 {
		core.LogWithTimef("[每日任务] 一次都没点击【蓝色按钮】→ 直接完成整个任务")
		return "#done", nil
	}
	/*	// 接取前：先点【组队】【任务入口】(各循环6秒/间隔2秒)
		点组队与任务入口()

		// 循环接取：检测【每日任务】取多个结果，逐个点击；没命中从下往上滑动（最多滑 最多滑动次数 次）
		进入中 := false
		for i := 1; i <= 最多滑动次数; i++ {
			if IsStopped() {
				return "no", nil
			}
			// 取多个【每日任务】结果（区域 929,99,981,371，坐标偏移在预设里）
			pts := core.ExecAll("每日任务", 929, 99, 981, 371)
			if len(pts) > 0 {
				core.LogWithTimef("[每日任务] 第%d次找到%d个【每日任务】", i, len(pts))
				// 一次打印本轮全部命中点
				明细 := ""
				for n, pt := range pts {
					明细 += fmt.Sprintf("[#%d 命中点(%d,%d) 偏移后(%d,%d) 右下(%d,%d)] ",
						n+1, pt.X-40, pt.Y+151, pt.X, pt.Y, pt.X2, pt.Y2)
				}
				core.LogWithTimef("[每日任务] %s", 明细)
				for _, pt := range pts {
					if IsStopped() {
						return "no", nil
					}
					// 点击该任务
					core.RandomClickInAreaWithDelay(pt.X, pt.Y, pt.X+3, pt.Y+3)
					// 点了之后点击【退出】→ 说明已做完/已在进行中，不用再检测进行中，继续点下一个
					if CheckLoopClick("退出", 3*time.Second, 1*time.Second) {
						core.LogWithTimef("[每日任务] 点击(%d,%d)后点击【退出】→ 继续点下一个", pt.X, pt.Y)
						continue
					}
					// 点了之后循环检测【每日任务进行中】(3秒/间隔1秒)
					if CheckLoop("每日任务进行中", 3*time.Second, 1*time.Second) {
						core.LogWithTimef("[每日任务] 点击(%d,%d)后进入【每日任务进行中】→ 跳出循环", pt.X, pt.Y)
						进入中 = true
						break
					}
					// 没检测到 → 没进去，继续点下一个
					core.LogWithTimef("[每日任务] 点击(%d,%d)后未进入【每日任务进行中】→ 继续", pt.X, pt.Y)
				}
				if 进入中 {
					break
				}
			}
			// 未找到 → 从下往上滑动（区域 999,146,1034,298，速度1500，滑后延时2000）
			cx := (999 + 1034) / 2
			core.LogWithTimef("[每日任务] 第%d次未找到【每日任务】→ 滑动 (%d,%d)→(%d,%d)", i, cx, 298, cx, 146)
			core.RandomSwipe(cx, 298, cx, 146, 1500, 2000)
		}
	*/
	core.LogWithTimef("[每日任务] 全部完成")
	return "ok", nil
}

// 点组队与任务入口 接取任务前，先点【组队】【任务入口】(各循环6秒/间隔2秒，找到就点)
func 点组队与任务入口() {
	CheckLoopClick("组队", 6*time.Second, 2*time.Second)
	CheckLoopClick("任务入口", 6*time.Second, 2*time.Second)
}
