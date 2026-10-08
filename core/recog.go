package core

import (
	"fmt"
	"sync/atomic"
	"time"
)

// RecogResult 识别执行结果
type RecogResult struct {
	X, Y    int // 命中点坐标（找字=文字左上角，其余=命中点）
	X2, Y2  int // 命中矩形右下角（字库找字多结果时有效，其余为 0）
	Sim     float64
	Word    string // OCR 识别出的文字
	Found   bool
	Type    string // "multicolor" / "image" / "translucent" / "text" / "color"
}

// 需要输出识别过程日志的预设名（用于排查"剧情对话"识别问题，确认命中/未命中、区域、相似度）
var debugRecogNames = map[string]bool{
	"剧情对话":         true,
	"剧情对话多个":      true,
	"剧情对话下一步":    true,
	"剧情_接取主线任务":  true,
	"购买药品":         true,
}

// debugRecogLog 输出剧情相关预设的识别过程日志（区域、相似度、命中坐标）
func debugRecogLog(name string, cfg map[string]interface{}, r *RecogResult) {
	if !debugRecogNames[name] {
		return
	}
	x1, y1, x2, y2 := recogRegion(cfg)
	sim := recogSim(cfg)
	if r.Found {
		LogWithTimef("[Recog] %s 区域[%d,%d,%d,%d] 相似度%.2f → 命中(%d,%d) type=%s", name, x1, y1, x2, y2, sim, r.X, r.Y, r.Type)
	} else {
		LogWithTimef("[Recog] %s 区域[%d,%d,%d,%d] 相似度%.2f → 未命中", name, x1, y1, x2, y2, sim)
	}
}

// Exec 根据预设名称执行识别（走 recogs.json 预设库）
// 预设内若配置"坐标偏移"[dx,dy]，命中后坐标加偏移返回；
// 若同时"返回中间坐标": true（找字/模板图/透明图），偏移已并入居中计算（左上角先加偏移再与右下角取中），不再重复叠加。
// 可选变参 region: 传入 [x1,y1,x2,y2] 时用该范围覆盖预设的"截图区域"（浅拷贝，不影响全局预设）
func Exec(name string, region ...int) *RecogResult {
	cfg := GetRecog(name)
	if cfg == nil {
		return &RecogResult{}
	}
	if len(region) >= 4 {
		cfg = copyRecogWithRegion(cfg, region[0], region[1], region[2], region[3])
	}
	r := ExecWithConfig(cfg)
	debugRecogLog(name, cfg, r)
	if r.Found {
		// recogs.json 任一预设命中 → 刷新空闲检测心跳
		RecordConfigHit()
		if off := recogOffset(cfg, "坐标偏移"); off != [2]int{} && !centerOffsetMerged(cfg) {
			r.X += off[0]
			r.Y += off[1]
		}
	}
	return r
}

// ExecWithCache 按预设名执行识别，并启用截图级缓存（区别于 ExecCached 的结果级缓存）：
// 同一 cacheKey 在 ttl 内只截一次屏，多次识别复用同一张截图（适合同一时刻批量检测多个目标/弹窗）。
// cacheKey 为空或 ttl<=0 时等价于 Exec（不走截图缓存）。
// 可选变参 region: 传入 [x1,y1,x2,y2] 时用该范围覆盖预设的"截图区域"（浅拷贝，不影响全局预设）。
func ExecWithCache(name string, cacheKey string, ttl time.Duration, region ...int) *RecogResult {
	cfg := GetRecog(name)
	if cfg == nil {
		return &RecogResult{}
	}
	if len(region) >= 4 {
		cfg = copyRecogWithRegion(cfg, region[0], region[1], region[2], region[3])
	}
	r := ExecWithConfig(cfg, cacheKey, ttl)
	debugRecogLog(name, cfg, r)
	if r.Found {
		// recogs.json 任一预设命中 → 刷新空闲检测心跳
		RecordConfigHit()
		if off := recogOffset(cfg, "坐标偏移"); off != [2]int{} && !centerOffsetMerged(cfg) {
			r.X += off[0]
			r.Y += off[1]
		}
	}
	return r
}

