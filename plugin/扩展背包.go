package plugin

import (
	"strings"
	"time"

	"app/core"
)

func init() {
	Register(&扩展背包{})
}

// 扩展背包 插件（供 assets/tasks/扩展背包.json 以"插件模式"调用）
//
// 需求: 按脚本配置里配的"要扩展的背包上限"扩背包:
//
//  1. 点"背包"(recogs 预设) 进背包;
//  2. 取文"背包数量"(预设已存在, 取文) → 原文格式 "12/45": 前面=已使用, 后面=背包上限;
//     上限 >= 配置上限 → 不用扩展, 直接结束; 否则继续;
//  3. 点"背包数量"取到的坐标 偏移 (-72,-11) → 打开扩展界面;
//  4. 循环(检测 + 点击):
//     - 检测到"钻石不够"(多点找色 #FF0C0B, 区域[671,434,705,464]) → 断开循环;
//     - 取文"背包扩展数量"(预设已存在) → 原文格式 "12/45": 前面=要扩展的背包数量, 后面=总扩展上限;
//     要扩展数量 >= 配置上限 → 断开循环;
//     - 否则点"加号"(字库, 区域[726,266,759,298]) → 要扩展数量 +1 → 继续循环;
//  5. 点"进行扩充"(字库, 区域[593,485,680,523]);
//  6. 循环查找"点击关闭"(最多10秒, 间隔1秒) → 点掉。
//
// 识别: 全部走 recogs 预设 core.Exec(背包/背包数量/钻石不够/背包扩展数量/加号/进行扩充/点击关闭),
// 区域与相似度都写在 assets/recogs.json 的对应预设里, 改识别不用动本文件。
type 扩展背包 struct{}

func (p *扩展背包) Name() string { return "扩展背包" }

const (
	bagNumOffX   = -72 // 点"背包数量"取到坐标的 X 偏移
	bagNumOffY   = -11 // 点"背包数量"取到坐标的 Y 偏移
	baMaxClicks  = 300 // 点"加号"次数上限(防识别异常时死循环狂点)
	baCloseSec   = 10  // 收尾找"点击关闭"最多等几秒
	baClickSleep = 600 // 点一次"加号"后等多少毫秒再取文
)

// bagExpandCooldownSec 扩展背包冷却: 1天(86400秒)。
// 野外挂机/打怪中杂事/任务进行中三处调 RunTask 时统一传它, 与任务 JSON 的"冷却秒"保持一致。
const bagExpandCooldownSec = 86400

