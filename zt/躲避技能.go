package zt

import (
	"math"
	"sort"
	"time"

	"app/core"
	imageHandler "app/imageHandler"

	"github.com/Dasongzi1366/AutoGo/motion"
)

// ══════════════════════════════════════════════════════════════════════════
// 躲避技能（照 plugin/躲避技能.go 搬；主线任务 / 支线任务 打 BOSS 时调）
//
// 老框架是插件内部一个大循环（检测到 BOSS 就一直躲），zt 改成【每轮干一格】：
//	每轮：BOSS 检测（10 秒一次）→ 没 BOSS 直接返回 → 截屏算危险区 → 危险就摇杆躲一下
//	跨轮要留的东西（BOSS 名 / 血量 / 摇杆按住状态）放包级变量，重置时清。
//
// 流程（照 plugin）：
//  1. 检测 BOSS(416,45,534,160)：检测不到 → 直接返回
//  2. 截屏 → inRange 提取危险区（红）→ 最小外接矩形
//  3. 角色固定在屏幕 (640,360)，判断这些矩形是不是"危险"
//  4. 危险 → 算背离方向，滑摇杆躲一次；不危险 → 松手
//
// 摇杆范围 149,398,350,600 → 中心(249,499)，拉杆长度 80（不超过半径，防滑出范围）。
// ══════════════════════════════════════════════════════════════════════════

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
	摇杆手指  = 5 // 按住/移动/抬起用同一个 fingerID
)

// 单次滑动时长（毫秒）：按住不放走一段距离
const 躲避滑动时长 = 1500

// 射线探测（找最远安全方向）
const (
	射线数       = 24  // 方向数（每 15° 一条）
	射线步长     = 10  // 每走 10px 检测一次是否碰到矩形
	射线最大距离 = 550 // 单条射线最长探测距离（像素）
)

// 危险判定：判断"当前是不是危险"（决定要不要躲）
//   - 存在一个矩形：宽 >= 危险最小边长 或 高 >= 危险最小边长
//   - 且该矩形与 危险判定区域 有重叠（1 像素也算）
var 危险判定区域 = [4]int{588, 219, 688, 401} // x1,y1,x2,y2

const 危险最小边长 = 100 // 矩形宽或高达到该值才算得上"危险"

// 小地图区域 [x1,y1,x2,y2]（人物在小地图中心）
var 小地图区域 = [4]int{22, 35, 196, 206}

// 小地图射线采样参数
const (
	小地图采样方向数 = 23  // 射线方向数
	小地图采样步长   = 3   // 每步采样像素
	小地图采样最大   = 250 // 单条射线最长采样距离
	小地图可走最小   = 12  // 连续白色达到该长度才算"可走"
	小地图白色阈值   = 128 // 灰度 ≥ 该值算白色（可走）
)

// BOSS名单 / 危险技能默认 / 危险技能按BOSS（+ BOSS技能 类型）都配在 点库.go 的「业务配置」段

// ── 躲避技能自己的状态（跨轮保留）─────────────────────────────────────────
var (
	躲避BOSS名           = ""        // 当前 BOSS 名（检测到一次就固化）
	躲避剩3血            = false     // BOSS 剩 3 条血
	躲避当前血量          = 0         // 当前血量（只识别到"3"，没识别到 = 0 未知）
	躲避下次检测BOSS      time.Time  // 下次该检 BOSS 的时间（10 秒一次）
	躲避摇杆按住中        = false     // 摇杆是否按住中
	躲避上次打BOSS任务名  = ""        // 哪个任务在打 BOSS
	躲避上次打BOSS时间    time.Time  // 上次打 BOSS 的时间
)

// 躲避技能重置 由调用它的任务（主线任务/支线任务）重置时调
func 躲避技能重置() {
	躲避BOSS名 = ""
	躲避剩3血 = false
	躲避当前血量 = 0
	躲避下次检测BOSS = time.Time{}
	摇杆松手(&躲避摇杆按住中)
}

