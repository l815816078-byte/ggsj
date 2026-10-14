package zt

import (
	"sync/atomic"
	"time"

	"app/core"
)

// ══════════════════════════════════════════════════════════════════════════
// 基础 —— 对应老脚本 BaseClass.lua
//
// 老脚本 BaseClass.lua 提供四个原语：
//	_找色(arr)              定点/多点找色，返回是否命中
//	_多点找色(arr,fw)       区域内找他，返回 x,y
//	_表找色点击(t, d)       扫整张表，命中就点（一轮点完所有命中的）
//	_持续找色(arr,time)     timeout 内循环找
//
// 这里对应：
//	识别(name, 区域...)      识别一次，返回 x,y,是否命中
//	查找点击延时(name,区域,延时) 找 + 点 + 延时（延时可不传，默认 1000）
//	扫弹窗(名单)             扫一遍名单，命中就处理（点击/滑动，不 break）
//	持续找色(name, timeout)  timeout 内循环找
//
// ★ 本版脱离老引擎：不 import app/plugin，停止标志自带。
// ★ 找色不走 core.Exec（那是喂 JSON 的），直接用点库的 Go 结构 + 调 core 底层函数。
// ══════════════════════════════════════════════════════════════════════════

// 停止标志（脱离老引擎后自带；老脚本对应"脚本被停止/退出"）
var 停止 atomic.Bool

// 设停止 供外部（信号处理等）置位
func 设停止() { 停止.Store(true) }

// 已停止 各长循环的检查点
func 已停止() bool { return 停止.Load() }

// 识别（= 老脚本 `_多点找色` + core.Exec）：按名字从点库取配置识别一次。
// 命中返回 (x, y, true)；区域可选，传 [x1,y1,x2,y2] 时覆盖点库的"区域"（= 老脚本的 Scope）。
//
// 多区域：点库配了多块区域时逐个试，第一个命中就返回（= core.Exec 的多区域逻辑）。
// 识别类型按字段自动选，顺序同 core.ExecWithConfig：
//
//	多点找色 → 模板图片 → 透明图 → 取文 → 取文字库 → 找字 → 颜色数组
//
// 本轮有命中 本轮是否有过识别命中（主流程用它做"卡死检测"：连续多轮没命中就重置）
var 本轮有命中 = false

func 识别(名 string, 区域 ...int) (int, int, bool) {
	p, ok := 特征表[名]
	if !ok {
		// ★ 这是【配置缺失】不是"没找到"：重试多少次都不可能命中。
		//   调用方若把它当"没找到"去滑动/重来 → 死循环刷屏（去地图() 已在开头守卫）。
		core.LogWithTimef("[识别][当前任务:%s] ★★ 点库里没有【%s】—— 配置缺失，请往 点库.go 的 特征表 补这条预设", 当前任务, 名)
		return 0, 0, false
	}

	// 区域：调用处传了就覆盖（单区域），否则用点库配的（可能多块）
	if len(区域) == 4 {
		p.区域 = [][4]int{{区域[0], 区域[1], 区域[2], 区域[3]}}
	}
	if len(p.区域) == 0 {
		p.区域 = [][4]int{{0, 0, 0, 0}} // 没配区域 → 走 core 默认
	}

	// 多区域：逐个区域识别，第一个命中就返回
	for _, 区 := range p.区域 {
		x, y, ok := 区域识别(p, 区[0], 区[1], 区[2], 区[3])
		if ok {
			本轮有命中 = true
			return x, y, true
		}
	}
	return 0, 0, false
}

