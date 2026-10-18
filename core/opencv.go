package core

import (
	imageHandler "app/imageHandler"
	"fmt"
	"time"
)

// parseScreenCacheArgs 解析可选截图缓存参数：(cacheKey string, cacheExpire time.Duration)。
// cacheKey 为空或 cacheExpire<=0 表示不走缓存，返回 ("", 0)。
func parseScreenCacheArgs(args ...interface{}) (string, time.Duration) {
	if len(args) == 0 {
		return "", 0
	}
	key, ok := args[0].(string)
	if !ok || key == "" {
		return "", 0
	}
	var ttl time.Duration
	if len(args) >= 2 {
		if d, ok := args[1].(time.Duration); ok && d > 0 {
			ttl = d
		}
	}
	if ttl <= 0 {
		return "", 0
	}
	return key, ttl
}

// captureScreenMat 截屏，支持可选截图缓存：
// 传 (cacheKey, cacheExpire) 时，同一 cacheKey 在 cacheExpire 内复用同一张截图（不重复截屏）。
// 返回的 Mat 一律由调用方负责 Recycle（缓存命中返回的是副本）。
func captureScreenMat(x1, y1, x2, y2 int, cacheArgs ...interface{}) *imageHandler.Mat {
	key, ttl := parseScreenCacheArgs(cacheArgs...)
	if key == "" {
		return imageHandler.CaptureScreenMat(x1, y1, x2, y2)
	}
	return imageHandler.CaptureScreenMatCached(key, ttl, x1, y1, x2, y2)
}

// FindImgFast 使用 imageHandler.FindImgFast 快速查找图像
// isStrictMode: 严格模式，如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false（对应 SceneConfig 的 isStrictMode 字段）
// cacheArgs: 可选参数：cacheKey (string), cacheExpire (time.Duration)：同一 key 在过期时间内复用同一张截图，不传则不走缓存
// 返回值: (x, y, similarity) - 找到返回坐标和相似度，未找到返回 (-1, -1, similarity)
func FindImgFast(x1, y1, x2, y2 int, picName string, isStrictMode bool, sim float32, cacheArgs ...interface{}) (int, int, float32) {
	matches := FindImgFastAll(x1, y1, x2, y2, picName, isStrictMode, sim, 1, cacheArgs...)
	if len(matches) > 0 {
		return matches[0].X, matches[0].Y, matches[0].Sim
	}
	return -1, -1, 0
}

// FindImgFastAll 使用 imageHandler.FindImgFast 快速查找多个图像
// isStrictMode: 严格模式，如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false（对应 SceneConfig 的 isStrictMode 字段）
// count: 查找数量，默认1
// cacheArgs: 可选参数：cacheKey (string), cacheExpire (time.Duration)：同一 key 在过期时间内复用同一张截图，不传则不走缓存
// 返回值: 所有匹配结果（坐标已加上区域偏移）
func FindImgFastAll(x1, y1, x2, y2 int, picName string, isStrictMode bool, sim float32, count int, cacheArgs ...interface{}) []MatchResult {
	if count <= 0 {
		count = 1
	}

	screenMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	defer screenMat.Recycle()
	if screenMat.Empty() {
		return nil
	}

	templateMat := imageHandler.ReadResMat(picName)
	defer templateMat.Recycle()
	if templateMat.Empty() {
		return nil
	}

	threshold := float64(0.5 + sim*0.5)
	matches := imageHandler.FindImgFast(screenMat, templateMat, threshold, 0, 0, 0, 0, count, isStrictMode)

	if matches == nil || len(matches) == 0 {
		return nil
	}

	// 处理所有结果的坐标（加上区域偏移）
	result := make([]MatchResult, 0, len(matches))
	for _, match := range matches {
		result = append(result, MatchResult{
			X:   match.Left + x1,
			Y:   match.Top + y1,
			X2:  match.Right + x1,
			Y2:  match.Bottom + y1,
			Sim: float32(match.Similarity),
		})
	}
	return result
}

// MatchResult 匹配结果（坐标已加上区域偏移，但未应用scale）
type MatchResult struct {
	X, Y, X2, Y2 int
	Sim          float32
}

