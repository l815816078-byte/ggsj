package beibao

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"app/core"
	"app/imageHandler"
)

// ============================================================================
// 背包物品识别（轮廓直切版，go run beibao_recognize.go）
//
// 不再用"锚点+网格"切割，直接切外接矩形：
//
//   实时截屏 → 裁剪背包区域 → 二值化 → 取所有轮廓(GetContoursRect)
//     → 按尺寸区间 [itemMinWH, itemMaxWH] 筛出"物品外接框"
//       (过滤掉数字角标/碎片这类小框，也过滤掉相邻物品粘连出来的大框)
//     → 外扩 cropPad 像素切图 → 统一缩放 uniformSize
//     → 算 pHash → 写 hashtable.txt + 导出 png
//
//   识别(识别物品)：同样筛选切图 → 算 pHash → 与哈希表逐项算汉明距离 →
//     取最相似项作为识别结果，坐标直接用外接框 + bagRect 偏移，比网格更准
//
// 注意：
//   - 识别时每个物品与整张表全表比对（不是按位置对齐），空格子没轮廓
//     就不会输出，天然只报"有物品的格子"
//   - hashtable.txt 每行 "编号|hash"，编号仅供你人工看图命名
//     （把编号改成物品名也 OK，如 "金疮药|xxxx"）
//   - pHash 字符串长度 64，汉明距离归一化相似度 = 1 - dist/64
//   - 哈希表的制作 是增量式：先清洗表内重复（哈希几乎一样的同种物品
//     只留一条，你改过名字的行优先保留），再对截图新物品追加，
//     编号接着已有最大编号递增，旧行原样保留不清空
// ============================================================================

// 背包区域（相对全屏）：[left, top, right, bottom]
var bagRect = [4]int{627, 200, 1233, 592}

// 物品名称取文区域（全屏绝对坐标）：点击背包里的物品后，名称就显示在这块区域，
// 用飞浆OCR(PaddleOCR)把里面的文字原样读出来，直接作为该物品在 hashtable.txt 里的名字
var nameRect = [4]int{503, 103, 783, 150}

const (
	// 取文相似度阈值：低于该值的文字块丢弃（与 core 取文预设的默认值一致）
	nameOcrThresh = 0.5

	// 改名最多重试次数：写回 hashtable.txt 后要读回校验，没校验通过就重试；
	// 重试到次数用尽仍失败 → 停止整个命名流程，不再点下一个物品
	renameMaxRetry = 3
)

const (
	// 物品外接框筛选区间：宽高都落在 [itemMinWH, itemMaxWH] 才算一个物品
	//  - itemMinWH 下限：过滤数字角标、碎片等小轮廓（它们远小于物品）
	//  - itemMaxWH 上限：过滤相邻物品粘连出来大框（如两个90宽物品连一起≈185）
	// 若筛选出的框数量不对，先调这两个值，再看 4_筛选物品.png 验证
	itemMinWH = 85
	itemMaxWH = 115

	cropPad      = 0  // 切图外扩像素，防止物品边缘贴框导致哈希抖动
	uniformSize  = 96 // 统一缩放边长，消除外接框每次切出来大小不一致的抖动
	pHashDctSize = 32
	pHashROISize = 8

	// 增量制作判定"同一物品"的相似度阈值（判定用大于等于 >= 该值）：
	//   新截图哈希与表内已有条目相似度 >= 该值 → 视为已入库，跳过不重复加
	//   当前 >=0.9：64 位哈希最多差 6 位算同一物品
	//     (dist=6 → sim=0.90625 >= 0.9 已入库；dist=7 → sim=0.890625 < 0.9 算新物品)
	//   调大(如 0.95)：更严格，稍有变化就当新物品重复入库
	//   调小(如 0.85)：更宽松，可能把两个长得像的不同物品当同一个漏加
	sameThresh = 0.89

	// 涂鸦区域：物品切图先涂鸦后缩放，涂鸦时是原尺寸
	// （外接框宽高约 itemMinWH~itemMaxWH=85~115，随物品大小浮动）。
	// 物品角标/角落的数字（数量、价格等）会随数值变化直接改写哈希，
	// 入库或比对前先把数字区域涂黑盖掉，只留物品外观，数字怎么变都不抖。
	// 共两块（DrawRect 的 (x1,y1,x2,y2)，坐标相对切图左上角）：
	//   ① 左上角数量角标：x1=0,y1=0 起，横向盖满到 x2=95，高度盖到 y2=28，
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

	// 导出目录：手机里的 autogo_lightpillar 文件夹
	outDir = "/sdcard/autogo_lightpillar/"
)