// 区域识别 在给定区域内识别一次（= core.ExecWithConfig 的全部识别类型）
func 区域识别(p 预设, x1, y1, x2, y2 int) (int, int, bool) {
	相似度 := p.相似度

	// ① 多点找色（无块边界：命中点 + 偏移）
	if len(p.多点找色) > 0 {
		if 相似度 <= 0 {
			相似度 = 0.9
		}
		个数 := p.寻找个数
		if 个数 <= 0 {
			个数 = 1
		}
		for _, raw := range p.多点找色 {
			first, rest := core.SplitMultiColor(raw)
			if x, y, ok := core.FindMultiColor(x1, y1, x2, y2, first, rest, 相似度, 个数, 1); ok {
				return x + p.偏移[0], y + p.偏移[1], true
			}
		}
		return 0, 0, false
	}

	// ② 模板图片（= FindImgFast / FindImgFastAll）
	if len(p.模板图片) > 0 {
		sim := float32(相似度)
		if sim <= 0 {
			sim = 0.9
		}
		for _, img := range p.模板图片 {
			if p.中坐标 {
				ms := core.FindImgFastAll(x1, y1, x2, y2, img, p.严格模式, sim, 1)
				if len(ms) > 0 && ms[0].Sim >= sim {
					return (ms[0].X + p.偏移[0] + ms[0].X2) / 2, (ms[0].Y + p.偏移[1] + ms[0].Y2) / 2, true
				}
				continue
			}
			if x, y, s := core.FindImgFast(x1, y1, x2, y2, img, p.严格模式, sim); s >= sim {
				return x, y, true
			}
		}
		return 0, 0, false
	}

	// ③ 透明图
	if len(p.透明图) > 0 {
		sim := float32(相似度)
		if sim <= 0 {
			sim = 0.95
		}
		for _, img := range p.透明图 {
			ms := core.FindTranslucentImgAll(x1, y1, x2, y2, img, sim, 1)
			if len(ms) > 0 {
				if p.中坐标 {
					return (ms[0].X + p.偏移[0] + ms[0].X2) / 2, (ms[0].Y + p.偏移[1] + ms[0].Y2) / 2, true
				}
				return ms[0].X, ms[0].Y, true
			}
		}
		return 0, 0, false
	}

	// ④ 取文（paddle OCR 读区域文字）
	if p.取文 {
		if 相似度 <= 0 {
			相似度 = 0.5
		}
		r := core.OcrPaddleText(x1, y1, x2, y2, 相似度, p.颜色信息)
		if !r.Found {
			return 0, 0, false
		}
		return r.X, r.Y, true
	}

	// ⑤ 取文字库（字库识字读文）
	if p.取文字库 {
		if 相似度 <= 0 {
			相似度 = 0.9
		}
		字库 := p.字库名
		if 字库 == "" {
			字库 = "library"
		}
		x, y, _, _ := core.GetWords(x1, y1, x2, y2, p.颜色信息, 相似度, p.颜色信息 != "", 字库)
		if x < 0 || y < 0 {
			return 0, 0, false
		}
		return x, y, true
	}

	// ⑥ 找字（识别文字）
	if p.识别文字 != "" {
		if 相似度 <= 0 {
			相似度 = 0.85
		}
		字库 := p.字库名
		if 字库 == "" {
			字库 = "library"
		}
		if p.中坐标 {
			ms := core.FindStrAll(x1, y1, x2, y2, p.识别文字, 相似度, 字库, 1)
			if len(ms) == 0 {
				return 0, 0, false
			}
			m := ms[0]
			return (m.Left + p.偏移[0] + m.Right) / 2, (m.Top + p.偏移[1] + m.Bottom) / 2, true
		}
		x, y, _, _ := core.FindStr(x1, y1, x2, y2, p.识别文字, 相似度, 字库)
		if x <= 0 || y <= 0 {
			return 0, 0, false
		}
		return x + p.偏移[0], y + p.偏移[1], true
	}

	// ⑦ 颜色数组（单色比色）
	if len(p.颜色数组) > 0 {
		if 相似度 <= 0 {
			相似度 = 0.9
		}
		个数 := p.寻找个数
		if 个数 <= 0 {
			个数 = 1
		}
		方向 := p.找色方向
		if 方向 <= 0 {
			方向 = 1
		}
		for _, clr := range p.颜色数组 {
			if x, y, ok := core.FindColor(x1, y1, x2, y2, clr, 相似度, 个数, 方向); ok {
				return x, y, true
			}
		}
		return 0, 0, false
	}

	return 0, 0, false
}

// 找色文字 取文字库读出的文字（药品数量这类"读数"用）
func 找色文字(名 string, 区域 ...int) (string, bool) {
	p, ok := 特征表[名]
	if !ok || !p.取文字库 {
		return "", false
	}
	if len(区域) == 4 {
		p.区域 = [][4]int{{区域[0], 区域[1], 区域[2], 区域[3]}}
	}
	if len(p.区域) == 0 {
		p.区域 = [][4]int{{0, 0, 0, 0}}
	}
	相似度 := p.相似度
	if 相似度 <= 0 {
		相似度 = 0.9
	}
	字库 := p.字库名
	if 字库 == "" {
		字库 = "library"
	}
	区 := p.区域[0]
	_, _, _, info := core.GetWords(区[0], 区[1], 区[2], 区[3], p.颜色信息, 相似度, p.颜色信息 != "", 字库)
	return info, info != ""
}