// FindImgContour 使用 imageHandler.FindImgByContour 轮廓找图
// imageType=1 时使用：只做轮廓匹配，基本无视颜色
// cacheArgs: 可选参数：cacheKey (string), cacheExpire (time.Duration)：同一 key 在过期时间内复用同一张截图，不传则不走缓存
// 返回值: (x, y, similarity) - 找到返回坐标和相似度，未找到返回 (-1, -1, similarity)
func FindImgContour(x1, y1, x2, y2 int, picName string, sim float32, cacheArgs ...interface{}) (int, int, float32) {

	screenMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	defer screenMat.Recycle()
	if screenMat.Empty() {
		return -1, -1, 0
	}

	templateMat := imageHandler.ReadResMat(picName)
	defer templateMat.Recycle()
	if templateMat.Empty() {
		return -1, -1, 0
	}

	// 使用轮廓匹配：1.0 缩放，阈值按 sim 直接使用（若未配置 sim，可在配置里给 0.8）
	resize := 1.0
	threshold := float64(sim)
	if threshold <= 0 {
		threshold = 0.8
	}
	matches := imageHandler.FindImgByContour(screenMat, templateMat, resize, threshold, -1, 0, 0, 0, 1, 50, 100)
	fmt.Println(matches)

	if matches != nil && len(matches) > 0 {
		return matches[0].Left + x1, matches[0].Top + y1, float32(matches[0].Similarity)
	}

	return -1, -1, 0
}

// FindTranslucentImg 使用 imageHandler.FindTranslucentImg 半透明找图
// cacheArgs: 可选参数：cacheKey (string), cacheExpire (time.Duration)：同一 key 在过期时间内复用同一张截图，不传则不走缓存
// 返回值: (x, y, similarity) - 找到返回坐标和相似度，未找到返回 (-1, -1, similarity)
func FindTranslucentImg(x1, y1, x2, y2 int, picName string, sim float32, cacheArgs ...interface{}) (int, int, float32) {
	matches := FindTranslucentImgAll(x1, y1, x2, y2, picName, sim, 1, cacheArgs...)
	if len(matches) > 0 {
		return matches[0].X, matches[0].Y, matches[0].Sim
	}
	return -1, -1, 0
}

// FindTranslucentImgAll 使用 imageHandler.FindTranslucentImg 半透明找图（支持多个结果）
// count: 查找数量，默认1
// cacheArgs: 可选参数：cacheKey (string), cacheExpire (time.Duration)：同一 key 在过期时间内复用同一张截图，不传则不走缓存
// 返回值: 所有匹配结果（坐标已加上区域偏移）
func FindTranslucentImgAll(x1, y1, x2, y2 int, picName string, sim float32, count int, cacheArgs ...interface{}) []MatchResult {

	if count <= 0 {
		count = 1
	}

	screenMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	defer screenMat.Recycle()
	if screenMat.Empty() {
		return nil
	}

	templateMat := imageHandler.ReadResMat(picName)
	defer templateMat.Recycle()
	if templateMat.Empty() {
		return nil
	}

	resize := 1.0
	threshold := float64(0.5 + sim*0.5)
	colorOffset := 20
	matches := imageHandler.FindTranslucentImg(screenMat, templateMat, resize, threshold, 0, 0, 0, 0, count, colorOffset)

	if matches == nil || len(matches) == 0 {
		return nil
	}

	// 处理所有结果的坐标（加上区域偏移）
	result := make([]MatchResult, 0, len(matches))
	for _, match := range matches {
		result = append(result, MatchResult{
			X:   match.Left + x1,
			Y:   match.Top + y1,
			X2:  match.Right + x1,
			Y2:  match.Bottom + y1,
			Sim: float32(match.Similarity),
		})
	}
	return result
}

// FindImageAnyContour 在指定区域内使用轮廓匹配查找多张图片，找到任意一张即返回成功
// imageType=1 时由场景层调用
func FindImageAnyContour(x1, y1, x2, y2 int, picNames []string, sim float32) (int, int, float32, string) {

	if len(picNames) == 0 {
		return -1, -1, 0, ""
	}

	screenMat := imageHandler.CaptureScreenMat(x1, y1, x2, y2)
	defer screenMat.Recycle()
	if screenMat.Empty() {
		return -1, -1, 0, ""
	}

	for _, picName := range picNames {
		templateMat := imageHandler.ReadResMat(picName)
		if templateMat.Empty() {
			templateMat.Recycle()
			continue
		}

		resize := 1.0
		threshold := float64(sim)
		if threshold <= 0 {
			threshold = 0.8
		}
		matches := imageHandler.FindImgByContour(screenMat, templateMat, resize, threshold, -1, 0, 0, 0, 1, 50, 100)
		templateMat.Recycle()
		if matches != nil && len(matches) > 0 {
			return matches[0].Left + x1, matches[0].Top + y1, float32(matches[0].Similarity), picName
		}
	}

	return -1, -1, 0, ""
}

