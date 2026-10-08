package bagrecog

// ============================================================================
// 背包物品哈希识别（纯识别包，只读）
//
// 只做一件事：告诉调用方"当前背包里有哪些物品、分别在哪个坐标"。
//
//   按 bagRect 截屏 → 二值化 → 取轮廓 → 按尺寸区间筛出物品外接框
//     → 切图 + 涂鸦(盖掉数量/数字) → 统一缩放 → 算 pHash
//     → 与 assets/hashtable.txt 逐项算汉明距离 → 取最相似项作为该物品的名字
//
// 哈希表来源：读 assets/hashtable.txt（embed 打包进程序的那份，每行 "名字|hash"）。
// 表里写什么名字，识别就报什么名字（繁体原名，不做简繁转换）。
//
// 铁律（本包是"单纯识别"，不做别的）：
//   - 只读表，不写任何文件、不导出 png、不新建条目
//   - 不点击、不取文(OCR)、不改表、不关弹窗
//   - 识别不到就是识别不到，返回空，绝不尝试"补录"
//
// 建表/命名那一套在 test/beibao_recognize.go（RunBeibaoRecognize），与本包无关。
//
// 关于导出：Go 里只有 Unicode 大写字母开头的标识符才导出，中文一律未导出，
// 所以内部实现保持中文命名不变，对外只提供一层英文壳：
//   Recognize() []Item               识别默认背包区域
//   RecognizeRegion(x1,y1,x2,y2)     识别指定全屏区域
//   RecognizeRegionSim(x1,y1,x2,y2,minSim) 识别指定全屏区域 + 指定相似度阈值
// ============================================================================

import (
	"fmt"
	"sort"
	"strings"

	"app/assets"
	"app/imageHandler"
)

// 默认背包区域（全屏绝对坐标）：[left, top, right, bottom]。
// Recognize() 用默认区域；需要识别其他区域(如贩售物品面板)用 RecognizeRegion 覆盖。
var bagRect = [4]int{627, 200, 1233, 592}

const (
	// 物品外接框筛选区间：宽高都落在 [itemMinWH, itemMaxWH] 才算一个物品
	//  - itemMinWH 下限：过滤数字角标、碎片等小轮廓（它们远小于物品）
	//  - itemMaxWH 上限：过滤相邻物品粘连出来大框（如两个90宽物品连一起≈185）
	// 若筛选出的框数量不对，先调这两个值
	itemMinWH = 85
	itemMaxWH = 115

	// 识别命中的最低相似度：全表比完后最相似的那条 < itemMinSim 就认不出
	// （表里没收录/长得像但不够像的新道具），直接丢弃不报名字，防止误点。
	// 64 bit 感知哈希下 0.9 ≈ 汉明距离 6 位以内才算同一件物品。
	itemMinSim = 0.9

	cropPad      = 0  // 切图外扩像素，防止物品边缘贴框导致哈希抖动
	uniformSize  = 96 // 统一缩放边长，消除外接框每次切出来大小不一致的抖动
	pHashDctSize = 32
	pHashROISize = 8

	// 涂鸦区域：物品切图先涂鸦后缩放，涂鸦时是原尺寸
	// （外接框宽高约 itemMinWH~itemMaxWH=85~115，随物品大小浮动）。
	// 物品角标/角落的数字（数量、价格等）会随数值变化直接改写哈希，
	// 比对前先把数字区域涂黑盖掉，只留物品外观，数字怎么变都不抖。
	// 共两块（DrawRect 的 (x1,y1,x2,y2)，坐标相对切图左上角）：
	//   ① 左上角数量角标：x1=0,y1=0 起，横向盖满到 x2=96，高度盖到 y2=28，
	//      物品数量 99+ 的角标也基本都在这个高度内；
	//   ② 右下角数字区：切图宽高(w,h)会随外接框大小浮动（85~115），
	//      起点若写死，盖的高度就会随图高变化、有的盖不全。
	//      所以从右下角反向盖：x1=w-宽, y1=h-高，终点 x2=w, y2=h，
	//      不管切图多大，右下角都固定涂掉 graffitiRightW×graffitiRightH 一块。
	// 若发现数量/数字变化仍导致哈希抖动，多半是这两块没盖全，优先调这里。
	graffitiLeftX1 = 0  // 左上角涂鸦区：起点 X（图最左）
	graffitiLeftY1 = 0  // 左上角涂鸦区：起点 Y（图最上）
	graffitiLeftX2 = 96 // 左上角涂鸦区：终点 X（横向盖满整行）
	graffitiLeftY2 = 28 // 左上角涂鸦区：终点 Y（数量角标高度）
	graffitiRightW = 45 // 右下角涂鸦区：从图右下角往里盖的宽度
	graffitiRightH = 29 // 右下角涂鸦区：从图右下角往里盖的高度
)

