package plugin

import (
	"math"
	"sort"
	"time"

	"app/core"
	imageHandler "app/imageHandler"

	"github.com/Dasongzi1366/AutoGo/motion"
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

// 危险判定：判断"当前是不是危险"（决定要不要躲、要不要刷新"上次危险"）
//   - 存在一个矩形：宽 >= 危险最小边长 或 高 >= 危险最小边长
//   - 且该矩形与 危险判定区域 有重叠（1 像素也算）
var 危险判定区域 = [4]int{588, 219, 688, 401} // x1,y1,x2,y2

const 危险最小边长 = 100 // 矩形宽或高达到该值才算得上"危险"

// 小地图区域 [x1,y1,x2,y2]（人物在小地图中心）
var 小地图区域 = [4]int{22, 35, 196, 206}

// 小地图射线采样参数
const (
	小地图采样方向数 = 23  // 射线方向数（每 15° 一条）
	小地图采样步长   = 3   // 每步采样像素
	小地图采样最大   = 250 // 单条射线最长采样距离（像素）
	小地图可走最小   = 12  // 连续白色达到该长度才算"可走"
	小地图白色阈值   = 128 // 灰度 ≥ 该值算白色（可走）
)

// BOSS技能 一个技能：名字（备注）+ 危险颜色范围（下限/上限）[R,G,B] + 触发血量
type BOSS技能 struct {
	技能名   string
	下限     []int
	上限     []int
	触发血量 int // 0 = 一直检测；非 0 = 只有当前血量等于它才检测（如 3 = 只剩 3 血时才检测）
}

// 危险技能默认 兜底技能：只在"没读到该 BOSS 的专属配置"（BOSS 名空 / 危险技能按BOSS 里没配）时才用。
// 可配多个，会组合到一起检测。
var 危险技能默认 = []BOSS技能{
	{技能名: "默认", 下限: []int{253, 112, 125}, 上限: []int{255, 217, 190}},
}

// 危险技能按BOSS 按 BOSS 名单独配的技能颜色范围（键 = BOSS 名 = recogs 预设名）。
// 配了就只用这里的技能（默认不参与）；一个 BOSS 可配多个技能，会组合到一起检测。
var 危险技能按BOSS = map[string][]BOSS技能{
	"自书鬼": {
		{技能名: "常规", 下限: []int{253, 112, 125}, 上限: []int{255, 217, 190}},
		{技能名: "黑雾", 下限: []int{19, 11, 48}, 上限: []int{32, 21, 78}, 触发血量: 3},
	},
	"主线59万狐狸": {
		{技能名: "常规", 下限: []int{181, 143, 110}, 上限: []int{254, 202, 166}},
	},
}

// BOSS名单 可按 BOSS 名覆盖"危险技能按BOSS"的 BOSS（识别预设名 = BOSS 名）
var BOSS名单 = []string{"自书鬼", "主线59万狐狸"}

// 上次打 BOSS 记录：哪个任务在打 BOSS + 时间（复活插件据此判断"是不是主线打 BOSS 时死的"）
var (
	上次打BOSS任务名 string
	上次打BOSS时间   time.Time
)

func (p *躲避技能) Exec(ctx *Context) (string, error) {
	// BOSS 检测 10 秒一次（不用每轮都检）
	下次检测BOSS := time.Now()
	剩3血 := false // BOSS 剩 3 条血：即使没有危险区域也必须动
	当前血量 := 0  // 当前血量（目前只识别到"3"，没识别到 = 0 未知）
	/*	上次危险 := time.Now() // 上次"检测到危险区域"的时间（找不到危险技能超过 12 秒就动一下）*/
	BOSS名 := ""        // 当前 BOSS 名：检测到一次就确定，不再重复检测
	摇杆按住中 := false // 摇杆是否按住中（按住中只需 TouchMove 实时改向）
	//已存图 := 0           // 调试危险图：本次最多存 500 张
	for {
		if time.Now().After(下次检测BOSS) {
			if !core.Exec("BOSS", 416, 45, 534, 160).Found {
				摇杆松手(&摇杆按住中)
				core.LogWithTimef("[躲避技能] 未检测到 BOSS → 结束")
				return "ok", nil
			}
			// 检测到 BOSS → 记录"哪个任务在打 BOSS + 时间"（复活插件用它判断是不是主线打 BOSS）
			上次打BOSS任务名 = ctx.TaskName
			上次打BOSS时间 = time.Now()

			// 检测到 BOSS 后检测血量"3"（剩 3 条血）：检测到一次就固化，不再重复检测
			if !剩3血 {
				剩3血 = core.Exec("3", 782, 96, 790, 107).Found
			}
			if 剩3血 {
				当前血量 = 3
			}
			// 检测 BOSS 名：检测到一次就确定，之后不再检测
			if BOSS名 == "" {
				BOSS名 = 当前BOSS名()
			}
			core.LogWithTimef("[躲避技能] 当前 BOSS名=%q 血量=%d 危险技能范围=%d个",
				BOSS名, 当前血量, len(危险颜色范围(BOSS名, 当前血量)))
			下次检测BOSS = time.Now().Add(10 * time.Second)
		}
		if IsStopped() {
			摇杆松手(&摇杆按住中)
			return "no", nil
		}

		src := imageHandler.CaptureScreenMat(0, 0, 0, 0)
		if src.Empty() {
			摇杆松手(&摇杆按住中)
			core.LogWithTimef("[躲避技能] 截屏为空")
			return "no", nil
		}
		// 危险区域：把当前 BOSS 的所有技能颜色范围都提取出来，合并成一张"危险图"
		var 危险 *imageHandler.Mat
		for _, r := range 危险颜色范围(BOSS名, 当前血量) {
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

		/*// 调试：把"膨胀图 + 最小矩形"存到手机（文件名带时间戳，唯一，不覆盖；最多存 500 张）
		if 已存图 < 500 {
			画框 := 画最小矩形(膨胀, minRects)
			存路径 := "/sdcard/危险图_" + time.Now().Format("150405_000") + ".png"
			存成功 := imageHandler.SaveMat(画框, 存路径, "png", 100)
			core.LogWithTimef("[躲避技能] 存危险图 矩形=%d 路径=%s 成功=%v", len(minRects), 存路径, 存成功)
			画框.Recycle()
			已存图++
		}*/
		膨胀.Recycle()

		// 危险判定：存在"宽或高 >= 100"且与 危险判定区域 有重叠的矩形 → 当前算危险
		if !是危险(minRects) {
			// 不是危险 → 松手等待
			摇杆松手(&摇杆按住中)
			core.RandomSleep(1000, 1000)
			continue
			// 剩 3 血且超过 12 秒没危险 → 移动一下（先注释掉）
			// if !剩3血 || time.Since(上次危险) < 12*time.Second {
			// 	摇杆松手(&摇杆按住中)
			// 	core.RandomSleep(1000, 1000)
			// 	continue
			// }
			// // 移动一下：方向用小地图可走方向（取最优那个），找不到就向下
			// mx, my := 0.0, 1.0
			// if 可走列表, _ := 小地图可走方向(); len(可走列表) > 0 {
			// 	mx, my = 可走列表[0][0], 可走列表[0][1]
			// }
			// core.LogWithTimef("[躲避技能] BOSS剩3血、超过12秒没找到危险技能 → 移动一下(%.2f,%.2f)", mx, my)
			// 摇杆移动(&摇杆按住中, mx, my)
			// core.RandomSleep(100, 300)
			// continue
		}
		//上次危险 = time.Now() // 只要检测到危险（危险矩形）就刷新

		// 找躲避方向：24 条射线，每条从角色往外走（步长 10，最大 150），
		// 取"一路都不碰任何最小矩形、且能走最远"的方向（躲避用全部最小矩形）
		dx, dy, 最大通行 := 最远安全方向(minRects, 射线数, 射线步长, 射线最大距离)
		if 最大通行 == 0 {
			// 所有方向都被矩形挡住（被围死）→ 退化为"小地图可走方向"，再不行向下
			dx, dy = 0, 1
			if 可走列表, _ := 小地图可走方向(); len(可走列表) > 0 {
				dx, dy = 可走列表[0][0], 可走列表[0][1]
			}
			core.LogWithTimef("[躲避技能] 所有方向都被技能矩形挡住 → 兜底(%.2f,%.2f)", dx, dy)
		}

		// 摇杆：按住并朝方向推（已按住则只改向，实时改向）
		core.LogWithTimef("[躲避技能] 危险矩形=%d个 安全方向(%.2f,%.2f) 通行距离=%d",
			len(minRects), dx, dy, 最大通行)
		摇杆移动(&摇杆按住中, dx, dy)
		core.RandomSleep(100, 300)
	}

	摇杆松手(&摇杆按住中)
	core.LogWithTimef("[躲避技能] 达到最大躲避次数 → 结束")
	return "ok", nil
}

// 摇杆移动 按住摇杆并朝 (dx,dy) 方向推，方向实时可变：
//   - 未按住 → 先 TouchDown 按下，再 TouchMove 推到方向
//   - 已按住 → 只 TouchMove 改方向（按住中实时改向）
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

// 方向夹角 两个单位向量之间的夹角（度）
func 方向夹角(ax, ay, bx, by float64) float64 {
	点 := ax*bx + ay*by
	if 点 > 1 {
		点 = 1
	}
	if 点 < -1 {
		点 = -1
	}
	return math.Acos(点) * 180 / math.Pi
}

// 危险颜色范围 返回当前要检测的危险技能颜色范围列表：
//   - 危险技能默认（通用，所有 BOSS 都用）
//   - 再叠加当前 BOSS 的专属技能（危险技能按BOSS[BOSS名]）
//   - 两者合并成一张危险图
func 危险颜色范围(BOSS名 string, 当前血量 int) []BOSS技能 {
	按BOSS := 危险技能按BOSS[BOSS名]
	// 没读到该 BOSS 的专属配置（BOSS 名空 / 没配）→ 用"危险技能默认"兜底
	if BOSS名 == "" || len(按BOSS) == 0 {
		var 范围 []BOSS技能
		for _, sk := range 危险技能默认 {
			if sk.触发血量 == 0 || sk.触发血量 == 当前血量 {
				范围 = append(范围, sk)
			}
		}
		return 范围
	}
	// 读到了专属配置 → 只用按 BOSS 配的技能（默认不参与）；多个技能组合在一起
	var 范围 []BOSS技能
	for _, sk := range 按BOSS {
		// 触发血量：0 = 一直检测；非 0 = 只有当前血量等于它才检测
		if sk.触发血量 == 0 || sk.触发血量 == 当前血量 {
			范围 = append(范围, sk)
		}
	}
	return 范围
}

// 当前BOSS名 返回当前 BOSS 名字（用于取 危险技能按BOSS 的配置）；
// 按 BOSS名单 逐个识别，命中哪个就是哪个；都没命中 → 空串（用 危险技能默认）
func 当前BOSS名() string {
	for _, n := range BOSS名单 {
		r := core.Exec(n)
		if r.Found {
			core.LogWithTimef("[躲避技能] 检测到 BOSS名【%s】(%d,%d) sim=%.2f", n, r.X, r.Y, r.Sim)
			return n
		}
		core.LogWithTimef("[躲避技能] BOSS名【%s】未命中", n)
	}
	core.LogWithTimef("[躲避技能] 未检测到任何 BOSS名 → 用默认技能范围")
	return ""
}

// 小地图可走方向 检测小地图，返回所有"可走方向"的单位向量列表（按连续白从长到短排序）：
//  1. 截小地图 → 双阈值二值化（白=可走）；
//  2. InRange 提取箭头掩膜 → 反相 → 与二值图相与，把箭头从可走图里抹掉；
//  3. 以区域中心（人物）为原点，向 小地图采样方向数 个方向发射射线，统计
//     "跳过开头黑（中心/箭头遮挡）后的连续白色长度"；
//  4. 把连续白 >= 小地图可走最小 的方向都收进列表；一个都没有 → ok=false。
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

	// 先算每个方向的"连续白长度"
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
				break // 白段结束
			}
		}
		长度表[k] = 长度
	}

	// 收集所有"可走"方向（连续白 >= 阈值），按长度从大到小排序
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
	// 按长度降序（简单选择排序，方向数很少）
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

