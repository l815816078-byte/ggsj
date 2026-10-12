package plugin

import (
	"app/core"
)

func init() {
	Register(&物品使用{})
}

// 背包分类变量：需要遍历点击的分类（顺序即处理顺序）
var 背包分类 = []string{"消耗品" /*, "材料", "装备"*/}

// 背包格子：区域起点 (933,175)，格子 63x62，左右间隔 6（X 步长 69），上下间隔 7（Y 步长 69），
// 一行 4 格，共 7 行 = 28 格。
const (
	格子起点X = 933
	格子起点Y = 175
	格子宽    = 63
	格子高    = 62
	格子步长X = 69
	格子步长Y = 69
	格子列数  = 4
	格子行数  = 7
)

// 连续空格子上限：连续这么多次格子都点不出东西 → 结束当前分类的循环（一旦有物品就清零重数）
const 连续空格子上限 = 3

// 物品使用 插件
//
// 流程（对应任务里的 for 循环 + 内层循环检测）：
//
//	for 背包分类（装备/消耗品/材料） {
//	    点【背包分类on】打开该分类
//	    for 每个背包格子 {
//	        点格子中心
//	        循环检测：命中【打开】→ 点它 → 再点【关闭】关弹窗
//	                  命中【使用】→ 点它
//	    }
//	}
type 物品使用 struct{}

func (p *物品使用) Name() string { return "物品使用" }

func (p *物品使用) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	for _, 分类 := range 背包分类 {
		if IsStopped() {
			return "no", nil
		}

		// 点【背包分类on】切到该分类（分类 tab 的识别引用就是分类名）
		core.LogWithTimef("[物品使用] 切换分类: %s", 分类)
		ClickRecog(分类)

		// 遍历所有背包格子；连续 3 次点不出东西 → 结束当前分类
		连续空格子 := 0
		for row := 0; row < 格子行数; row++ {
			for col := 0; col < 格子列数; col++ {
				if IsStopped() {
					return "no", nil
				}

				// 格子中心
				x1 := 格子起点X + col*格子步长X
				y1 := 格子起点Y + row*格子步长Y
				cx := x1 + 格子宽/2
				cy := y1 + 格子高/2

				core.LogWithTimef("[物品使用] 点格子(行%d,列%d) 中心(%d,%d)", row+1, col+1, cx, cy)
				core.RandomClickInAreaWithDelay(cx, cy, cx, cy)

				// 点开后循环检测：打开 / 使用
				if p.处理格子弹窗() {
					连续空格子 = 0 // 有物品 → 计数清零
					// 用过物品后背包会重排：后面的物品往前补位，所以下一次点击仍点这一格（原来的位置）
					col--
				} else {
					连续空格子++
					core.LogWithTimef("[物品使用] 空格子，连续 %d 次", 连续空格子)
					if 连续空格子 >= 连续空格子上限 {
						core.LogWithTimef("[物品使用] 连续 %d 次空格子 → 结束当前分类【%s】", 连续空格子上限, 分类)
						row, col = 格子行数, 格子列数 // 双重跳出遍历
						break
					}
				}
			}
		}
	}

	core.LogWithTimef("[物品使用] 全部处理完成")
	return "ok", nil
}

// 处理格子弹窗 点开一个格子后，自适应循环处理弹出来的按钮：
//   - 【使用】→ 点它（使用物品）
//   - 【打开】→ 点它（展开物品）
//   - 【点击关闭】→ 点它（收起弹窗，结束按钮）
//
// 用 ClickAdaptive 自适应：按顺序一个一个找、找到一个就点一个；格子点开后的弹窗、
// 按钮可能连锁出现，循环点到连续 3 秒没有按钮为止。
//
// 返回 true 表示该格子有物品（至少点中过按钮），false 表示空格子。
func (p *物品使用) 处理格子弹窗() bool {
	点中过 := false
	ClickAdaptive(2000, 1000, func(name string) {
		// 结束按钮（关闭/点击关闭）只是收尾，不代表该格子有物品，不算"点中过"
		if name == "关闭" || name == "盒子列表" || name == "全部" {
			return
		}
		点中过 = true
	},
		btn("使用"),
		btn("打开"),
		btn("选蓝色装备"),
		btn("选黄色武功"),
		btn("生命的精华"),
		btn("狙击强化书"),
		/*	btn("连击强化书"),
			btn("强击强化书"),
			btn("毁灭强化书"),*/
		btn("铁矿"),
		/*		btn("布料"),
				btn("皮革"),
				btn("宝石"),*/
		btn("全部"),
		btn("确认"),
		btn("盒子列表"),
		btn("关闭", 731, 51, 825, 146, true), // 结束按钮（限定区域，避免误点）
		btn("点击关闭", true),                // 结束按钮

	)
	core.LogWithTimef("[物品使用] 格子处理完毕，有点中过=%v", 点中过)
	return 点中过
}
