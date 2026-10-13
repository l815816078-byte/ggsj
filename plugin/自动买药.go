package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&自动买药{})
}

// 地点 一个城市下的地点：名称 + 战力门槛 + 商店类型（可选）
type 地点 struct {
	名称   string
	战力门槛 int
	商店类型 string // 商店类型名（可选；空 = 不需要点商店类型）
}

// 地图配置 通用配置（配置好主城/地点数组）
type 地图配置 struct {
	主城名  string // 主城名（recogs/字库预设名）
	地点列表 []地点   // 该主城下的地点数组
}

// 地图配置表 可配多个城市（用时指定"哪个主城下的哪个地点"）
var 地图配置表 = []地图配置{
	{
		主城名: "塔巴罗城",
		地点列表: []地点{
			{名称: "商店", 战力门槛: 0, 商店类型: "药品商店"},
		},
	},
	{
		主城名: "英勇的清山",
		地点列表: []地点{
			{名称: "倒塌的据点", 战力门槛: 50000},
			{名称: "废弃通道", 战力门槛: 50000},
			{名称: "基土小路", 战力门槛: 38000},
			{名称: "不详的湖", 战力门槛: 38000},
			{名称: "高处岔路", 战力门槛: 25000},
		},
	},
}

// 指定"去哪个主城下的哪个地点"（用时改这两个变量）
var (
	指定主城 = "塔巴罗城"
	指定地点 = "商店"
)

// 药品下限：当前药品数量低于该值就买药（默认 50）
var 药品下限 = 50

// 取配置 按 指定主城/指定地点 取出配置与地点；找不到返回 ok=false
func 取配置(主城名, 地点名 string) (地图配置, 地点, bool) {
	for _, c := range 地图配置表 {
		if c.主城名 != 主城名 {
			continue
		}
		for _, p := range c.地点列表 {
			if p.名称 == 地点名 {
				return c, p, true
			}
		}
	}
	return 地图配置{}, 地点{}, false
}

// 取主城配置 按主城名取出配置（不指定地点）；找不到返回 ok=false
func 取主城配置(主城名 string) (地图配置, bool) {
	for _, c := range 地图配置表 {
		if c.主城名 == 主城名 {
			return c, true
		}
	}
	return 地图配置{}, false
}

// 找主城/折叠开的区域（地图列表）
var 地图列表区域 = [4]int{79, 108, 333, 720}

// 折叠开循环里上下滑动的起终点（从上往下）
var (
	列表滑动起点 = [2]int{185, 323}
	列表滑动终点 = [2]int{187, 669}
)

// 找地点时的滑动起终点（从下往上）
var (
	地点滑动起点 = [2]int{187, 669}
	地点滑动终点 = [2]int{185, 123}
)

const (
	列表滑动时长  = 2000
	主城最多找   = 2 // 主城循环最多找几次
	折叠开最多滑动 = 3 // 折叠开循环里最多滑动次数
	地点最多滑动  = 6 // 找地点时最多滑动次数（参考野外挂机）
)

// 自动买药 插件：检测缺药 → 去主城 → 找地点 → 点商店类型 → 走传送那套。
//
// 流程：
//  1. 检测【是否缺药】，不缺 → 直接结束
//  2. 打开【地图】→【地图列表】
//  3. 循环找主城（区域 79,108,333,720）：找到点击 → 停止
//     没找到 → 循环找【折叠开】：找到点击 → 停止；没找到 → 从上往下滑动3次，还找不到 → 停
//  4. 找到主城后找地点（商店）→ 找到点它；没找到 → 滑动6次找
//  5. 点开地点后：若配了商店类型 → 检测点击【药品商店】
//  6. 正常走【绿色按钮】那套（绿色按钮 / 前往 / 立即传送循环）
//  7. 去地图成功后：循环找【药店npc】(超时30秒/间隔2秒) → 点【药水】→ 全部 → 确认 → 购买 → 退出
type 自动买药 struct{}

func (p *自动买药) Name() string { return "自动买药" }