// 新条目：本次新增入库的一个物品。
// ID 用于回头把 hashtable.txt 里"编号|hash"这一行的编号改成物品名；
// X/Y 是该物品格子中心的全屏绝对坐标，点它就能弹出物品详情（名称就在 nameRect 区域）。
type 新条目 struct {
	ID int
	X  int
	Y  int
}

// rectW / rectH 直接计算，避免依赖 Rect.GetHeight() 的已知笔误(Bottom-Right)
func rectW(r imageHandler.Rect) int { return r.Right - r.Left }
func rectH(r imageHandler.Rect) int { return r.Bottom - r.Top }

// 是物品框：宽高都落在筛选区间
func 是物品框(r imageHandler.Rect) bool {
	return rectW(r) >= itemMinWH && rectW(r) <= itemMaxWH &&
		rectH(r) >= itemMinWH && rectH(r) <= itemMaxWH
}

// 排序框：按 上→下、左→右 排序，保证编号顺序稳定（建表和识别走同一套）
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

// 清洗重复表：把表内哈希几乎一样（相似度 ≥ sameThresh）的【编号行】合并成一条，
// 去掉早期全量刷新/重复入库攒下的冗余编号条目。
//
// 铁律：你改过名字的行（非纯数字名）一条都不删 —— 哪怕同一种物品你手动
// 命名了两条（哈希几乎一样），两条都原样保留，绝不因"重复"吃掉你的命名。
// 只有纯数字编号行会被去重：一组相似里留最靠前的一条，其余删（png 一并删）。
// 因此本函数只会删编号行，delIDs 收集的就是这些被删编号，写回时旧行原样保留。
func 清洗重复表(rows, hashes []string) ([]string, []string, []int) {
	已命名 := make([]bool, len(rows))
	pngID := make([]int, len(rows))
	del := make([]bool, len(rows))
	for i, line := range rows {
		name := strings.SplitN(line, "|", 2)[0]
		id, err := strconv.Atoi(strings.TrimSpace(name))
		if err != nil {
			已命名[i] = true
			pngID[i] = -1
		} else {
			pngID[i] = id
		}
	}
	for i := 0; i < len(rows); i++ {
		if del[i] {
			continue
		}
		for j := i + 1; j < len(rows); j++ {
			if del[j] {
				continue
			}
			dist := imageHandler.HanMingDistance(hashes[i], hashes[j])
			if 1.0-float64(dist)/64.0 < sameThresh {
				continue
			}
			// 几乎一样：
			//   - 双命名行 → 都保留（宁冗余不删用户命名）
			//   - 编号行 vs 命名行 → 删编号行
			//   - 双编号行 → 删靠后的编号行
			if 已命名[i] {
				if !已命名[j] {
					del[j] = true
				}
			} else if 已命名[j] {
				del[i] = true
			} else {
				del[j] = true
			}
		}
	}
	keptRows := make([]string, 0, len(rows))
	keptHashes := make([]string, 0, len(rows))
	delIDs := make([]int, 0)
	for i := range rows {
		if del[i] {
			if pngID[i] >= 0 {
				delIDs = append(delIDs, pngID[i])
			}
			continue
		}
		keptRows = append(keptRows, rows[i])
		keptHashes = append(keptHashes, hashes[i])
	}
	return keptRows, keptHashes, delIDs
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
	// ② 盖右下角数字区：从右下角反向盖固定 45x27，不随切图尺寸浮动
	m2 := imageHandler.DrawRect(m1, w-graffitiRightW, h-graffitiRightH, w, h, "#000000", -1, 4)
	m1.Recycle()
	return m2
}

// dbgSave 调试辅助：把中间过程图存到 outDir，方便肉眼检查每步筛得对不对
func dbgSave(mat *imageHandler.Mat, name string) {
	p := filepath.Join(outDir, name)
	if imageHandler.SaveMat(mat, p, "png", 100) {
		fmt.Printf("[调试] 已保存 %s\n", p)
	} else {
		fmt.Printf("[调试] 保存失败 %s\n", p)
	}
}