// 取文 paddle OCR 读区域里的文字（"战斗力"这类取文型读数用）
// 返回读到的文字；读不到返回 "" , false
func 取文(名 string, 区域 ...int) (string, bool) {
	p, ok := 特征表[名]
	if !ok {
		return "", false
	}
	if len(区域) == 4 {
		p.区域 = [][4]int{{区域[0], 区域[1], 区域[2], 区域[3]}}
	}
	if len(p.区域) == 0 {
		return "", false
	}
	相似度 := p.相似度
	if 相似度 <= 0 {
		相似度 = 0.5
	}
	区 := p.区域[0]
	r := core.OcrPaddleText(区[0], 区[1], 区[2], 区[3], 相似度, p.颜色信息)
	if !r.Found {
		return "", false
	}
	return r.Word, true
}

// 查找点击延时：查找（识别）→ 命中就点（点击自带延时）。
// 一次识别、一次点击，不会重复识别。命中返回 true，未命中返回 false（不点）。
//
// 参数（区域和延时都可省略，按个数推导）：
//
//	名            点库预设名
//	区域/延时...   4 个数 = 区域 [x1,y1,x2,y2]；第 5 个数 = 点击延时（毫秒）
//	              区域不传 = 用点库自带；延时不传 = 用 core 默认（2000~2500）
//
// 调法：
//
//	查找点击延时("地图")                       // 点库区域 + 默认延时
//	查找点击延时("地点", 79,108,333,720)        // 指定区域 + 默认延时
//	查找点击延时("绿色按钮", 6000)              // 点库区域 + 点完等 6 秒
//	查找点击延时("地点", 79,108,333,720, 6000)  // 指定区域 + 点完等 6 秒
func 查找点击延时(名 string, 区域延时 ...int) bool {
	var 区域, 延时 []int
	if len(区域延时) >= 4 {
		区域 = 区域延时[:4]
		延时 = 区域延时[4:]
	} else {
		延时 = 区域延时
	}
	x, y, ok := 识别(名, 区域...)
	if !ok {
		return false
	}
	// 预设开了"点击用截图区域" → 不点命中点，改用区域当点击范围（照 plugin/wait.go:78-84）
	// 区域优先用调用处传入的，没有才用预设"区域"的第一个
	if p, 有 := 特征表[名]; 有 && p.点击用截图区域 {
		x1, y1, x2, y2 := 0, 0, 0, 0
		if len(区域) == 4 {
			x1, y1, x2, y2 = 区域[0], 区域[1], 区域[2], 区域[3]
		} else if len(p.区域) > 0 {
			x1, y1, x2, y2 = p.区域[0][0], p.区域[0][1], p.区域[0][2], p.区域[0][3]
		}
		if x2 > x1 && y2 > y1 {
			core.LogWithTimef("[查找点击延时] %s 用截图区域 %d,%d,%d,%d", 名, x1, y1, x2, y2)
			core.RandomClickInAreaWithDelay(x1, y1, x2, y2, 延时...)
			return true
		}
	}
	core.LogWithTimef("[查找点击延时] %s (%d,%d)", 名, x, y)
	core.RandomClickInAreaWithDelay(x, y, x+5, y+5, 延时...)
	return true
}

// 扫弹窗（= 老脚本 `_表找色点击`）：扫名单，**命中就处理**。
// 一轮把整张表扫完、所有命中的都点掉（BaseClass.lua:70-86 没有 break），这里照抄，不 break。
//
// 默认动作 = 点击；名字在 滑动表 里的 → 改成滑动（如"省电模式"）。
// 延时靠 `点击`（自带）和 `RandomSwipe` 的滑后延时参数，不在外面再加 sleep。
//
// 轮数：可选参数（不传 = 扫 1 遍）。
//
//	扫弹窗(名单)        → 扫 1 遍（老行为）
//	扫弹窗(名单, 3)     → 扫最多 3 遍（点掉一个又冒一个的连锁弹窗用）
//
// 返回是否命中过任何一个。
func 扫弹窗(名单 []string, 轮数 ...int) bool {
	轮 := 1
	if len(轮数) > 0 && 轮数[0] > 0 {
		轮 = 轮数[0]
	}
	命中 := false
	for i := 0; i < 轮; i++ {
		if 已停止() {
			return 命中
		}
		本轮命中 := false
		for _, 名 := range 名单 {
			if 已停止() {
				return 命中
			}
			// 挂机中不滑【省电模式】：滑掉它就中断挂机（挂机.go 的 挂机中）
			if 挂机中 && 名 == "省电模式" {
				continue
			}
			x, y, ok := 识别(名)
			if !ok {
				continue
			}
			if m, 是滑动 := 滑动表[名]; 是滑动 {
				core.LogWithTimef("[扫弹窗] %s 命中 → 滑动 (%d,%d)→(%d,%d)", 名, m[0], m[1], m[2], m[3])
				core.RandomSwipe(m[0], m[1], m[2], m[3], m[4], m[5], 2500)
			} else {
				core.LogWithTimef("[扫弹窗] %s 命中 (%d,%d) → 点击", 名, x, y)
				core.RandomClickInAreaWithDelay(x, y, x+5, y+5) // 自带延迟
			}
			命中 = true
			本轮命中 = true
		}
		// 这一轮一个都没命中 → 屏幕干净了，不用再扫
		if !本轮命中 {
			break
		}
	}
	return 命中
}