// 是危险 判断"当前是不是危险"：
//   - 存在一个矩形：宽(包围盒) >= 危险最小边长 或 高(包围盒) >= 危险最小边长
//   - 且该矩形与 危险判定区域（AABB）有重叠（1 像素也算）
func 是危险(矩形列表 [][]imageHandler.Point) bool {
	for _, 矩形 := range 矩形列表 {
		左, 上, 右, 下, ok := 矩形包围盒(矩形)
		if !ok {
			continue
		}
		if 右-左 < 危险最小边长 && 下-上 < 危险最小边长 {
			continue // 宽和高都不到阈值
		}
		if 右 < 危险判定区域[0] || 左 > 危险判定区域[2] ||
			下 < 危险判定区域[1] || 上 > 危险判定区域[3] {
			continue // 与该区域无重叠
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

// 最远安全方向 从角色出发，向 方向数 条射线各探测：
//   - 每条射线沿路每 步长 像素取一个点，点落在任意最小矩形内 → 记为"碰到"，停止
//   - 一路到 最大距离 都没碰到 → 通行距离 = 最大距离（安全）
//
// 返回：通行距离最长的方向（单位向量）及其通行距离；全部被挡则返回 0,0,0
func 最远安全方向(矩形列表 [][]imageHandler.Point, 方向数, 步长, 最大距离 int) (dx, dy float64, 最远通行 int) {
	最远通行 = 0
	for k := 0; k < 方向数; k++ {
		角 := 2 * math.Pi * float64(k) / float64(方向数)
		ux, uy := math.Cos(角), math.Sin(角)
		通行 := 最大距离 // 默认走到底都没碰
		for 距 := 步长; 距 <= 最大距离; 距 += 步长 {
			x := 躲避角色X + int(ux*float64(距))
			y := 躲避角色Y + int(uy*float64(距))
			if x < 0 || y < 0 || x >= 1280 || y >= 720 {
				通行 = 距 - 步长 // 出界算到上一步
				break
			}
			if 点在任意矩形内(矩形列表, x, y) {
				通行 = 距 - 步长 // 碰到矩形 → 只能走到上一步
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

// 点在任意矩形内 判断点 (x,y) 是否落在矩形列表里的任意一个最小矩形内
func 点在任意矩形内(矩形列表 [][]imageHandler.Point, x, y int) bool {
	for _, 矩形 := range 矩形列表 {
		if 点在矩形内(矩形, x, y) {
			return true
		}
	}
	return false
}

// 点在矩形内 判断点 (x,y) 是否在一个（凸四边形）矩形内：
//   - 角点可能不按顺序返回，先按相对中心的极角排成顺时针
//   - 点在四条边的同一侧 → 命中
func 点在矩形内(角点 []imageHandler.Point, x, y int) bool {
	if len(角点) < 3 {
		return false
	}
	// 按相对重心的极角排序，得到顺序顶点
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
	// 点在凸多边形内：所有边叉积同号
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

// 画最小矩形 把矩形列表（每个多个角点）画成红线，返回新图（底图为输入图；调用方负责 Recycle 返回值）
func 画最小矩形(底图 *imageHandler.Mat, 矩形列表 [][]imageHandler.Point) *imageHandler.Mat {
	结果 := imageHandler.CopyTo(底图, nil)
	for _, 矩形 := range 矩形列表 {
		if len(矩形) < 2 {
			continue
		}
		for i := 0; i < len(矩形); i++ {
			a := 矩形[i]
			b := 矩形[(i+1)%len(矩形)]
			新图 := imageHandler.DrawRect(结果, a.X, a.Y, b.X, b.Y, "#FF0000", 2, 4)
			结果.Recycle()
			结果 = 新图
		}
	}
	return 结果
}