// FindStr 在指定区域内查找文字
// 参数:
//   - x1, y1, x2, y2: 搜索区域坐标（如果 x2=0 或 y2=0，则截取全屏）
//   - text: 要查找的文字
//   - threshold: 相似度阈值 (0.0-1.0)，默认 0.9
//   - fontName: 字库名称，为空字符串则使用默认字库
//
// 返回值: (centerX, centerY, similarity, label) - 找到返回中心坐标、相似度和文字，未找到返回 (-1, -1, 0, "")
func FindStr(x1, y1, x2, y2 int, text string, threshold float64, fontName string, cacheArgs ...interface{}) (int, int, float64, string) {

	// 截取屏幕图片
	srcMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	if srcMat == nil || srcMat.Empty() {
		return -1, -1, 0, ""
	}
	defer srcMat.Recycle()

	// 如果 threshold 为 0，使用默认值 0.9
	if threshold <= 0 {
		threshold = 0.5
	}

	// 调用 FindStr 查找文字
	results := imageHandler.FindStr(srcMat, text, 0, 0, 0, 0, threshold, 1, fontName)
	if results == nil || len(results) == 0 {
		return -1, -1, 0, ""
	}

	// 返回第一个匹配结果（返回左上角坐标）
	match := results[0]

	// 判断是否为全屏：如果 x1, y1, x2, y2 不全为0，说明不是全屏，返回的是相对坐标，需要转换为绝对坐标
	isFullScreen := (x1 == 0 && y1 == 0 && x2 == 0 && y2 == 0)
	if !isFullScreen {
		// 不是全屏，将相对坐标转换为绝对坐标
		return match.Left + x1, match.Top + y1, match.Similarity, text
	}

	// 全屏模式，直接返回坐标
	return match.Left, match.Top, match.Similarity, text
}

// FindStrAll 在指定区域内查找文字并返回全部匹配点（字库找字多结果）
// 参数:
//   - x1, y1, x2, y2: 搜索区域坐标（如果 x2=0 或 y2=0，则截取全屏）
//   - text: 要查找的文字
//   - threshold: 相似度阈值 (0.0-1.0)，默认 0.5
//   - fontName: 字库名称，为空字符串则使用默认字库
//   - count: 最大匹配数量（<=0 按 1 处理）
//
// 返回值: 全部匹配点（坐标已加区域偏移转为绝对坐标），未找到返回 nil。
// 每个 Match 的 Left/Top 为文字左上角，Right/Bottom 为右下角，可直接用于区域计算。
func FindStrAll(x1, y1, x2, y2 int, text string, threshold float64, fontName string, count int, cacheArgs ...interface{}) []imageHandler.Match {
	// 截取屏幕图片
	srcMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	if srcMat == nil || srcMat.Empty() {
		return nil
	}
	defer srcMat.Recycle()

	if threshold <= 0 {
		threshold = 0.5
	}
	if count <= 0 {
		count = 1
	}

	results := imageHandler.FindStr(srcMat, text, 0, 0, 0, 0, threshold, count, fontName)
	if len(results) == 0 {
		return nil
	}

	// 所有匹配结果转为绝对坐标
	for i := range results {
		results[i].Left += x1
		results[i].Top += y1
		results[i].Right += x1
		results[i].Bottom += y1
	}
	return results
}