// Item 识别到的一个物品（对外返回的识别结果）
type Item struct {
	Name string  // 物品名：hashtable.txt 里那一行的名字（你改过名就是名字，没改过就是编号）
	// 物品外接框的完整全屏坐标（覆盖整个物品格子，格内右上角等角标也在框内）：
	X1   int // 格子 左上角 的全屏 X
	Y1   int // 格子 左上角 的全屏 Y
	X2   int // 格子 右下角 的全屏 X
	Y2   int // 格子 右下角 的全屏 Y
	X    int // 格子中心的全屏 X 坐标（点击这个位置即可选中该物品）
	Y    int // 格子中心的全屏 Y 坐标
	Sim  float64 // 与表里该条的相似度（1 - 汉明距离/64），越高越可信
}

// rectW / rectH 直接计算，避免依赖 Rect.GetHeight() 的已知笔误(Bottom-Right)
func rectW(r imageHandler.Rect) int { return r.Right - r.Left }
func rectH(r imageHandler.Rect) int { return r.Bottom - r.Top }

// 是物品框：宽高都落在筛选区间
func 是物品框(r imageHandler.Rect) bool {
	return rectW(r) >= itemMinWH && rectW(r) <= itemMaxWH &&
		rectH(r) >= itemMinWH && rectH(r) <= itemMaxWH
}

// 排序框：按 上→下、左→右 排序，保证顺序稳定
func 排序框(rects []imageHandler.Rect) {
	sort.Slice(rects, func(i, j int) bool {
		if rects[i].Top != rects[j].Top {
			return rects[i].Top < rects[j].Top
		}
		return rects[i].Left < rects[j].Left
	})
}

// 筛物品框：从所有轮廓里挑出物品外接框并排序
func 筛物品框(rects []imageHandler.Rect) []imageHandler.Rect {
	items := make([]imageHandler.Rect, 0, len(rects))
	for _, r := range rects {
		if 是物品框(r) {
			items = append(items, r)
		}
	}
	排序框(items)
	return items
}

// 切物品：按外接框外扩 cropPad 再切（防越界），返回原尺寸切图，用完要 Recycle
func 切物品(背包区域 *imageHandler.Mat, r imageHandler.Rect) *imageHandler.Mat {
	sx := r.Left - cropPad
	if sx < 0 {
		sx = 0
	}
	sy := r.Top - cropPad
	if sy < 0 {
		sy = 0
	}
	ex := r.Right + cropPad
	if ex > 背包区域.GetWidth() {
		ex = 背包区域.GetWidth()
	}
	ey := r.Bottom + cropPad
	if ey > 背包区域.GetHeight() {
		ey = 背包区域.GetHeight()
	}
	return imageHandler.CropMat(背包区域, sx, sy, ex, ey)
}