// ExecWithConfig 根据识别配置 map 执行识别（预设 / 临时构造的配置均可）
// 预设字段 "返回中间坐标": true（默认 false = 返回原命中点）时，命中统一返回目标中间坐标：
// 找字 / 模板图 / 透明图 → 命中块矩形中心；多点找色 / 多点找色数组无块边界 → 命中点即 first 色点坐标（居中不额外加偏移）；
// 单色点、OCR 取文 / 取文字库无目标块，命中点即目标点，坐标不变。
// "坐标偏移"[dx,dy] 相对命中原始点(块左上角)叠加：单独配 = 左上角+偏移；
// 与"返回中间坐标"同配时（仅找字/模板图/透明图这些有块边界的类型，见 centerOffsetMerged），
// 在块内先给左上角加偏移、再与块右下角取中：((左+dx)+右)/2,((上+dy)+下)/2，Exec 外层不再重复叠加。
// cacheArgs: 可选参数 (cacheKey string, cacheExpire time.Duration)：
// 传入时开启截图级缓存，同一 cacheKey 在过期时间内复用同一张截图（不重复截屏），不传则不走缓存。
func ExecWithConfig(cfg map[string]interface{}, cacheArgs ...interface{}) *RecogResult {
	x1, y1, x2, y2 := recogRegion(cfg)

	// 1) 多点找色
	if mc := recogString(cfg, "多点找色"); mc != "" {
		first, rest := SplitMultiColor(mc)
		sim := recogSim(cfg)
		if sim <= 0 {
			sim = 0.9
		}
		count := recogCount(cfg)
		if count <= 0 {
			count = 1
		}
		dir := recogInt(cfg, "找色方向")
		if dir <= 0 {
			dir = 1
		}
		x, y, ok := FindMultiColor(x1, y1, x2, y2, first, rest, sim, count, dir, cacheArgs...)
		if !ok {
			return &RecogResult{}
		}
		// 多点找色无块边界：命中点即 first 色点坐标（配"返回中间坐标"也不加偏移，first 点本身就在目标上）
		return &RecogResult{X: x, Y: y, Sim: sim, Found: true, Type: "multicolor"}
	}

	// 1.5) 多点找色数组：逐个试，任一组合命中即返回（兼容一个预设配多组多点找色）
	if mcs := recogArray(cfg, "多点找色数组"); len(mcs) > 0 {
		sim := recogSim(cfg)
		if sim <= 0 {
			sim = 0.9
		}
		count := recogCount(cfg)
		if count <= 0 {
			count = 1
		}
		dir := recogInt(cfg, "找色方向")
		if dir <= 0 {
			dir = 1
		}
		for _, m := range mcs {
			mcStr, ok := m.(string)
			if !ok || mcStr == "" {
				continue
			}
			first, rest := SplitMultiColor(mcStr)
			x, y, ok2 := FindMultiColor(x1, y1, x2, y2, first, rest, sim, count, dir, cacheArgs...)
			if ok2 {
				// 多点找色无块边界：命中点即 first 色点坐标
				return &RecogResult{X: x, Y: y, Sim: sim, Found: true, Type: "multicolor"}
			}
		}
		return &RecogResult{}
	}

	// 2) 模板图片
	if imgs := recogArray(cfg, "模板图片"); len(imgs) > 0 {
		sim := float32(recogSim(cfg))
		if sim <= 0 {
			sim = 0.9
		}
		strict := recogInt(cfg, "严格模式") != 0
		for _, img := range imgs {
			imgName, ok := img.(string)
			if !ok {
				continue
			}
			// "返回中间坐标": true → 左上角先加"坐标偏移", 再与匹配矩形右下角取中
			if wantCenter(cfg) {
				ms := FindImgFastAll(x1, y1, x2, y2, imgName, strict, sim, 1, cacheArgs...)
				if len(ms) > 0 && ms[0].Sim >= sim {
					off := recogOffset(cfg, "坐标偏移")
					cx := (ms[0].X + off[0] + ms[0].X2) / 2
					cy := (ms[0].Y + off[1] + ms[0].Y2) / 2
					return &RecogResult{X: cx, Y: cy, Sim: float64(ms[0].Sim), Found: true, Type: "image"}
				}
				continue
			}
			x, y, s := FindImgFast(x1, y1, x2, y2, imgName, strict, sim, cacheArgs...)
			if s >= sim {
				return &RecogResult{X: x, Y: y, Sim: float64(s), Found: true, Type: "image"}
			}
		}
		return &RecogResult{}
	}

	// 2.5) 半透明找图（透明图：模板四角同色视为背景忽略，适合背景变化的半透明图标）
	if imgs := recogArray(cfg, "透明图"); len(imgs) > 0 {
		sim := recogSim(cfg)
		if sim <= 0 {
			sim = 0.95
		}
		for _, img := range imgs {
			imgName, ok := img.(string)
			if !ok {
				continue
			}
			ms := FindTranslucentImgAll(x1, y1, x2, y2, imgName, float32(sim), 1, cacheArgs...)
			if len(ms) > 0 {
				if wantCenter(cfg) {
					off := recogOffset(cfg, "坐标偏移")
					cx := (ms[0].X + off[0] + ms[0].X2) / 2
					cy := (ms[0].Y + off[1] + ms[0].Y2) / 2
					return &RecogResult{X: cx, Y: cy, Sim: float64(ms[0].Sim), Found: true, Type: "translucent"}
				}
				return &RecogResult{X: ms[0].X, Y: ms[0].Y, Sim: float64(ms[0].Sim), Found: true, Type: "translucent"}
			}
		}
		return &RecogResult{}
	}

	// 3) 识别文字分类：找字(默认) / 取文(paddle取文) / 取文字库(字库识字取文)
	// 找字：字库 FindStr 匹配目标文字；取文：paddle 读回区域全部文字；
	// 取文字库：字库 GetWords 识字读回区域全部文字（取文类均不匹配目标文字）。
	cat := recogString(cfg, "识别文字分类")
	if cat == "" && recogBool(cfg, "取文模式") { // 兼容旧字段"取文模式":true
		cat = "取文"
	}
	switch cat {
	case "取文": // 单纯取文：飞浆OCR把区域内文字原样读出来返回
		threshold := recogSim(cfg)
		if threshold <= 0 {
			threshold = 0.5
		}
		// 颜色信息：传了就按该颜色识别（格式 "#FFFFFF-0x101010|#E1E1E1-0x101010"）
		colorStr := recogString(cfg, "颜色信息")
		return OcrPaddleText(x1, y1, x2, y2, threshold, colorStr)
	case "取文字库": // 取文字库：字库识字把区域内文字全部读出来返回
		threshold := recogSim(cfg)
		if threshold <= 0 {
			threshold = 0.9
		}
		font := recogString(cfg, "字库名")
		if font == "" {
			font = "library"
		}
		// 颜色信息：可配置如 "#FFFFFF-0x101010|#E1E1E1-0x101010"；为空则自动使用字库内置颜色。
		// 只要传了颜色信息，就自动开启颜色过滤（用该颜色对字库筛选提速，需与字库颜色一致）
		colorInfo := recogString(cfg, "颜色信息")
		x, y, sim, info := GetWords(x1, y1, x2, y2, colorInfo, threshold, colorInfo != "", font, cacheArgs...)
		if x < 0 || y < 0 {
			return &RecogResult{}
		}
		return &RecogResult{X: x, Y: y, Sim: sim, Word: info, Found: true, Type: "text"}
	default: // 找字（默认）：字库 FindStr 查找目标文字，命中才 Found
		if text := recogString(cfg, "识别文字"); text != "" {
			threshold := recogSim(cfg)
			if threshold <= 0 {
				threshold = 0.85
			}
			font := recogString(cfg, "字库名")
			if font == "" {
				font = "library"
			}
			// "返回中间坐标": true 时, 命中坐标 = 文字块左上角先加"坐标偏移", 再与块右下角取中:
			//   只配"坐标偏移" → (左上+dx, 左上+dy); 只配"返回中间坐标" → 文字块中心;
			//   两者都配 → ((左+dx)+右)/2, ((上+dy)+下)/2（偏移量相对左上角, 先偏后居中, Exec 层不再重复叠加）
			// （单点 FindStr 只回左上角，拿不到块宽高，须走 FindStrAll 取首个块的边界算中点）
			if recogBool(cfg, "返回中间坐标") {
				ms := FindStrAll(x1, y1, x2, y2, text, threshold, font, 1, cacheArgs...)
				if len(ms) == 0 {
					return &RecogResult{}
				}
				m := ms[0]
				off := recogOffset(cfg, "坐标偏移")
				return &RecogResult{
					X:     (m.Left + off[0] + m.Right) / 2,
					Y:     (m.Top + off[1] + m.Bottom) / 2,
					Sim:   m.Similarity,
					Word:  text,
					Found: true,
					Type:  "text",
				}
			}
			x, y, s, w := FindStr(x1, y1, x2, y2, text, threshold, font, cacheArgs...)
			return &RecogResult{X: x, Y: y, Sim: s, Word: w, Found: x > 0 && y > 0, Type: "text"}
		}
	}

	// 4) 颜色数组
	if colors := recogArray(cfg, "颜色数组"); len(colors) > 0 {
		sim := recogSim(cfg)
		if sim <= 0 {
			sim = 0.9
		}
		count := recogCount(cfg)
		if count <= 0 {
			count = 1
		}
		dir := recogInt(cfg, "找色方向")
		if dir <= 0 {
			dir = 1
		}
		// 单色点无块宽高可算中心："返回中间坐标" 下命中点即目标像素本身，坐标不变
		for _, c := range colors {
			clr, ok := c.(string)
			if !ok {
				continue
			}
			x, y, ok2 := FindColor(x1, y1, x2, y2, clr, sim, count, dir, cacheArgs...)
			if ok2 {
				return &RecogResult{X: x, Y: y, Sim: sim, Found: true, Type: "color"}
			}
		}
		return &RecogResult{}
	}

	return &RecogResult{}
}