// 躲避技能跑 每轮：检 BOSS → 有危险就摇杆躲一下（没 BOSS 直接返回）
func 躲避技能跑() {
	if 已停止() {
		摇杆松手(&躲避摇杆按住中)
		return
	}

	// BOSS 检测 10 秒一次（不用每轮都检）
	if time.Now().After(躲避下次检测BOSS) {
		if _, _, ok := 识别("BOSS", 416, 45, 534, 160); !ok {
			摇杆松手(&躲避摇杆按住中)
			core.LogWithTimef("[躲避技能] 未检测到 BOSS → 本轮不躲")
			return
		}
		躲避上次打BOSS任务名 = 当前任务
		躲避上次打BOSS时间 = time.Now()

		// 血量"3"（剩 3 条血）：检测到一次就固化
		if !躲避剩3血 {
			_, _, 躲避剩3血 = 识别("3", 782, 96, 790, 107)
		}
		if 躲避剩3血 {
			躲避当前血量 = 3
		}
		if 躲避BOSS名 == "" {
			躲避BOSS名 = 当前BOSS名()
		}
		core.LogWithTimef("[躲避技能] 当前 BOSS名=%q 血量=%d 危险技能范围=%d个",
			躲避BOSS名, 躲避当前血量, len(危险颜色范围(躲避BOSS名, 躲避当前血量)))
		躲避下次检测BOSS = time.Now().Add(10 * time.Second)
	}

	src := imageHandler.CaptureScreenMat(0, 0, 0, 0)
	if src.Empty() {
		摇杆松手(&躲避摇杆按住中)
		core.LogWithTimef("[躲避技能] 截屏为空")
		return
	}
	// 危险区域：把当前 BOSS 的所有技能颜色范围都提取出来，合并成一张"危险图"
	var 危险 *imageHandler.Mat
	for _, r := range 危险颜色范围(躲避BOSS名, 躲避当前血量) {
		m := imageHandler.InRange(src, r.下限, r.上限)
		if 危险 == nil {
			危险 = m
		} else {
			合 := imageHandler.Bitwise_or(危险, m, nil)
			危险.Recycle()
			m.Recycle()
			危险 = 合
		}
	}
	src.Recycle()

	// 膨胀：把碎白点连成片，减少轮廓/矩形数量
	膨胀 := imageHandler.Dilate(危险, 0, 5, 5, -1, -1, 1)
	危险.Recycle()

	// 最小外接矩形：每个白块一个矩形（4 角点），能贴合斜的技能圈
	minRects := imageHandler.GetContoursMinRect(膨胀, 0, 2, nil)
	膨胀.Recycle()

	// 不是危险 → 松手，本轮结束
	if !是危险(minRects) {
		摇杆松手(&躲避摇杆按住中)
		return
	}

	// 找躲避方向：射线取"一路不碰矩形、能走最远"的方向
	dx, dy, 最大通行 := 最远安全方向(minRects, 射线数, 射线步长, 射线最大距离)
	if 最大通行 == 0 {
		// 所有方向都被挡住（被围死）→ 退化为"小地图可走方向"，再不行向下
		dx, dy = 0, 1
		if 可走列表, _ := 小地图可走方向(); len(可走列表) > 0 {
			dx, dy = 可走列表[0][0], 可走列表[0][1]
		}
		core.LogWithTimef("[躲避技能] 所有方向都被技能矩形挡住 → 兜底(%.2f,%.2f)", dx, dy)
	}

	core.LogWithTimef("[躲避技能] 危险矩形=%d个 安全方向(%.2f,%.2f) 通行距离=%d",
		len(minRects), dx, dy, 最大通行)
	摇杆移动(&躲避摇杆按住中, dx, dy)
	core.RandomSleep(100, 300)
}

