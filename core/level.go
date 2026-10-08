package core

import (
	"strconv"
	"strings"
	"time"
)

// DefaultLevelCacheTTL 等级缓存默认过期时间。
// 等级是稳定业务数据（升级才变），缓存期内重复读取不截图识别；
// 各调用处可用 GetLevel 自行传参覆盖（ttl<=0 表示不缓存）。
const DefaultLevelCacheTTL = 600 * time.Second

// levelCachedLogAt 缓存命中日志节流时间（仅控制打印频率，不影响任何逻辑）
var levelCachedLogAt time.Time

// GetLevel 读取角色等级（返回 *int，识别不到绝不产生数字）：
//  1. 有缓存先走缓存：缓存命中直接返回（不截图识别）；
//  2. 缓存未命中直接 Exec("等级") 真识别（本游戏没有战力图标，不做任何前置门控），并从 Word 提取数字解析；
//  3. 识别成功才写缓存；识别不到/解析不出数字绝不写缓存 → 缓存里只有有效等级值，
//     下次调用时缓存未命中，继续真识别，直到识别到为止。
//
// 返回值：识别到 → (*int, true)，解引用即等级；识别不到 → (nil, false)，
// 不产生 0 之类的数字。调用方必须用 ok 判断，ok=false 时不得做任何基于等级的处理。
// ttl<=0 表示不缓存（每次真识别，识别成功也不写缓存）。
func GetLevel(ttl time.Duration) (*int, bool) {
	// 内部委托通用"取文+缓存"函数，不带任何前置门控
	return GetCachedTextInt("等级", ttl)
}

// RefreshLevel 刷新等级缓存（识别到才更新，识别不到保留旧缓存）：
// 直接真识别一次"等级"，识别成功用新值覆盖缓存（TTL 300s）；
// 识别失败/解析不出数字 → 不动已有缓存（旧值继续有效），避免"先删后识"把有效缓存弄丢。
// 用于"点击进入主线/剧情任务"等等级可能已变化的时刻，避免后续按等级判断的逻辑（如任务剧情买药）使用旧等级。
// 返回值同 GetLevel：识别到 → (*int, true)；识别不到 → (nil, false)。
func RefreshLevel() (*int, bool) {
	r := execOrBackup("等级")
	if !r.Found {
		return nil, false
	}
	n, ok := parseLeadingInt(r.Word)
	if !ok {
		LogWithTimef("[等级] 识别到文字但解析不出数字: %q", r.Word)
		return nil, false
	}
	cp := *r
	CacheSet("recog:等级", &cp, DefaultLevelCacheTTL)
	LogWithTimef("[等级] 当前等级 = %d (点击任务后刷新)", n)
	ReportField("等级")
	return &n, true
}

// RefreshTextInt 刷新指定"取文"识别（等级/战斗力等稳定数值字段）的缓存：
// 直接真识别一次 name，识别成功用新值覆盖缓存（TTL 300s）；
// 识别失败/解析不出数字 → 不动已有缓存（旧值继续有效），避免"先删后识"把有效缓存弄丢。
// 返回值同 GetCachedTextInt：识别到 → (*int, true)；识别不到 → (nil, false)。
func RefreshTextInt(name string) (*int, bool) {
	r := execOrBackup(name)
	if !r.Found {
		return nil, false
	}
	n, ok := parseLeadingInt(r.Word)
	if !ok {
		if name == "等级" {
			LogWithTimef("[等级] 识别到文字但解析不出数字: %q", r.Word)
		}
		return nil, false
	}
	cp := *r
	CacheSet("recog:"+name, &cp, DefaultLevelCacheTTL)
	LogWithTimef("[%s] 当前%s = %d (刷新)", name, name, n)
	ReportField(name)
	return &n, true
}