// 涂鸦切图：盖掉切图左上角数量角标和右下角数字区，
// 避免数字随数量/价格变化干扰哈希。返回新 Mat（原图未回收，由调用方负责）
func 涂鸦切图(mat *imageHandler.Mat) *imageHandler.Mat {
	w, h := mat.GetWidth(), mat.GetHeight()
	// ① 盖左上角数量角标：区域见常量注释，横向盖满整行
	m1 := imageHandler.DrawRect(mat, graffitiLeftX1, graffitiLeftY1, graffitiLeftX2, graffitiLeftY2, "#000000", -1, 4)
	// ② 盖右下角数字区：从右下角反向盖固定宽高，不随切图尺寸浮动
	m2 := imageHandler.DrawRect(m1, w-graffitiRightW, h-graffitiRightH, w, h, "#000000", -1, 4)
	m1.Recycle()
	return m2
}

// 截屏识别区域（按传入的全屏区域截屏）
func 截屏识别区域(区域 [4]int) (*imageHandler.Mat, error) {
	背包区域 := imageHandler.CaptureScreenMat(区域[0], 区域[1], 区域[2], 区域[3])
	if 背包区域.Empty() {
		return nil, fmt.Errorf("截屏识别区域(%v)失败", 区域)
	}
	return 背包区域, nil
}

// 读哈希表：读 assets/hashtable.txt（打包进程序的哈希表），解析每行 "名字|hash"，
// 返回名字表与哈希表的两个平行切片。只读，不改动文件。
// 表不存在/为空都返回空切片（调用方按"表里没东西"处理）。
func 读哈希表() ([]string, []string, error) {
	data, err := assets.HashTable.ReadFile("hashtable.txt")
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0)
	hashCode := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		names = append(names, parts[0])
		hashCode = append(hashCode, parts[1])
	}
	return names, hashCode, nil
}

// 识别当前区域：把屏幕上的每个物品认出来，返回 名字 + 格子中心坐标 + 相似度。
// 区域 是全屏坐标 [left, top, right, bottom]，物品坐标由它偏移换算。
// minSim 是本次识别的命中阈值：最相似度 < minSim 就丢弃不报名字（默认 itemMinSim=0.9）。
// 内部就是完整的哈希识别链路（截屏→二值化→轮廓→筛选→切图→涂鸦→缩放→pHash→全表比对），
// 空格子没轮廓就不会出现在结果里，天然只报"有物品的格子"。
// 物品外接框筛选区间(85~115)按背包格子尺寸定的，识别尺寸不同的区域要另调 itemMinWH/itemMaxWH。
//
// 为什么 minSim 要能传参：不同面板渲染出来的物品图不完全一样(如交易行上架面板 763,187,1256,579
// 里的物品比背包整体小 1~2px，缩放归一化后哈希还是有微小差异)，背包用 0.9 够严，
// 这类"缩水"面板就得放宽到 0.89 才认得出 —— 阈值随区域走，不写死在常量上。
//
// 返回空切片的几种情况（都是正常情况，不是错误）：
//   - 哈希表不存在 / 为空
//   - 区域里一个物品框都没筛出来
//
// 调用方按"这次没认出东西"处理即可。
func 识别区域(区域 [4]int, minSim float64) []Item {
	names, hashCode, err := 读哈希表()
	if err != nil || len(hashCode) == 0 {
		fmt.Printf("[识别] 哈希表不可用(表为空或读不到) → 本次不识别\n")
		return nil
	}

	背包区域, err := 截屏识别区域(区域)
	if err != nil {
		fmt.Printf("[识别] %v\n", err)
		return nil
	}
	defer 背包区域.Recycle()

	二值化 := imageHandler.DoubleBinaryzation(背包区域, 30, 150, 255)
	defer 二值化.Recycle()

	rects := imageHandler.GetContoursRect(二值化, 0, 2, nil)
	items := 筛物品框(rects)
	if len(items) == 0 {
		fmt.Printf("[识别] 轮廓=%d 筛出物品框=0（区间 %d~%d）→ 背包里没有可识别的物品\n",
			len(rects), itemMinWH, itemMaxWH)
		return nil
	}
	fmt.Printf("[识别] 轮廓=%d 筛出物品框=%d 个\n", len(rects), len(items))

	out := make([]Item, 0, len(items))
	for _, r := range items {
		cut := 切物品(背包区域, r)
		涂好 := 涂鸦切图(cut)
		cut.Recycle()
		scaled := imageHandler.ScaleMatByWH(涂好, uniformSize, uniformSize)
		涂好.Recycle()

		hashValue := imageHandler.PHashCode(scaled, pHashDctSize, pHashROISize)
		scaled.Recycle()

		// 与全表比对，取最相似的那一条作为这个物品的名字
		maxValue := 0.0
		index := 0
		for i := 0; i < len(hashCode); i++ {
			dist := imageHandler.HanMingDistance(hashValue, hashCode[i])
			sim := 1.0 - float64(dist)/64.0
			if sim > maxValue {
				maxValue = sim
				index = i
			}
		}
		// 全表最相似都不够 ≥minSim → 认不出（表里没收录/新道具），丢弃不报
		if maxValue < minSim {
			fmt.Printf("[识别] 最相似仅 %.4f < %.4f → 认不出(表里没有/相似度过低), 丢弃\n",
				maxValue, minSim)
			continue
		}
		out = append(out, Item{
			Name: names[index],
			// 完整格子外接框（全屏坐标 = 识别区域偏移 + 轮廓框）
			X1: r.Left + 区域[0],
			Y1: r.Top + 区域[1],
			X2: r.Right + 区域[0],
			Y2: r.Bottom + 区域[1],
			// 格子中心点：外接框左上角 + 区域偏移 + 半个框
			X:   r.Left + 区域[0] + rectW(r)/2,
			Y:   r.Top + 区域[1] + rectH(r)/2,
			Sim: maxValue,
		})
		fmt.Printf("[识别] 目标为%s 相似度:%.4f 全屏框(%d,%d)-(%d,%d) 中心(%d,%d)\n",
			names[index], maxValue,
			r.Left+区域[0], r.Top+区域[1], r.Right+区域[0], r.Bottom+区域[1],
			r.Left+区域[0]+rectW(r)/2, r.Top+区域[1]+rectH(r)/2)
	}
	return out
}