func (p *扩展背包) Exec(_ *Context) (string, error) {
	limit := scriptBagExpandLimit()
	if limit <= 0 {
		core.LogWithTimef("[扩展背包] 脚本配置'扩展背包配置'没配/为0 → 不执行")
		return "no", nil
	}
	core.LogWithTimef("[扩展背包] 开始, 配置: 要扩展的背包上限=%d", limit)

	// 1) 背包
	b := core.Exec("背包")
	if !b.Found {
		core.LogWithTimef("[扩展背包] 没找到'背包'(不在主界面?) → 结束")
		return "no", nil
	}
	core.LogWithTimef("[扩展背包] 点击'背包' (%d,%d)", b.X, b.Y)
	core.RandomClickInAreaWithDelay(b.X, b.Y, b.X+5, b.Y+5)
	sleepWithStop(2000 * time.Millisecond)
	if IsStopped() {
		return "no", nil
	}

	// 2) 取"背包数量" = "已使用/上限"
	bn := core.Exec("背包数量")
	if !bn.Found || bn.Word == "" {
		core.LogWithTimef("[扩展背包] 取文'背包数量'失败 → 结束")
		return "no", nil
	}
	used, total, ok := parseSlashText(bn.Word)
	if !ok {
		core.LogWithTimef("[扩展背包] '背包数量'原文=%q 解析不出 'a/b' → 结束", bn.Word)
		return "no", nil
	}
	core.LogWithTimef("[扩展背包] 背包数量=%q → 已使用%d / 背包上限%d (配置上限%d)", bn.Word, used, total, limit)
	if total >= limit {
		core.LogWithTimef("[扩展背包] 背包上限%d 已达配置上限%d → 不需要扩展", total, limit)
		return "no", nil
	}

	// 3) 点"背包数量"坐标 偏移(-72,-11) → 打开扩展界面
	tx, ty := bn.X+bagNumOffX, bn.Y+bagNumOffY
	core.LogWithTimef("[扩展背包] '背包数量'(%d,%d) 偏移(%d,%d) → 点击 (%d,%d)", bn.X, bn.Y, bagNumOffX, bagNumOffY, tx, ty)
	core.RandomClickInAreaWithDelay(tx, ty, tx+5, ty+5)
	sleepWithStop(1500 * time.Millisecond)
	if IsStopped() {
		return "no", nil
	}

	// 4) 循环: 钻石不够 → 断; 要扩展数量达标 → 断; 否则点"加号"+1
	clicked := 0
	noDiamond := false
	for clicked < baMaxClicks {
		if IsStopped() {
			return "no", nil
		}
		if dz := core.Exec("钻石不够"); dz.Found {
			core.LogWithTimef("[扩展背包] 检测到'钻石不够' (%d,%d) → 断开循环", dz.X, dz.Y)
			noDiamond = true
			break
		}
		ex := core.Exec("背包扩展数量")
		if !ex.Found || ex.Word == "" {
			core.LogWithTimef("[扩展背包] 取文'背包扩展数量'失败 → 断开循环")
			break
		}
		want, capTotal, ok2 := parseSlashText(ex.Word)
		if !ok2 {
			core.LogWithTimef("[扩展背包] '背包扩展数量'原文=%q 解析不出 'a/b' → 断开循环", ex.Word)
			break
		}
		if want >= limit {
			core.LogWithTimef("[扩展背包] 要扩展数量%d 已达配置上限%d → 停止点加号", want, limit)
			break
		}
		jh := core.Exec("加号")
		if !jh.Found {
			core.LogWithTimef("[扩展背包] 没找到'加号'(区域[726,266,759,298]) → 断开循环")
			break
		}
		clicked++
		core.LogWithTimef("[扩展背包] 要扩展%d/目标%d (总扩展上限%d) → 点'加号' (%d,%d) 第%d次",
			want, limit, capTotal, jh.X, jh.Y, clicked)
		core.RandomClickInAreaWithDelay(jh.X, jh.Y, jh.X+5, jh.Y+5, 1000)
	}

	// 5) 进行扩充(钻石不够 → 跳过, 点了也失败)
	if noDiamond {
		core.LogWithTimef("[扩展背包] 钻石不够 → 跳过'进行扩充'")
	} else {
		kc := core.Exec("进行扩充")
		if !kc.Found {
			core.LogWithTimef("[扩展背包] 没找到'进行扩充'(区域[593,485,680,523]) → 跳过")
		} else {
			core.LogWithTimef("[扩展背包] 点击'进行扩充' (%d,%d) (本次共点加号%d次)", kc.X, kc.Y, clicked)
			core.RandomClickInAreaWithDelay(kc.X, kc.Y, kc.X+5, kc.Y+5)
			sleepWithStop(2000 * time.Millisecond)
		}
	}

	// 6) 循环查找"点击关闭" → 点掉
	deadline := time.Now().Add(baCloseSec * time.Second)
	for time.Now().Before(deadline) {
		if IsStopped() {
			break
		}
		if rc := core.Exec("点击关闭"); rc.Found {
			core.LogWithTimef("[扩展背包] 点击'点击关闭' (%d,%d)", rc.X, rc.Y)
			core.RandomClickInAreaWithDelay(rc.X, rc.Y, rc.X+5, rc.Y+5)
			break
		}
		core.RandomSleep(1000, 1000)
	}

	core.LogWithTimef("[扩展背包] 结束 (点加号%d次)", clicked)
	return "ok", nil
}

// parseSlashText 解析 "12/45" 形式的取文 → (斜杠前值, 斜杠后值, 是否解析成功)
// 复用同包 parseIntText(按名使用物品.go) 抠数字, 能容忍空格等杂字符。
func parseSlashText(s string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	a, okA := parseIntText(parts[0])
	b, okB := parseIntText(parts[1])
	if !okA || !okB {
		return 0, 0, false
	}
	return a, b, true
}