// ExecAll 根据预设名称执行识别，返回全部匹配点（走 recogs.json 预设库）
// 多点找色按预设"寻找个数"返回所有命中点；其余识别类型（模板图/透明图/OCR/颜色数组）退回单点
// 可选变参 region: 传入 [x1,y1,x2,y2] 时用该范围覆盖预设的"截图区域"（浅拷贝，不影响全局预设）
func ExecAll(name string, region ...int) []*RecogResult {
	cfg := GetRecog(name)
	if cfg == nil {
		return nil
	}
	if len(region) >= 4 {
		cfg = copyRecogWithRegion(cfg, region[0], region[1], region[2], region[3])
	}
	rs := ExecAllWithConfig(cfg)
	if len(rs) > 0 {
		// recogs.json 任一预设（多点形式）命中 → 刷新空闲检测心跳
		RecordConfigHit()
		if debugRecogNames[name] {
			x1, y1, x2, y2 := recogRegion(cfg)
			sim := recogSim(cfg)
			pts := make([]string, 0, len(rs))
			for _, p := range rs {
				pts = append(pts, fmt.Sprintf("(%d,%d)", p.X, p.Y))
			}
			LogWithTimef("[Recog] %s 区域[%d,%d,%d,%d] 相似度%.2f → 命中%d点 %v", name, x1, y1, x2, y2, sim, len(rs), pts)
		}
	} else if debugRecogNames[name] {
		x1, y1, x2, y2 := recogRegion(cfg)
		sim := recogSim(cfg)
		LogWithTimef("[Recog] %s 区域[%d,%d,%d,%d] 相似度%.2f → 未命中", name, x1, y1, x2, y2, sim)
	}
	return rs
}