// GetWords 在指定区域内识字（字库识字）
// 参数:
//   - x1, y1, x2, y2: 搜索区域坐标（如果 x2=0 或 y2=0，则截取全屏）
//   - colorInfo: 颜色信息，例如 "#FFFFFF-0x101010|#E1E1E1-0x101010"，如果为空字符串则自动使用字库的颜色
//   - threshold: 相似度阈值 (0.0-1.0)，默认 0.9
//   - colorFilter: 是否用设定的colorInfo对字库进行筛选，默认 false
//   - fontName: 字库名称，为空字符串则使用默认字库
//
// 返回值: (leftX, topY, similarity, label) - 找到返回左上角坐标、相似度和文字，未找到返回 (-1, -1, 0, "")
func GetWords(x1, y1, x2, y2 int, colorInfo string, threshold float64, colorFilter bool, fontName string, cacheArgs ...interface{}) (int, int, float64, string) {
	// 截取屏幕图片
	srcMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	if srcMat == nil || srcMat.Empty() {
		LogWithTimef("[GetWords] 截屏为空, 返回未找到")
		return -1, -1, 0, ""
	}
	defer srcMat.Recycle()

	// 如果 threshold 为 0，使用默认值 0.9
	if threshold <= 0 {
		threshold = 0.9
	}

	// 调用 GetWords 识字
	results := imageHandler.GetWords(srcMat, colorInfo, 0, 0, 0, 0, threshold, colorFilter, fontName)
	if results == nil || len(results) == 0 {
		return -1, -1, 0, ""
	}

	// 返回第一个匹配结果（返回左上角坐标）
	match := results[0]

	// 判断是否为全屏：如果 x1, y1, x2, y2 不全为0，说明不是全屏，返回的是相对坐标，需要转换为绝对坐标
	isFullScreen := (x1 == 0 && y1 == 0 && x2 == 0 && y2 == 0)
	if !isFullScreen {
		// 不是全屏，将相对坐标转换为绝对坐标（返回左上角）
		LogWithTimef("[GetWords] 命中 文字=%q 坐标=(%d,%d) 相似度=%v", match.Info, match.Left+x1, match.Top+y1, match.Similarity)
		return match.Left + x1, match.Top + y1, match.Similarity, match.Info
	}

	// 全屏模式，直接返回左上角坐标
	LogWithTimef("[GetWords] 命中 文字=%q 坐标=(%d,%d) 相似度=%v", match.Info, match.Left, match.Top, match.Similarity)
	return match.Left, match.Top, match.Similarity, match.Info
}

// FindColor 在指定区域内单点找色
// 参数:
//   - x1, y1, x2, y2: 检测区域坐标
//   - color: 要找的颜色，例如 "#D3AC14-0x2C2614"
//   - threshold: 颜色相似度阈值，范围 0.0-1.0，默认 0.9
//   - count: 寻找的个数，默认 1
//   - direction: 寻找的方向 1~8，默认 1
//
// 返回值: (x, y, isFind) - 找到返回坐标和true，未找到返回 (-1, -1, false)
func FindColor(x1, y1, x2, y2 int, color string, threshold float64, count, direction int, cacheArgs ...interface{}) (int, int, bool) {

	// 截取屏幕图片
	srcMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	if srcMat == nil || srcMat.Empty() {
		return -1, -1, false
	}
	defer srcMat.Recycle()

	// 如果 threshold 为 0，使用默认值 0.9
	if threshold <= 0 {
		threshold = 0.9
	}

	// 如果 count 为 0，使用默认值 1
	if count <= 0 {
		count = 1
	}

	// 如果 direction 为 0，使用默认值 1
	if direction <= 0 {
		direction = 1
	}

	// 调用 FindColor 找色
	points := imageHandler.FindColor(srcMat, color, threshold, 0, 0, 0, 0, count, direction)
	if points == nil || len(points) == 0 {
		return -1, -1, false
	}

	// 返回第一个匹配结果的坐标（转换为绝对坐标）
	point := points[0]
	return point.X + x1, point.Y + y1, true
}

// findMultiColorPoints 多点找色公共实现：截屏 → 调 opencv 现成方法 → 坐标转绝对坐标
// count 为最大匹配数量；返回全部匹配点，未找到返回 nil
func findMultiColorPoints(x1, y1, x2, y2 int, firstColor, colorMulti string, threshold float64, count, direction int, cacheArgs ...interface{}) []imageHandler.Point {

	// 截取屏幕图片
	srcMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	if srcMat == nil || srcMat.Empty() {
		return nil
	}
	defer srcMat.Recycle()

	// 如果 threshold 为 0，使用默认值 0.9
	if threshold <= 0 {
		threshold = 0.9
	}

	// 如果 count 为 0，使用默认值 1
	if count <= 0 {
		count = 1
	}

	// 如果 direction 为 0，使用默认值 1
	if direction <= 0 {
		direction = 1
	}

	// 调用 opencv 现成方法 FindMultiColor（本身支持 count 返回多个点）
	points := imageHandler.FindMultiColor(srcMat, firstColor, colorMulti, threshold, 0, 0, 0, 0, count, direction)
	if len(points) == 0 {
		return nil
	}

	// 所有匹配结果转为绝对坐标
	result := make([]imageHandler.Point, 0, len(points))
	for _, p := range points {
		result = append(result, imageHandler.Point{X: p.X + x1, Y: p.Y + y1})
	}
	return result
}