func (p *自动买药) Exec(ctx *Context) (string, error) {
	// 1) 检测是否缺药
	if !core.Exec("是否缺药").Found {
		core.LogWithTimef("[自动买药] 未检测到缺药 → 结束")
		return "ok", nil
	}
	core.LogWithTimef("[自动买药] 检测到缺药 → 开始去主城买药")

	// 指定"去哪个主城下的哪个地点"（用时改这两个变量）
	配置, 目标地点, ok := 取配置(指定主城, 指定地点)
	if !ok {
		core.LogWithTimef("[自动买药] 未找到配置 主城=%q 地点=%q → 结束", 指定主城, 指定地点)
		return "no", nil
	}
	core.LogWithTimef("[自动买药] 目标：主城【%s】→ 地点【%s】(战力门槛 %d)", 配置.主城名, 目标地点.名称, 目标地点.战力门槛)

	if !去地图(配置, 目标地点) {
		core.LogWithTimef("[自动买药] 去地图失败 → 结束")
		return "fail", nil
	}

	// 去地图成功后 → 找药店NPC买药
	买药()

	return "ok", nil
}

// 买药 到达地点后：循环找【药店npc】(超时30秒/间隔1-2秒) → 点击 → 点【药水】→ 全部 → 确认 → 购买 → 退出
func 买药() {
	// 1) 循环找药店npc（超时 30 秒，间隔 1-2 秒）
	deadline := time.Now().Add(30 * time.Second)
	找到 := false
	for time.Now().Before(deadline) {
		if IsStopped() {
			return
		}
		if ClickRecog("药店npc", 238, 53, 960, 627) {
			找到 = true
			core.RandomSleep(3000, 6000)
			break
		}
		core.RandomSleep(1000, 2000)
	}
	if !找到 {
		core.LogWithTimef("[买药] 30秒内未找到药店npc → 结束")
		return
	}
	core.RandomSleep(500, 1000)

	// 2) 点【药水】
	ClickRecog("药水")
	core.RandomSleep(500, 1000)
	// 3) 全部 → 确认 → 购买 → 退出
	ClickRecog("全部")
	core.RandomSleep(500, 1000)
	ClickRecog("确认")
	core.RandomSleep(500, 1000)
	ClickRecog("购买")
	core.RandomSleep(500, 1000)
	ClickRecog("退出")
	core.LogWithTimef("[买药] 买药流程结束")
}

// 去地图 打开地图→地图列表，找主城（找不到进折叠开循环），找到主城后去地点。
// 返回 true = 成功去成；false = 中途失败（没打开地图/没找到主城/没找到地点/没传送到）
func 去地图(配置 地图配置, 目标地点 地点) bool {
	// 打开地图 → 地图列表（任一没找到 → 直接失败，不往下走）
	if !CheckLoopClick("地图", 6*time.Second, 2*time.Second) {
		core.LogWithTimef("[去地图] 未找到【地图】→ 失败")
		return false
	}
	if !CheckLoopClick("地图列表", 6*time.Second, 2*time.Second) {
		core.LogWithTimef("[去地图] 未找到【地图列表】→ 失败")
		return false
	}

	// 主城循环（最多找 主城最多找 次）；找不到主城 → 内层折叠开循环（不 break，滑满3次自然结束）
	找到主城 := false
	for i := 1; i <= 主城最多找; i++ {
		if IsStopped() {
			return false
		}
		r := core.Exec(配置.主城名, 地图列表区域[:]...)
		if r.Found {
			// 用识别到的主城命中点坐标 + 偏移 (195,-4) → 得到"折叠开"检测区域（40×40）
			折叠开区 := [4]int{r.X + 153, r.Y - 10, r.X + 153 + 40, r.Y - 10 + 40}
			core.LogWithTimef("[去地图] 主城命中 %d,%d → 折叠开检测区 %d,%d,%d,%d",
				r.X, r.Y, 折叠开区[0], 折叠开区[1], 折叠开区[2], 折叠开区[3])
			if core.Exec("折叠开", 折叠开区[:]...).Found {
				// 检测到折叠开 → 主城已展开，不点主城
				core.LogWithTimef("[去地图] 第%d次找到主城【%s】%d,%d → 已展开，不点击", i, 配置.主城名, r.X, r.Y)
			} else {
				// 没检测到折叠开 → 点击主城展开
				core.LogWithTimef("[去地图] 第%d次找到主城【%s】%d,%d → 点开", i, 配置.主城名, r.X, r.Y)
				core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+10, r.Y+10)
			}
			找到主城 = true
			break
		}
		core.LogWithTimef("[去地图] 第%d次未找到主城【%s】→ 进折叠开循环", i, 配置.主城名)

		// 折叠开循环（内层）：找到点击；没找到滑动（最多3次，滑满自然结束，不 break）
		for j := 1; j <= 折叠开最多滑动; j++ {
			if IsStopped() {
				return false
			}
			f := core.Exec("折叠开", 地图列表区域[:]...)
			if f.Found {
				core.LogWithTimef("[去地图] 折叠开第%d次找到【折叠开】%d,%d → 点击", j, f.X, f.Y)
				core.RandomClickInAreaWithDelay(f.X, f.Y, f.X+10, f.Y+10)
				core.RandomSleep(500, 1000)
				continue
			}
			core.LogWithTimef("[去地图] 折叠开第%d次未找到【折叠开】→ 滑动 %d,%d → %d,%d",
				j, 列表滑动起点[0], 列表滑动起点[1], 列表滑动终点[0], 列表滑动终点[1])
			core.RandomSwipe(列表滑动起点[0], 列表滑动起点[1], 列表滑动终点[0], 列表滑动终点[1], 列表滑动时长, 2000)
		}
		// 折叠开循环结束 → 回主城循环继续找主城
	}

	// 只有找到主城才找地点
	if !找到主城 {
		core.LogWithTimef("[去地图] %d次都没找到主城【%s】→ 失败", 主城最多找, 配置.主城名)
		return false
	}
	return 去地点(配置, 目标地点)
}