// 截屏背包区域（直接按 bagRect 区域截屏）
func 截屏背包区域() (*imageHandler.Mat, error) {
	背包区域 := imageHandler.CaptureScreenMat(bagRect[0], bagRect[1], bagRect[2], bagRect[3])
	if 背包区域.Empty() {
		return nil, fmt.Errorf("截屏背包区域失败")
	}
	return 背包区域, nil
}

// 哈希表的制作：增量制作，不覆盖旧表，先清洗表内重复再追加新物品。
// 1) 清洗：表内哈希几乎一样（同种物品多份拷贝/重复入库）的只留一条；
// 2) 追加：截图中与表内已有条目相似度 ≥ sameThresh 的视为已入库直接跳过；
// 3) 新条目编号接着已有最大编号递增，你改过的名字行原样保留。
// 返回本次新增的条目（含编号与格子中心坐标），交给命名新物品逐个取名字。
func 哈希表的制作() ([]新条目, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	txtPath := filepath.Join(outDir, "hashtable.txt")

	// 读已有表（可能不存在）
	oldRows := make([]string, 0) // 已有行原文
	oldHash := make([]string, 0) // 已有哈希，用于相似度比对
	if data, err := os.ReadFile(txtPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 2)
			if len(parts) != 2 {
				continue
			}
			oldRows = append(oldRows, line)
			oldHash = append(oldHash, parts[1])
		}
	}
	fmt.Printf("[哈希表] 读入条目 %d 条\n", len(oldRows))

	// 清洗表内重复：几乎一样的同种物品只留一条，删掉多余 png
	cleaned := false
	if len(oldRows) > 1 {
		newRows, newHashes, delIDs := 清洗重复表(oldRows, oldHash)
		if len(delIDs) > 0 {
			for _, id := range delIDs {
				os.Remove(filepath.Join(outDir, fmt.Sprintf("%d.png", id)))
			}
			fmt.Printf("[哈希表] 清理表内重复 %d 条（同种物品只留一条）编号: %v\n",
				len(delIDs), delIDs)
			oldRows, oldHash = newRows, newHashes
			cleaned = true
		}
	}

	// 新编号接着已有最大编号递增（清洗可能留下编号空洞，不能用条数当编号）。
	// maxID 从两处取最大：
	//   1) 表里纯数字编号行 —— 只有它们能直接读出编号；
	//   2) 目录里 *.png 的文件名数字 —— 你改名的行当初也存过"编号.png"，
	//      所以即使整张表全改成名字，编号照样接着走，绝不会从 0 重新开始
	//      （从 0 重来会让人以为表被重置、名字被刷没了）
	maxID := -1
	for _, line := range oldRows {
		name := strings.SplitN(line, "|", 2)[0]
		if id, err := strconv.Atoi(strings.TrimSpace(name)); err == nil && id > maxID {
			maxID = id
		}
	}
	if ents, err := os.ReadDir(outDir); err == nil {
		for _, ent := range ents {
			name := ent.Name()
			if !strings.HasSuffix(name, ".png") {
				continue
			}
			if id, err := strconv.Atoi(strings.TrimSuffix(name, ".png")); err == nil && id > maxID {
				maxID = id
			}
		}
	}
	fmt.Printf("[哈希表] 已有条目 %d 条\n", len(oldRows))

	背包区域, err := 截屏背包区域()
	if err != nil {
		return nil, err
	}
	defer 背包区域.Recycle()
	dbgSave(背包区域, "1_背包区域原图.png")

	二值化 := imageHandler.DoubleBinaryzation(背包区域, 30, 150, 255)
	defer 二值化.Recycle()
	dbgSave(二值化, "2_二值化.png")

	rects := imageHandler.GetContoursRect(二值化, 0, 2, nil)
	fmt.Printf("[调试] 轮廓数量=%d\n", len(rects))
	if len(rects) > 0 {
		轮廓图 := imageHandler.DrawRects(背包区域, rects, "#FF0000", 1, 4)
		dbgSave(轮廓图, "3_所有轮廓.png")
		轮廓图.Recycle()
	}

	items := 筛物品框(rects)
	fmt.Printf("[调试] 筛出物品框=%d 个（区间 %d~%d）\n", len(items), itemMinWH, itemMaxWH)
	if len(items) == 0 {
		return nil, fmt.Errorf("筛选后没有物品框，请调 itemMinWH/itemMaxWH 或检查二值化参数")
	}
	if len(items) > 0 {
		筛选图 := imageHandler.DrawRects(背包区域, items, "#00FF00", 2, 4)
		dbgSave(筛选图, "4_筛选物品.png")
		筛选图.Recycle()
	}

	// 是否已在表内：与已有条目（含本次刚新增的）逐项算相似度取最高值，
	// 每个物品都把最高相似度打出来，便于人工确认阈值定多少合适
	已比对 := append([]string{}, oldHash...)

	var sb strings.Builder
	add := 0
	id := maxID + 1                  // 新编号接着已有最大编号递增
	新增 := make([]新条目, 0, len(items)) // 本次新增的条目，全部入库后逐一点开取名字
	for _, r := range items {
		cut := 切物品(背包区域, r)
		涂好 := 涂鸦切图(cut)
		cut.Recycle()
		scaled := imageHandler.ScaleMatByWH(涂好, uniformSize, uniformSize)
		涂好.Recycle()

		hash := imageHandler.PHashCode(scaled, pHashDctSize, pHashROISize)

		// 与表内每条比一次，取最高相似度
		maxSim := -1.0
		for _, oh := range 已比对 {
			sim := 1.0 - float64(imageHandler.HanMingDistance(hash, oh))/64.0
			if sim > maxSim {
				maxSim = sim
			}
		}
		if maxSim >= sameThresh {
			fmt.Printf("[哈希表] 坐标(%d,%d) 最高相似度%.4f >= %.2f → 已在表内，跳过\n",
				r.Left+bagRect[0], r.Top+bagRect[1], maxSim, sameThresh)
			scaled.Recycle()
			continue
		}
		已比对 = append(已比对, hash) // 同一张图里重复出现的同种物品只入库一次

		sb.WriteString(fmt.Sprintf("%d|%s\n", id, hash))
		pngPath := filepath.Join(outDir, fmt.Sprintf("%d.png", id))
		imageHandler.SaveMat(scaled, pngPath, "png", 100)
		fmt.Printf("[哈希表] 新增模板%d 尺寸%dx%d 全屏坐标(%d,%d) 最高相似度%.4f < %.2f → %s\n",
			id, rectW(r), rectH(r), r.Left+bagRect[0], r.Top+bagRect[1], maxSim, sameThresh, pngPath)
		// 记下格子中心点（后面点这里弹出物品详情取名字）
		新增 = append(新增, 新条目{
			ID: id,
			X:  r.Left + bagRect[0] + rectW(r)/2,
			Y:  r.Top + bagRect[1] + rectH(r)/2,
		})
		scaled.Recycle()
		id++
		add++
	}

	// 只有清理过重复、或新增了新物品，才需要写回
	if add == 0 && !cleaned {
		fmt.Printf("[哈希表] 没有发现新物品，表不变 → %s\n", txtPath)
		return nil, nil
	}

	// 清洗后的旧行原样保留 + 新行追加，整体写回（不清掉已命名的名字）
	var all strings.Builder
	for _, line := range oldRows {
		all.WriteString(line)
		all.WriteString("\n")
	}
	all.WriteString(sb.String())
	if err := os.WriteFile(txtPath, []byte(all.String()), 0o644); err != nil {
		return nil, err
	}
	if add == 0 {
		fmt.Printf("[哈希表] 仅清理重复，共 %d 条 → %s\n", len(oldRows), txtPath)
	} else {
		fmt.Printf("[哈希表] 本次新增 %d 条，共 %d 条 → %s\n", add, len(oldRows)+add, txtPath)
	}
	return 新增, nil
}