// execOrBackup 取文识别：先 Exec(name)，未命中则重试 name+"_备用"（如 等级→等级_备用）。
// 只有存在"_备用"预设的名字才会真正生效；其余名字重试后返回未命中，行为不变。
// 对 name="等级" 额外打识别日志（主识别是否命中、备用是否命中/均未命中），便于排查识别不到的问题。
func execOrBackup(name string) *RecogResult {
	r := Exec(name)
	if r.Found {
		if name == "等级" {
			LogWithTimef("[等级] 主识别'等级'命中 sim=%.2f", r.Sim)
		}
		return r
	}
	if name == "等级" {
		LogWithTimef("[等级] 主识别'等级'未命中 → 尝试'等级_备用'")
	}
	b := Exec(name + "_备用")
	if name == "等级" {
		if b.Found {
			LogWithTimef("[等级] 备用'等级_备用'命中 sim=%.2f", b.Sim)
		} else {
			LogWithTimef("[等级] 主识别'等级'与'等级_备用'均未识别到")
		}
	}
	return b
}

// GetCachedTextInt 通用"取文+缓存+数字提取"（等级/战斗力等稳定数值字段共用，逻辑同 GetLevel）：
//  1. 有缓存先走缓存：缓存命中直接返回（key="recog:"+name，与 ExecCached 同一 key，命中不截图识别）；
//  2. 缓存未命中：若传了 gates，先逐个检测前置门控，任一未命中直接返回失败；不传 gates 则直接识别；
//  3. Exec(name) 取文，parseLeadingInt 提取首个连续数字；
//  4. 识别成功才写缓存；识别不到/解析不出数字绝不写缓存 → 缓存里只有有效值，
//     下次调用缓存未命中，继续真识别，直到识别到为止。
//
// 返回值：识别到 → (*int, true)，解引用即数值；识别不到 → (nil, false)，
// 不产生 0 之类的数字。调用方必须用 ok 判断，ok=false 时不得做任何基于数值的处理。
// ttl<=0 表示不缓存（每次真识别，识别成功也不写缓存）。
func GetCachedTextInt(name string, ttl time.Duration, gates ...string) (*int, bool) {
	key := "recog:" + name
	// 1) 有缓存先走缓存（与 ExecCached 同一 key，缓存命中时不做任何识别）
	if ttl > 0 {
		if v, ok := CacheGet(key); ok {
			if r, ok := v.(*RecogResult); ok && r != nil {
				if n, ok2 := parseLeadingInt(r.Word); ok2 {
					// 缓存命中日志保留（真假都可见），但 60 秒最多打一条，避免调度 tick 每 2 秒刷屏
					if time.Since(levelCachedLogAt) >= 60*time.Second {
						levelCachedLogAt = time.Now()
						LogWithTimef("[%s] 当前%s = %d (缓存)", name, name, n)
					}
					return &n, true
				}
			}
		}
	}
	// 2) 缓存未命中 → 先逐个检测前置门控，检测到才真识别
	for _, g := range gates {
		if !Exec(g).Found {
			return nil, false
		}
	}
	r := execOrBackup(name)
	if !r.Found {
		return nil, false
	}
	n, ok := parseLeadingInt(r.Word)
	if !ok {
		if name == "等级" {
			LogWithTimef("[等级] 识别到文字但解析不出数字: %q", r.Word)
		}
		return nil, false
	}
	// 3) 识别成功才写缓存；识别失败绝不写缓存（避免下次命中失败结果而不再识别）
	if ttl > 0 {
		cp := *r
		CacheSet(key, &cp, ttl)
	}
	LogWithTimef("[%s] 当前%s = %d", name, name, n)
	// 中控上报挂钩：识别成功即通知（内部按字段 + 3 秒节流，等级/战力等稳定值不会刷屏）
	ReportField(name)
	return &n, true
}

// parseLeadingInt 从 OCR 文本中提取首个连续数字串并转 int。
// 兼容 "59级"、"8级"、"Lv 24" 等形态，以及带千位分隔符的战力 "102,344"（半角/全角逗号，后跟数字才算分隔符）。
func parseLeadingInt(s string) (int, bool) {
	// 全角逗号统一成半角，便于千位分隔符处理
	s = strings.ReplaceAll(s, "，", ",")
	start, i := -1, 0
	for ; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			if start < 0 {
				start = i
			}
		} else if start >= 0 && c == ',' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			// 千位分隔符且后跟数字 → 视为数字的一部分，跳过
			continue
		} else if start >= 0 {
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	// 去掉千位分隔符再转 int（"102,344" → 102344）
	digits := strings.ReplaceAll(s[start:i], ",", "")
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return n, true
}