// 摇杆移动 按住摇杆并朝 (dx,dy) 方向推（已按住则只改向，实时改向）
func 摇杆移动(按住中 *bool, dx, dy float64) {
	终点X := 摇杆中心X + int(dx*摇杆拉杆)
	终点Y := 摇杆中心Y + int(dy*摇杆拉杆)
	if !*按住中 {
		motion.TouchDown(摇杆中心X, 摇杆中心Y, 摇杆手指, 0)
		*按住中 = true
	}
	motion.TouchMove(终点X, 终点Y, 摇杆手指, 0)
}

// 摇杆松手 松开摇杆并回中（避免卡在边缘一直走）；没按住则什么都不做
func 摇杆松手(按住中 *bool) {
	if !*按住中 {
		return
	}
	motion.TouchMove(摇杆中心X, 摇杆中心Y, 摇杆手指, 0)
	motion.TouchUp(摇杆中心X, 摇杆中心Y, 摇杆手指, 0)
	*按住中 = false
}

// 危险颜色范围 返回当前要检测的危险技能颜色范围列表（照 plugin）
func 危险颜色范围(BOSS名 string, 当前血量 int) []BOSS技能 {
	按BOSS := 危险技能按BOSS[BOSS名]
	if BOSS名 == "" || len(按BOSS) == 0 {
		var 范围 []BOSS技能
		for _, sk := range 危险技能默认 {
			if sk.触发血量 == 0 || sk.触发血量 == 当前血量 {
				范围 = append(范围, sk)
			}
		}
		return 范围
	}
	var 范围 []BOSS技能
	for _, sk := range 按BOSS {
		if sk.触发血量 == 0 || sk.触发血量 == 当前血量 {
			范围 = append(范围, sk)
		}
	}
	return 范围
}

// 当前BOSS名 按 BOSS名单 逐个识别，命中哪个就是哪个；都没命中 → 空串（用默认技能范围）
func 当前BOSS名() string {
	for _, n := range BOSS名单 {
		if x, y, ok := 识别(n); ok {
			core.LogWithTimef("[躲避技能] 检测到 BOSS名【%s】(%d,%d)", n, x, y)
			return n
		}
		core.LogWithTimef("[躲避技能] BOSS名【%s】未命中", n)
	}
	core.LogWithTimef("[躲避技能] 未检测到任何 BOSS名 → 用默认技能范围")
	return ""
}

// 小地图可走方向 检测小地图，返回所有"可走方向"的单位向量列表（按连续白从长到短排序）
func 小地图可走方向() (方向列表 [][2]float64, ok bool) {
	m := imageHandler.CaptureScreenMat(小地图区域[0], 小地图区域[1], 小地图区域[2], 小地图区域[3])
	if m.Empty() {
		core.LogWithTimef("[躲避技能] 小地图截图为空")
		return nil, false
	}
	defer m.Recycle()

	二值 := imageHandler.DoubleBinaryzation(m, 100, 255, 255)
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

	长度表 := make([]int, 小地图采样方向数)
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
				break
			}
		}
		长度表[k] = 长度
	}

	type 方向长度 struct {
		ux, uy float64
		长度   int
	}
	var 全部 []方向长度
	for k := 0; k < 小地图采样方向数; k++ {
		if 长度表[k] < 小地图可走最小 {
			continue
		}
		角 := 2 * math.Pi * float64(k) / 小地图采样方向数
		全部 = append(全部, 方向长度{ux: math.Cos(角), uy: math.Sin(角), 长度: 长度表[k]})
	}
	for i := 0; i < len(全部); i++ {
		for j := i + 1; j < len(全部); j++ {
			if 全部[j].长度 > 全部[i].长度 {
				全部[i], 全部[j] = 全部[j], 全部[i]
			}
		}
	}
	for _, d := range 全部 {
		方向列表 = append(方向列表, [2]float64{d.ux, d.uy})
	}

	if len(方向列表) == 0 {
		core.LogWithTimef("[躲避技能] 小地图无可走方向")
		return nil, false
	}
	core.LogWithTimef("[躲避技能] 小地图可走方向 %d 个，最优(%.2f,%.2f)", len(方向列表), 方向列表[0][0], 方向列表[0][1])
	return 方向列表, true
}