// 命名新物品：把本次新增入库的每个物品点开，取到名称后写回 hashtable.txt。
//
// 逐个循环处理（处理完所有新增即结束），每个物品的步骤：
//
//	① 点击该物品格子中心 → 弹出物品详情
//	② 对 nameRect 区域飞浆OCR取文 → 读出来的文字就是物品名（直接就是繁体，不做转换）
//	③ 字库找"关闭"按钮点掉详情弹窗
//	④ 把 hashtable.txt 里"编号|hash"这一行的编号换成物品名
//
// 取不到名字（OCR空）就跳过改名，但弹窗照样关，保证能继续处理下一个。
func 命名新物品(news []新条目) {
	if len(news) == 0 {
		return
	}
	// 必须先加载字库：core.Exec("关闭") 走字库找字，不加载会访问空字库
	if !imageHandler.SetDict("ziku.txt", "library") {
		fmt.Printf("[命名] 字库加载失败，无法找\"关闭\"按钮 → 放弃本次命名\n")
		return
	}
	txtPath := filepath.Join(outDir, "hashtable.txt")
	for i, it := range news {
		fmt.Printf("[命名] %d/%d: 点击物品%d (%d,%d)\n", i+1, len(news), it.ID, it.X, it.Y)
		// ① 点开物品（用默认延时，够弹窗出来）
		core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)

		// ② 取物品名称
		r := core.OcrPaddleText(nameRect[0], nameRect[1], nameRect[2], nameRect[3], nameOcrThresh)
		name := ""
		if r.Found {
			name = 清理名字(r.Word)
		}
		if name == "" {
			fmt.Printf("[命名] %d/%d: 物品%d 取文失败(原文=%q) → 保留编号%d\n",
				i+1, len(news), it.ID, r.Word, it.ID)
		}

		// ③ 关掉详情弹窗
		关闭弹窗()

		// ④ 改名：写回并读回校验，写不成功就重试，不能带着没写进去的状态往下走
		if name == "" {
			continue
		}
		ok := false
		for try := 1; try <= renameMaxRetry; try++ {
			if err := 改条目名(txtPath, it.ID, name); err != nil {
				fmt.Printf("[命名] %d/%d: 物品%d 第%d/%d次改名失败: %v\n",
					i+1, len(news), it.ID, try, renameMaxRetry, err)
				core.RandomSleep(500, 1000)
				continue
			}
			ok = true
			break
		}
		if !ok {
			// 表里还是编号，但这个物品已经点过、弹窗也关了，
			// 继续点下一个只会让"点过的"和"写进去的"对不上 → 直接停止命名
			fmt.Printf("[命名] %d/%d: 物品%d 改名最终失败 → 停止命名，剩余 %d 个不再处理\n",
				i+1, len(news), it.ID, len(news)-i-1)
			return
		}
		fmt.Printf("[命名] %d/%d: 物品%d → %s（已写入并读回校验通过）\n", i+1, len(news), it.ID, name)
	}
}

