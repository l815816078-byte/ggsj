package plugin

import (
	"math"

	"app/core"
	imageHandler "app/imageHandler"
)

func init() {
	Register(&躲避技能{})
}

// 躲避技能 插件
//
// 流程：
//  1. 检测 BOSS(416,45,534,160)：检测不到 → 直接停
//  2. 截屏 → inRange 提取危险区（红）→ 八连通域，得到每个危险块的 Center / Rect
//  3. 角色固定在屏幕 (640,360)，逐个块按"圆心+半径"判断角色是否在危险里
//  4. 在危险里 → 算"背离该圆心的方向"，滑摇杆一次，重新检测 BOSS + 识别
//     不在危险里 → 继续循环（BOSS 还在就继续守着）
//
// 摇杆范围 149,398,350,600 → 中心(249,499)，拉杆长度 80（不超过半径，防滑出范围）。
type 躲避技能 struct{}

func (p *躲避技能) Name() string { return "躲避技能" }

// 角色固定屏幕坐标（屏幕 1280x720）
const (
	躲避角色X = 640
	躲避角色Y = 360
)

// 摇杆：中心 (249,499)，拉杆最大长度 80
const (
	摇杆中心X = 249
	摇杆中心Y = 499
	摇杆拉杆  = 80
)

// 单次滑动时长（毫秒）：按住不放走一段距离
const 躲避滑动时长 = 2000

// 小地图区域 [x1,y1,x2,y2]（人物在小地图中心）
var 小地图区域 = [4]int{22, 35, 196, 206}

// 小地图射线采样参数
const (
	小地图采样方向数 = 24  // 射线方向数（每 15° 一条）
	小地图采样步长   = 3   // 每步采样像素
	小地图采样最大   = 120 // 单条射线最长采样距离（像素）
	小地图可走最小   = 12  // 连续白色达到该长度才算"可走"
	小地图白色阈值   = 128 // 灰度 ≥ 该值算白色（可走）
)

func (p *躲避技能) Exec(ctx *Context) (string, error) {
	for {
		// 检测不到 BOSS → 直接停
		if !core.Exec("BOSS", 416, 45, 534, 160).Found {
			core.LogWithTimef("[躲避技能] 未检测到 BOSS → 结束")
			return "ok", nil
		}
		if IsStopped() {
			return "no", nil
		}

		src := imageHandler.CaptureScreenMat(0, 0, 0, 0)
		if src.Empty() {
			core.LogWithTimef("[躲避技能] 截屏为空")
			return "no", nil
		}
		危险 := imageHandler.InRange(src, []int{179, 0, 0}, []int{255, 154, 192})
		src.Recycle()

		blocks := imageHandler.ConnectedComponentsWithStats(危险, 8)
		危险.Recycle()

		// 找"包含角色"的危险块（圆心 + 半径判定，避免矩形四角误判）
		// 过滤：长宽 >= 200 且 白点面积 >= 30000 才算真正的危险区（滤掉 UI/噪点碎块）
		var 目标 *imageHandler.ConnectedState
		for j := range blocks {
			b := &blocks[j]
			宽 := b.Rect.Right - b.Rect.Left
			高 := b.Rect.Bottom - b.Rect.Top
			if 宽 < 200 || 高 < 200 || b.WhiteArea < 30000 {
				continue
			}
			半径 := float64(宽+高) / 4.0
			距离 := math.Hypot(float64(躲避角色X-b.Center.X), float64(躲避角色Y-b.Center.Y))
			if 距离 < 半径 {
				目标 = b
				break
			}
		}

		if 目标 == nil {
			core.LogWithTimef("[躲避技能] 角色不在危险区 → 继续守着")
			core.RandomSleep(300, 500)
			continue
		}

		// 跑动方向：优先用小地图"可走方向"（白=可走），找不到再退化为"背离危险圆心"
		dx, dy, 可走 := 小地图可走方向()
		if !可走 {
			// 背离圆心方向：从圆心指向角色
			// 角色固定在屏幕中心，若危险区圆心也压在这附近，方向退化 → 改朝"离圆心最远的屏幕角落"跑，保证方向非零
			dx = float64(躲避角色X - 目标.Center.X)
			dy = float64(躲避角色Y - 目标.Center.Y)
			长度 := math.Hypot(dx, dy)
			if 长度 < 1 {
				dx, dy = float64(640-目标.Center.X), float64(360-目标.Center.Y)
				长度 = math.Hypot(dx, dy)
			}
			if 长度 < 1 { // 圆心正好压在屏幕中心 → 任选一个方向（向下）
				dx, dy, 长度 = 0, 1, 1
			}
			dx, dy = dx/长度, dy/长度
			core.LogWithTimef("[躲避技能] 小地图无可走方向 → 退回背离危险方向(%.2f,%.2f)", dx, dy)
		}

		// 摇杆：中心 → 中心 + 方向*拉杆（不超出摇杆范围）
		终点X := 摇杆中心X + int(dx*摇杆拉杆)
		终点Y := 摇杆中心Y + int(dy*摇杆拉杆)
		core.LogWithTimef("[躲避技能] 危险块 区域(%d,%d,%d,%d) 中心(%d,%d) → 背离方向(%.2f,%.2f) 滑动(%d,%d)→(%d,%d)",
			目标.Rect.Left, 目标.Rect.Top, 目标.Rect.Right, 目标.Rect.Bottom,
			目标.Center.X, 目标.Center.Y, dx, dy,
			摇杆中心X, 摇杆中心Y, 终点X, 终点Y)
		core.RandomSwipe2(摇杆中心X, 摇杆中心Y, 终点X, 终点Y, 躲避滑动时长, 5)
	}

	core.LogWithTimef("[躲避技能] 达到最大躲避次数 → 结束")
	return "ok", nil
}