// 是危险 判断"当前是不是危险"：存在大矩形且与 危险判定区域 有重叠
func 是危险(矩形列表 [][]imageHandler.Point) bool {
	for _, 矩形 := range 矩形列表 {
		左, 上, 右, 下, ok := 矩形包围盒(矩形)
		if !ok {
			continue
		}
		if 右-左 < 危险最小边长 && 下-上 < 危险最小边长 {
			continue
		}
		if 右 < 危险判定区域[0] || 左 > 危险判定区域[2] ||
			下 < 危险判定区域[1] || 上 > 危险判定区域[3] {
			continue
		}
		return true
	}
	return false
}

// 矩形包围盒 求一组角点的轴对齐包围盒 (left,top,right,bottom)
func 矩形包围盒(角点 []imageHandler.Point) (左, 上, 右, 下 int, ok bool) {
	if len(角点) == 0 {
		return 0, 0, 0, 0, false
	}
	左, 上 = 角点[0].X, 角点[0].Y
	右, 下 = 角点[0].X, 角点[0].Y
	for _, p := range 角点[1:] {
		if p.X < 左 {
			左 = p.X
		}
		if p.X > 右 {
			右 = p.X
		}
		if p.Y < 上 {
			上 = p.Y
		}
		if p.Y > 下 {
			下 = p.Y
		}
	}
	return 左, 上, 右, 下, true
}

// 最远安全方向 从角色出发向各方向探测，返回通行距离最长的方向（单位向量）+ 通行距离
func 最远安全方向(矩形列表 [][]imageHandler.Point, 方向数, 步长, 最大距离 int) (dx, dy float64, 最远通行 int) {
	最远通行 = 0
	for k := 0; k < 方向数; k++ {
		角 := 2 * math.Pi * float64(k) / float64(方向数)
		ux, uy := math.Cos(角), math.Sin(角)
		通行 := 最大距离
		for 距 := 步长; 距 <= 最大距离; 距 += 步长 {
			x := 躲避角色X + int(ux*float64(距))
			y := 躲避角色Y + int(uy*float64(距))
			if x < 0 || y < 0 || x >= 1280 || y >= 720 {
				通行 = 距 - 步长
				break
			}
			if 点在任意矩形内(矩形列表, x, y) {
				通行 = 距 - 步长
				break
			}
		}
		if 通行 > 最远通行 {
			最远通行 = 通行
			dx, dy = ux, uy
		}
	}
	return dx, dy, 最远通行
}

// 点在任意矩形内 点是否落在矩形列表里的任意一个最小矩形内
func 点在任意矩形内(矩形列表 [][]imageHandler.Point, x, y int) bool {
	for _, 矩形 := range 矩形列表 {
		if 点在矩形内(矩形, x, y) {
			return true
		}
	}
	return false
}

// 点在矩形内 点 (x,y) 是否在一个（凸四边形）矩形内
func 点在矩形内(角点 []imageHandler.Point, x, y int) bool {
	if len(角点) < 3 {
		return false
	}
	pts := make([]imageHandler.Point, len(角点))
	copy(pts, 角点)
	cx, cy := 0.0, 0.0
	for _, p := range pts {
		cx += float64(p.X)
		cy += float64(p.Y)
	}
	cx /= float64(len(pts))
	cy /= float64(len(pts))
	sort.Slice(pts, func(i, j int) bool {
		ai := math.Atan2(float64(pts[i].Y)-cy, float64(pts[i].X)-cx)
		aj := math.Atan2(float64(pts[j].Y)-cy, float64(pts[j].X)-cx)
		return ai < aj
	})
	n := len(pts)
	正, 负 := false, false
	for i := 0; i < n; i++ {
		a := pts[i]
		b := pts[(i+1)%n]
		叉 := float64(b.X-a.X)*float64(y-a.Y) - float64(b.Y-a.Y)*float64(x-a.X)
		if 叉 > 0 {
			正 = true
		} else if 叉 < 0 {
			负 = true
		}
		if 正 && 负 {
			return false
		}
	}
	return true
}