// 清理名字：OCR 多个文字块是用空格拼起来的，物品名里不该有空白，
// 这里把所有空白字符去掉，只留文字本身（繁体字原样保留，不做简繁转换）
func 清理名字(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

// 关闭弹窗：只点一次 —— 字库找"关闭"按钮，找到就点；
// 没找到说明弹窗已经关掉了，直接返回，不重复找、不重复点。
func 关闭弹窗() {
	c := core.Exec("关闭")
	if !c.Found {
		fmt.Printf("[命名] 未找到\"关闭\" → 弹窗已关，跳过\n")
		return
	}
	fmt.Printf("[命名] 点击\"关闭\" (%d,%d)\n", c.X, c.Y)
	core.RandomClickInAreaWithDelay(c.X, c.Y, c.X+5, c.Y+5)
}

// 改条目名：把 hashtable.txt 里"编号|hash"这一行的编号换成物品名，
// 只改编号严格等于 id 的那一行（哈希部分原样不动，已命名的名字行不受影响）。
//
// 写下之后立刻读回来做同步校验，三项都过才算真的写成功：
//
//	① 表里能找到"名字|原哈希"这一行；
//	② 旧编号行（"编号|..."）已经没了；
//	③ 总行数没变（没写丢、也没多写）。
//
// 任何一项不过都返回错误，交给调用方重试 —— 绝不出现"日志说已写入、文件其实没改"。
func 改条目名(txtPath string, id int, name string) error {
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return err
	}
	oldName := strconv.Itoa(id)
	oldHash := ""
	var sb strings.Builder
	lines := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && parts[0] == oldName {
			oldHash = parts[1]
			line = name + "|" + parts[1]
		}
		sb.WriteString(line)
		sb.WriteString("\n")
		lines++
	}
	if oldHash == "" {
		return fmt.Errorf("表里没有编号为 %s 的行", oldName)
	}
	if err := os.WriteFile(txtPath, []byte(sb.String()), 0o644); err != nil {
		return err
	}

	// 同步校验：写完之后重新读文件，确认改动的这一行确实变成"名字|哈希"
	back, err := os.ReadFile(txtPath)
	if err != nil {
		return err
	}
	want := name + "|" + oldHash
	got := 0
	cnt := 0
	for _, line := range strings.Split(string(back), "\n") {
		if line == "" {
			continue
		}
		cnt++
		switch {
		case line == want:
			got++
		case strings.HasPrefix(line, oldName+"|"):
			return fmt.Errorf("写入校验失败: 编号 %s 的行还在，文件没改成", oldName)
		}
	}
	if got == 0 {
		return fmt.Errorf("写入校验失败: 文件里找不到 %q", want)
	}
	if cnt != lines {
		return fmt.Errorf("写入校验失败: 行数 %d != 期望 %d", cnt, lines)
	}
	return nil
}