// ExecAllWithConfig 根据识别配置 map 执行识别，返回全部匹配点（预设 / 临时构造的配置均可）
// cacheArgs: 可选参数 (cacheKey string, cacheExpire time.Duration)，语义同 ExecWithConfig
func ExecAllWithConfig(cfg map[string]interface{}, cacheArgs ...interface{}) []*RecogResult {
	x1, y1, x2, y2 := recogRegion(cfg)

	// 1) 多点找色：返回全部命中点
	if mc := recogString(cfg, "多点找色"); mc != "" {
		first, rest := SplitMultiColor(mc)
		sim := recogSim(cfg)
		if sim <= 0 {
			sim = 0.9
		}
		count := recogCount(cfg)
		if count <= 0 {
			count = 1
		}
		dir := recogInt(cfg, "找色方向")
		if dir <= 0 {
			dir = 1
		}
		pts := FindMultiColorAll(x1, y1, x2, y2, first, rest, sim, count, dir, cacheArgs...)
		if len(pts) == 0 {
			return nil
		}
		off := recogOffset(cfg, "坐标偏移")
		// 多点找色无块边界：命中点即 first 色点坐标（配"返回中间坐标"也不加偏移）；"坐标偏移"仍在此叠加
		results := make([]*RecogResult, 0, len(pts))
		for _, p := range pts {
			results = append(results, &RecogResult{X: p.X + off[0], Y: p.Y + off[1], Sim: sim, Found: true, Type: "multicolor"})
		}
		return results
	}

	// 1.5) 多点找色数组：返回全部组合的命中点（逐个组合 FindMultiColorAll 累加）
	if mcs := recogArray(cfg, "多点找色数组"); len(mcs) > 0 {
		sim := recogSim(cfg)
		if sim <= 0 {
			sim = 0.9
		}
		count := recogCount(cfg)
		if count <= 0 {
			count = 1
		}
		dir := recogInt(cfg, "找色方向")
		if dir <= 0 {
			dir = 1
		}
		off := recogOffset(cfg, "坐标偏移")
		results := make([]*RecogResult, 0)
		for _, m := range mcs {
			mcStr, ok := m.(string)
			if !ok || mcStr == "" {
				continue
			}
			first, rest := SplitMultiColor(mcStr)
			pts := FindMultiColorAll(x1, y1, x2, y2, first, rest, sim, count, dir, cacheArgs...)
			for _, p := range pts {
				results = append(results, &RecogResult{X: p.X + off[0], Y: p.Y + off[1], Sim: sim, Found: true, Type: "multicolor"})
			}
		}
		if len(results) == 0 {
			return nil
		}
		return results
	}

	// 2) 识别文字（找字）：支持"寻找个数">1 返回多个匹配点
	// 其余识别类型退回单点
	cat := recogString(cfg, "识别文字分类")
	if cat == "" && recogBool(cfg, "取文模式") {
		cat = "取文"
	}
	if (cat == "" || cat == "找字") && recogString(cfg, "识别文字") != "" {
		count := recogCount(cfg)
		if count > 1 {
			threshold := recogSim(cfg)
			if threshold <= 0 {
				threshold = 0.85
			}
			font := recogString(cfg, "字库名")
			if font == "" {
				font = "library"
			}
			text := recogString(cfg, "识别文字")
			ms := FindStrAll(x1, y1, x2, y2, text, threshold, font, count, cacheArgs...)
			if len(ms) == 0 {
				return nil
			}
			off := recogOffset(cfg, "坐标偏移")
			results := make([]*RecogResult, 0, len(ms))
			for _, m := range ms {
				var cx, cy int
				if wantCenter(cfg) { // "返回中间坐标": 左上角先加偏移再与右下角取中（X2/Y2 保留块右下角供取矩形）
					cx, cy = centerOf(m.Left+off[0], m.Top+off[1], m.Right, m.Bottom)
				} else {
					cx, cy = m.Left+off[0], m.Top+off[1]
				}
				results = append(results, &RecogResult{
					X:     cx,
					Y:     cy,
					X2:    m.Right + off[0],
					Y2:    m.Bottom + off[1],
					Sim:   m.Similarity,
					Word:  text,
					Found: true,
					Type:  "text",
				})
			}
			return results
		}
	}

	// 3) 其余识别类型退回单点
	if r := ExecWithConfig(cfg, cacheArgs...); r.Found {
		return []*RecogResult{r}
	}
	return nil
}