// 持续找色（= BaseClass `_持续找色`）：timeout 内循环找，命中一次立即返回 true。
// 区域可选。
func 持续找色(名 string, timeout time.Duration, 区域 ...int) bool {
	end := time.Now().Add(timeout)
	for {
		if 已停止() {
			return false
		}
		if _, _, ok := 识别(名, 区域...); ok {
			return true
		}
		if time.Now().After(end) {
			return false
		}
		core.RandomSleep(1000, 2000)
	}
}

// ── 多结果识别（"红点多个逐个点"用；照 core/recog.go 的 ExecAll 两条分支）─────
// 坐标 一个屏幕点（识别多个 的返回元素）
type 坐标 struct{ X, Y int }

// 识别多个 在指定区域内多结果识别，返回全部命中点（已加点库偏移）。
// 区域可选，传 [x1,y1,x2,y2] 时覆盖点库的"区域"。
//
// 只支持 多点找色 / 找字 两类（照 core/recog.go:343-418 的两条多结果分支）：
//
//	多点找色 → 逐个组合 FindMultiColorAll，累加全部命中点
//	找字      → 寻找个数 > 1 才走 FindStrAll 返回多个；<= 1 返回空（只要一个点用 识别()）
func 识别多个(名 string, 区域 ...int) []坐标 {
	p, ok := 特征表[名]
	if !ok {
		core.LogWithTimef("[识别][当前任务:%s] ★★ 点库里没有【%s】—— 配置缺失，请往 点库.go 的 特征表 补这条预设", 当前任务, 名)
		return nil
	}
	if len(区域) == 4 {
		p.区域 = [][4]int{{区域[0], 区域[1], 区域[2], 区域[3]}}
	}
	if len(p.区域) == 0 {
		p.区域 = [][4]int{{0, 0, 0, 0}}
	}
	个数 := p.寻找个数
	if 个数 <= 1 {
		return nil // 单点识别用 识别()；这里只返多个
	}
	相似度 := p.相似度
	var 点 []坐标
	for _, 区 := range p.区域 {
		// 多点找色：全部命中点（相似度默认 0.9，照 区域识别 ①）
		for _, raw := range p.多点找色 {
			sim := 相似度
			if sim <= 0 {
				sim = 0.9
			}
			first, rest := core.SplitMultiColor(raw)
			for _, q := range core.FindMultiColorAll(区[0], 区[1], 区[2], 区[3], first, rest, sim, 个数, 1) {
				点 = append(点, 坐标{q.X + p.偏移[0], q.Y + p.偏移[1]})
			}
		}
		// 找字：FindStrAll 多个（相似度默认 0.85，照 区域识别 ⑥；中坐标时与块右下角取中）
		if p.识别文字 != "" {
			sim := 相似度
			if sim <= 0 {
				sim = 0.85
			}
			字库 := p.字库名
			if 字库 == "" {
				字库 = "library"
			}
			for _, m := range core.FindStrAll(区[0], 区[1], 区[2], 区[3], p.识别文字, sim, 字库, 个数) {
				if p.中坐标 {
					点 = append(点, 坐标{(m.Left + p.偏移[0] + m.Right) / 2, (m.Top + p.偏移[1] + m.Bottom) / 2})
				} else {
					点 = append(点, 坐标{m.Left + p.偏移[0], m.Top + p.偏移[1]})
				}
			}
		}
	}
	return 点
}