// 识别物品：加载哈希表，对当前背包截图每个物品轮廓比对，打印识别结果
func 识别物品() error {
	txtPath := filepath.Join(outDir, "hashtable.txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return fmt.Errorf("读取哈希表失败: %v（请先运行 哈希表的制作）", err)
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
	if len(hashCode) == 0 {
		return fmt.Errorf("哈希表为空")
	}

	背包区域, err := 截屏背包区域()
	if err != nil {
		return err
	}
	defer 背包区域.Recycle()
	dbgSave(背包区域, "r1_背包区域原图.png")

	二值化 := imageHandler.DoubleBinaryzation(背包区域, 30, 150, 255)
	defer 二值化.Recycle()
	dbgSave(二值化, "r2_二值化.png")

	rects := imageHandler.GetContoursRect(二值化, 0, 2, nil)
	fmt.Printf("[调试] 识别：轮廓数量=%d\n", len(rects))
	if len(rects) > 0 {
		轮廓图 := imageHandler.DrawRects(背包区域, rects, "#FF0000", 1, 4)
		dbgSave(轮廓图, "r3_所有轮廓.png")
		轮廓图.Recycle()
	}

	items := 筛物品框(rects)
	fmt.Printf("[调试] 识别：筛出物品框=%d 个（区间 %d~%d）\n", len(items), itemMinWH, itemMaxWH)
	if len(items) == 0 {
		return fmt.Errorf("筛选后没有物品框，请调 itemMinWH/itemMaxWH 或检查二值化参数")
	}
	if len(items) > 0 {
		筛选图 := imageHandler.DrawRects(背包区域, items, "#00FF00", 2, 4)
		dbgSave(筛选图, "r4_筛选物品.png")
		筛选图.Recycle()
	}

	for _, r := range items {
		cut := 切物品(背包区域, r)
		涂好 := 涂鸦切图(cut)
		cut.Recycle()
		scaled := imageHandler.ScaleMatByWH(涂好, uniformSize, uniformSize)
		涂好.Recycle()

		hashValue := imageHandler.PHashCode(scaled, pHashDctSize, pHashROISize)
		scaled.Recycle()

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
		fmt.Printf("[识别] 目标为%s 相似度:%.4f 全屏坐标 x:%d, y:%d\n",
			names[index], maxValue, r.Left+bagRect[0], r.Top+bagRect[1])
	}
	return nil
}

// RunBeibaoRecognize 供 main.go 调用的入口。
// 每次运行都先做一次"增量制作"：截图里的新物品（哈希表里没有的）
// 自动入库、编号接着递增，旧表（含你改过的名字）原样保留；
// 然后把本次新增的物品逐个点开取名字写回表（新物品自动命名）；
// 最后全表识别（此时新物品已经带名字了）。
func RunBeibaoRecognize() {
	fmt.Println("=== 背包物品识别(轮廓直切) ===")
	news, err := 哈希表的制作()
	if err != nil {
		fmt.Printf("增量制作哈希表失败: %v\n", err)
		return
	}
	命名新物品(news)
	if err := 识别物品(); err != nil {
		fmt.Printf("识别物品失败: %v\n", err)
		return
	}
	fmt.Println("=== 完成 ===")
}