// ExecCached 按预设名执行识别并缓存结果。
// 只缓存命中(Found)的结果，未命中不缓存（延续"没找到不缓存"原则，避免挡住新出现的目标）；
// ttl<=0 表示不缓存，每次真识别。返回的是缓存值的拷贝，外部修改不影响缓存。
func ExecCached(name string, ttl time.Duration) *RecogResult {
	key := "recog:" + name
	if ttl > 0 {
		if v, ok := CacheGet(key); ok {
			if r, ok := v.(*RecogResult); ok && r != nil {
				cp := *r
				return &cp
			}
		}
	}
	r := Exec(name)
	if r.Found && ttl > 0 {
		cp := *r
		CacheSet(key, &cp, ttl)
	}
	return r
}

// ---- helpers ----

// copyRecogWithRegion 浅拷贝识别预设并覆盖"截图区域"为指定范围（不影响全局预设库）
func copyRecogWithRegion(cfg map[string]interface{}, x1, y1, x2, y2 int) map[string]interface{} {
	nc := make(map[string]interface{}, len(cfg)+1)
	for k, v := range cfg {
		nc[k] = v
	}
	nc["截图区域"] = []interface{}{float64(x1), float64(y1), float64(x2), float64(y2)}
	return nc
}

func recogRegion(cfg map[string]interface{}) (int, int, int, int) {
	arr, ok := cfg["截图区域"].([]interface{})
	if !ok || len(arr) < 4 {
		return 0, 0, 0, 0
	}
	return toInt(arr[0]), toInt(arr[1]), toInt(arr[2]), toInt(arr[3])
}

func recogString(cfg map[string]interface{}, key string) string {
	if s, ok := cfg[key].(string); ok {
		return s
	}
	return ""
}

func recogFloat(cfg map[string]interface{}, key string) float64 {
	if f, ok := cfg[key].(float64); ok {
		return f
	}
	return 0
}