// 小地图可走方向 检测小地图，返回一个"可走方向"的单位向量：
//  1. 截小地图 → 双阈值二值化（白=可走）；
//  2. InRange 提取箭头掩膜 → 反相 → 与二值图相与，把箭头从可走图里抹掉；
//  3. 以区域中心（人物）为原点，向 小地图采样方向数 个方向发射射线，统计
//     "跳过开头黑（中心/箭头遮挡）后的连续白色长度"；
//  4. 取连续白色最长的方向；全部达不到 小地图可走最小 → ok=false。
func 小地图可走方向() (dx, dy float64, ok bool) {
	m := imageHandler.CaptureScreenMat(小地图区域[0], 小地图区域[1], 小地图区域[2], 小地图区域[3])
	if m.Empty() {
		core.LogWithTimef("[躲避技能] 小地图截图为空")
		return 0, 0, false
	}
	defer m.Recycle()

	二值 := imageHandler.DoubleBinaryzation(m, 100, 149, 255)
	defer 二值.Recycle()
	箭头 := imageHandler.InRange(m, []int{179, 0, 0}, []int{255, 255, 255})
	defer 箭头.Recycle()
	反箭头 := imageHandler.Bitwise_not(箭头, nil)
	defer 反箭头.Recycle()
	可走 := imageHandler.Bitwise_and(二值, 反箭头, nil)
	defer 可走.Recycle()

	宽 := 小地图区域[2] - 小地图区域[0]
	高 := 小地图区域[3] - 小地图区域[1]
	cx, cy := 宽/2, 高/2

	最长 := 0
	bestDX, bestDY := 0.0, 0.0
	for k := 0; k < 小地图采样方向数; k++ {
		角 := 2 * math.Pi * float64(k) / 小地图采样方向数
		ux, uy := math.Cos(角), math.Sin(角)
		开始 := false
		长度 := 0
		for 步 := 1; 步*小地图采样步长 <= 小地图采样最大; 步++ {
			x := cx + int(ux*float64(步*小地图采样步长))
			y := cy + int(uy*float64(步*小地图采样步长))
			if x < 0 || y < 0 || x >= 宽 || y >= 高 {
				break
			}
			if imageHandler.GetRed(可走, x, y) >= 小地图白色阈值 {
				开始 = true
				长度 = 步 * 小地图采样步长
			} else if 开始 {
				break // 白段结束
			}
		}
		if 长度 > 最长 {
			最长 = 长度
			bestDX, bestDY = ux, uy
		}
	}

	core.LogWithTimef("[躲避技能] 小地图可走方向: 最长连续白=%d 方向(%.2f,%.2f)", 最长, bestDX, bestDY)
	if 最长 < 小地图可走最小 {
		return 0, 0, false
	}
	return bestDX, bestDY, true
}