// FindMultiColor 在指定区域内多点找色
// 参数:
//   - x1, y1, x2, y2: 检测区域坐标
//   - firstColor: 第一个点的颜色，例如 "#D3AC14-0x2C2614"
//   - colorMulti: 多点颜色配置字符串，例如 "6|1|0x969696-0x000010,1|12|0x969696,-4|0|0x969696"
//   - threshold: 颜色相似度阈值，范围 0.0-1.0，默认 0.9
//   - count: 寻找的个数，默认 1
//   - direction: 寻找的方向 1~8，默认 1
//
// 返回值: (x, y, isFind) - 找到返回坐标和true，未找到返回 (-1, -1, false)
func FindMultiColor(x1, y1, x2, y2 int, firstColor, colorMulti string, threshold float64, count, direction int, cacheArgs ...interface{}) (int, int, bool) {
	points := findMultiColorPoints(x1, y1, x2, y2, firstColor, colorMulti, threshold, count, direction, cacheArgs...)
	if len(points) == 0 {
		return -1, -1, false
	}
	return points[0].X, points[0].Y, true
}

// FindMultiColorAll 在指定区域内多点找色，返回全部匹配点（坐标已转为绝对坐标）
// 内部复用 findMultiColorPoints，仅透传 opencv 现成方法的多点能力
func FindMultiColorAll(x1, y1, x2, y2 int, firstColor, colorMulti string, threshold float64, count, direction int, cacheArgs ...interface{}) []imageHandler.Point {
	return findMultiColorPoints(x1, y1, x2, y2, firstColor, colorMulti, threshold, count, direction, cacheArgs...)
}

// ── 以下三个 = 上面"多点找色"的【扩展版】：参数/行为/返回值完全一致，
//
//	唯一区别是底层 opencv 方法换成 imageHandler.FindMultiColorEx
//	（增强型多点找色，支持带绝对坐标的首点写法 "50|674|#RRGGBB-0x101010"）。
//	点库预设字段：【多点找色Ex】（区别于【多点找色】）。
func findMultiColorExPoints(x1, y1, x2, y2 int, firstColor, colorMulti string, threshold float64, count, direction int, cacheArgs ...interface{}) []imageHandler.Point {

	// 截取屏幕图片
	srcMat := captureScreenMat(x1, y1, x2, y2, cacheArgs...)
	if srcMat == nil || srcMat.Empty() {
		return nil
	}
	defer srcMat.Recycle()

	// 如果 threshold 为 0，使用默认值 0.9
	if threshold <= 0 {
		threshold = 0.9
	}

	// 如果 count 为 0，使用默认值 1
	if count <= 0 {
		count = 1
	}

	// 如果 direction 为 0，使用默认值 1
	if direction <= 0 {
		direction = 1
	}

	// ★ 唯一差异：FindMultiColorEx（其余与 findMultiColorPoints 逐行一致）
	points := imageHandler.FindMultiColorEx(srcMat, firstColor, colorMulti, threshold, 0, 0, 0, 0, count, direction)
	if len(points) == 0 {
		return nil
	}

	// 所有匹配结果转为绝对坐标
	result := make([]imageHandler.Point, 0, len(points))
	for _, p := range points {
		result = append(result, imageHandler.Point{X: p.X + x1, Y: p.Y + y1})
	}
	return result
}

// FindMultiColorEx 在指定区域内多点找色（扩展版）
// 参数与返回值同 FindMultiColor，仅底层调用 FindMultiColorEx。
//
// 返回值: (x, y, isFind) - 找到返回坐标和true，未找到返回 (-1, -1, false)
func FindMultiColorEx(x1, y1, x2, y2 int, firstColor, colorMulti string, threshold float64, count, direction int, cacheArgs ...interface{}) (int, int, bool) {
	points := findMultiColorExPoints(x1, y1, x2, y2, firstColor, colorMulti, threshold, count, direction, cacheArgs...)
	if len(points) == 0 {
		return -1, -1, false
	}
	return points[0].X, points[0].Y, true
}

// FindMultiColorExAll 在指定区域内多点找色（扩展版），返回全部匹配点（坐标已转为绝对坐标）
func FindMultiColorExAll(x1, y1, x2, y2 int, firstColor, colorMulti string, threshold float64, count, direction int, cacheArgs ...interface{}) []imageHandler.Point {
	return findMultiColorExPoints(x1, y1, x2, y2, firstColor, colorMulti, threshold, count, direction, cacheArgs...)
}