// recogSim 读取相似度阈值 —— 全项目统一字段名 "相似度"（多点找色/找图/找色/取文/取文字库/找字
// 全都用这一个字段，没有第二种写法）。
// 返回 0 = 该预设没配，由各识别分支套自己的默认值（多点找色/找图 0.9、透明图 0.95 等）。
func recogSim(cfg map[string]interface{}) float64 {
	return recogFloat(cfg, "相似度")
}

func recogInt(cfg map[string]interface{}, key string) int {
	if f, ok := cfg[key].(float64); ok {
		return int(f)
	}
	return 0
}

// recogCount 读取"要找几个"的数量配置 —— 全项目统一字段名 "寻找个数"
// （多点找色 / 多点找色数组 / 颜色数组 / 找字 全都用这一个字段，没有第二种写法）。
// 兼容旧字段 "找色数量"：只有在新字段没配时才回退读它，旧配置不会失效。
// 返回 0 = 都没配，由各分支套自己的默认值（通常 1）。
func recogCount(cfg map[string]interface{}) int {
	if n := recogInt(cfg, "寻找个数"); n > 0 {
		return n
	}
	return recogInt(cfg, "找色数量") // 兼容旧写法
}

func recogBool(cfg map[string]interface{}, key string) bool {
	if b, ok := cfg[key].(bool); ok {
		return b
	}
	return false
}

func recogArray(cfg map[string]interface{}, key string) []interface{} {
	if arr, ok := cfg[key].([]interface{}); ok {
		return arr
	}
	return nil
}

// recogOffset 读取预设内的 [dx, dy] 偏移配置
func recogOffset(cfg map[string]interface{}, key string) [2]int {
	var off [2]int
	if arr := recogArray(cfg, key); len(arr) >= 2 {
		off[0] = toInt(arr[0])
		off[1] = toInt(arr[1])
	}
	return off
}

func toInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// ---- "返回中间坐标" 通用支持 ----

// wantCenter 是否命中后返回目标中间坐标（预设字段 "返回中间坐标": true，默认 false = 返回原命中点）
func wantCenter(cfg map[string]interface{}) bool {
	return recogBool(cfg, "返回中间坐标")
}

// centerOffsetMerged 判断该预设的"坐标偏移"是否已并入"返回中间坐标"的居中计算
// （识别有块边界：找字文字块 / 模板图匹配矩形 / 透明图矩形）。
// 命中后 Exec/ExecWithCache 据此跳过外层重复叠加（块内已按"左上角先加偏移再与右下角取中"算好）。
// 无块边界的类型（多点找色 / 单色点 / OCR 取文）不并入，"坐标偏移"仍按"命中点(左上角)+偏移"叠加。
// 其中多点找色即便配了"返回中间坐标"也不产生任何居中偏移（命中点即 first 色点坐标）。
func centerOffsetMerged(cfg map[string]interface{}) bool {
	if !recogBool(cfg, "返回中间坐标") {
		return false
	}
	// 与 ExecWithConfig 类型优先级一致：多点找色/多点找色数组最先命中，无块边界 → 不并入
	if recogString(cfg, "多点找色") != "" || len(recogArray(cfg, "多点找色数组")) > 0 {
		return false
	}
	if len(recogArray(cfg, "模板图片")) > 0 || len(recogArray(cfg, "透明图")) > 0 {
		return true
	}
	// 找字（default 分支）有文字块边界 → 并入；取文/取文字库无块 → 不并入
	cat := recogString(cfg, "识别文字分类")
	if recogBool(cfg, "取文模式") {
		cat = "取文"
	}
	return recogString(cfg, "识别文字") != "" && (cat == "" || cat == "找字")
}

// centerOf 计算矩形 (l,t)-(r,b) 的中心点
func centerOf(l, t, r, b int) (int, int) {
	return (l + r) / 2, (t + b) / 2
}

// ---- 识别命中心跳（空闲检测判定依据） ----
// recogs.json 任一预设命中都会刷新心跳；空闲检测以它为唯一判定依据：
// 全部预设连续超过阈值无任何命中 → 空闲/卡死。
var lastConfigHit atomic.Value // time.Time

func init() {
	lastConfigHit.Store(time.Now()) // 启动起点，避免一启动就被判空闲
}

// RecordConfigHit 记录一次识别配置命中
func RecordConfigHit() {
	lastConfigHit.Store(time.Now())
}

// LastConfigHit 返回最后一次识别配置命中的时间
func LastConfigHit() time.Time {
	t, _ := lastConfigHit.Load().(time.Time)
	return t
}
