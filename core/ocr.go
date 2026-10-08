package core

import (
	"sort"
	"strings"
	"sync"

	"github.com/Dasongzi1366/AutoGo/ppocr"
)

// ---- 飞浆OCR（PaddleOCR）区域取文 ----

var (
	ppocrOnce sync.Once
	ppocrIns  *ppocr.Ppocr
)

// normalizePpocrColor 把字库颜色格式转成 ppocr 颜色格式：
// "#FFFFFF-0x101010|#E1E1E1-0x101010" → "FFFFFF-101010|E1E1E1-101010"
// （只去掉 # 和每段前的 0x 前缀，其余原样保留）
func normalizePpocrColor(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "0x", "")
	return s
}

// getPpocr 懒加载 PaddleOCR 实例（v2 模型）
func getPpocr() *ppocr.Ppocr {
	ppocrOnce.Do(func() {
		ppocrIns = ppocr.New("v5")
	})
	return ppocrIns
}

// OcrPaddleText 用飞浆OCR识别区域内全部文字：
// 只保留相似度 >= threshold 的文字块，按从上到下、从左到右排序后拼接返回
// （Word 字段），坐标取第一个文字块中心点。
// colorStr 为可选颜色过滤：传了则按该颜色识别（格式 "#FFFFFF-0x101010|#E1E1E1-0x101010"，自动转 ppocr 格式）。
func OcrPaddleText(x1, y1, x2, y2 int, threshold float64, colorStr ...string) *RecogResult {
	p := getPpocr()
	if p == nil {
		LogWithTimef("[OCR] PaddleOCR 实例为空，识别失败")
		return &RecogResult{}
	}

	// 颜色过滤：可选参数，传了才启用，并转成 ppocr 的 "FFFFFF-101010" 格式
	color := ""
	if len(colorStr) > 0 {
		color = normalizePpocrColor(colorStr[0])
	}
	rs := p.Ocr(x1, y1, x2, y2, color, 0)
	if len(rs) == 0 {
		return &RecogResult{}
	}

	// 过滤低于阈值的文字块
	kept := make([]ppocr.Result, 0, len(rs))
	for _, r := range rs {
		if r.Score >= threshold {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return &RecogResult{}
	}

	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Y != kept[j].Y {
			return kept[i].Y < kept[j].Y
		}
		return kept[i].X < kept[j].X
	})

	words := make([]string, 0, len(kept))
	for _, r := range kept {
		words = append(words, r.Label)
	}

	first := kept[0]
	cx, cy := first.CenterX, first.CenterY
	if cx == 0 {
		cx = first.X + first.Width/2
	}
	if cy == 0 {
		cy = first.Y + first.Height/2
	}
	return &RecogResult{X: cx, Y: cy, Sim: first.Score, Word: strings.Join(words, " "), Found: true, Type: "text"}
}