// Recognize 对外入口：识别默认背包区域(bagRect)的全部物品。
// RecognizeRegion 同 Recognize，但识别指定全屏区域(如贩售物品面板 763,187,1256,579)。
// RecognizeRegionSim 同 RecognizeRegion，但本次识别用调用方给的相似度阈值 minSim
// （覆盖默认 itemMinSim=0.9）。
//
// 纯识别、只读 —— 不写文件、不新增哈希条目、不点击、不取文、不关弹窗。
// 返回的坐标(中心 X,Y 与外接框 X1..Y2)是该区域内的全屏绝对坐标，可直接用于点击选中该物品。
// 只有最相似度 ≥ 阈值的物品才会返回（认不出的格子不报，防误点），
// 返回的 Item.Sim 就是那条最相似度，调用方可按需再用它过滤。
func Recognize() []Item {
	return 识别区域(bagRect, itemMinSim)
}

// RecognizeRegion 识别指定全屏区域里的物品(如贩售/上架面板 763,187,1256,579)。
// 区域坐标 = [left, top, right, bottom]；物品框尺寸与背包一致时可直接复用哈希表。
// 阈值用默认 itemMinSim(0.9)；该区域物品与背包有渲染差异(整体小 1~2px)时用
// RecognizeRegionSim 传更低阈值。
func RecognizeRegion(x1, y1, x2, y2 int) []Item {
	return 识别区域([4]int{x1, y1, x2, y2}, itemMinSim)
}

// RecognizeRegionSim 识别指定全屏区域里的物品，并指定本次的相似度阈值 minSim。
// 用于物品渲染与背包有差异、默认 0.9 卡得太严认不出的区域
// （如交易行上架面板 763,187,1256,579，物品比背包小 1~2px，用 0.89）。
func RecognizeRegionSim(x1, y1, x2, y2 int, minSim float64) []Item {
	return 识别区域([4]int{x1, y1, x2, y2}, minSim)
}