// 去地点 找地点（找不到就滑动），点开后可选点商店类型，最后走绿色按钮那套。
// 返回 true = 成功去成；false = 没找到地点/没传送到
func 去地点(配置 地图配置, 目标地点 地点) bool {
	// 4) 找地点（找到点它；没找到 → 滑动 地点最多滑动 次）
	找到 := false
	for i := 1; i <= 地点最多滑动; i++ {
		if IsStopped() {
			return false
		}
		r := core.Exec(目标地点.名称, 地图列表区域[:]...)
		if r.Found {
			core.LogWithTimef("[自动买药] 第%d次找到地点【%s】%d,%d → 点击", i, 目标地点.名称, r.X, r.Y)
			core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+10, r.Y+10)
			找到 = true
			break
		}
		core.LogWithTimef("[自动买药] 第%d次未找到地点【%s】→ 滑动 %d,%d → %d,%d",
			i, 目标地点.名称, 地点滑动起点[0], 地点滑动起点[1], 地点滑动终点[0], 地点滑动终点[1])
		core.RandomSwipe(地点滑动起点[0], 地点滑动起点[1], 地点滑动终点[0], 地点滑动终点[1], 列表滑动时长, 2500)
	}
	if !找到 {
		core.LogWithTimef("[自动买药] 未找到地点【%s】→ 失败", 目标地点.名称)
		return false
	}
	core.RandomSleep(500, 1000)

	// 5) 点开地点后：若配了商店类型 → 检测点击
	if 目标地点.商店类型 != "" {
		if ClickRecog(目标地点.商店类型) {
			core.LogWithTimef("[自动买药] 点击商店类型【%s】", 目标地点.商店类型)
			core.RandomSleep(500, 1000)
		}
	}

	// 6) 正常走绿色按钮那套（绿色按钮 = 原来的【咒术传送】）
	return 传送那套()
}

// 传送那套 绿色按钮（= 原来的【咒术传送】）→ 没找到则前往 → 循环检测立即传送（和野外挂机一致）。
// 返回 true = 成功传送；false = 没传送到
func 传送那套() bool {
	// 绿色按钮 = 原来的【咒术传送】
	if ClickRecog("绿色按钮") {
		return true
	}
	core.LogWithTimef("[自动买药] 未找到【绿色按钮】→ 点【前往】")
	ClickRecog("前往")

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if IsStopped() {
			return false
		}
		if ClickRecog("立即传送") {
			// 绿色按钮 = 原来的【咒术传送】
			ClickRecog("绿色按钮")
			return true
		}
		core.RandomSleep(1000, 1500)
	}
	core.LogWithTimef("[自动买药] 未检测到【立即传送】→ 失败")
	return false
}
