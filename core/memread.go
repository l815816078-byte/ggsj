package core

// 内存数值搜索/读取工具（基于 libmem.so）。
//
// 用途：读取游戏进程里的数值（金币、等级、血量等）。
// 由于这些数据的地址每次启动都会变，必须先"搜索定位"，流程：
//
//	1) SearchBegin(初值)            第一次搜：搜出当前值对应的所有地址（可能几千个）
//	2) 游戏里让数值变化
//	3) SearchNext(旧值, 新值)       在上一批地址里再筛（反复几次直到地址很少）
//	4) SearchRead()                 读取最终地址上的当前值
//
// 后端固定用 procfs（syscall 在多数 root 环境被 SELinux 拦，kernel 需要 KPM）。

import (
	"fmt"

	"Memory"

	"github.com/Dasongzi1366/AutoGo/app"
)

// currentForegroundPackage 取当前前台应用包名（通常就是正在跑的游戏）
func currentForegroundPackage() string {
	return app.CurrentPackage()
}

// 搜索会话状态（单例，够用；多目标同时搜再改成 map）
var (
	memClient   *Memory.Client
	memCand     []uint64 // 当前候选地址
	memSearchTy Memory.SearchValueType
)

// searchTypeNames 给日志用的类型名
func searchTypeName(t Memory.SearchValueType) string { return string(t) }

// MemInit 打开目标进程（传空则取当前前台应用），并把读后端设为 procfs。
// 返回进程 PID。
func MemInit(pkg string) (int, error) {
	if pkg == "" {
		pkg = currentForegroundPackage()
	}
	fmt.Printf("[Mem] 目标包名 = %s\n", pkg)

	client, err := Memory.Open(pkg)
	if err != nil {
		return 0, fmt.Errorf("打开进程失败: %w", err)
	}
	client.SetReadBackend(Memory.BackendProcfs) // 本机实测只有 procfs 可用
	memClient = client

	// 顺便打印一下进程架构
	arch, _ := client.Architecture()
	fmt.Printf("[Mem] 已连接进程，架构 = %d 位\n", arch)

	// 拿 PID（日志用）
	pid, _ := Memory.GetPID(pkg)
	return pid, nil
}

// SearchBegin 第一次搜索：在指定区域里找所有等于 value 的地址。
// valueType 建议先用 SearchTypeI32（4 字节整数，最常用）。
// regions 传 nil 时默认搜 [A, Other, CH]（覆盖大部分游戏动态数据）。
func SearchBegin(valueType Memory.SearchValueType, value any, regions ...Memory.SearchRegion) (int, error) {
	if memClient == nil {
		return 0, fmt.Errorf("未初始化，请先调用 MemInit")
	}
	if len(regions) == 0 {
		regions = []Memory.SearchRegion{Memory.RegionA, Memory.RegionOther, Memory.RegionCH}
	}

	memSearchTy = valueType
	req := Memory.NewSearchRequest(Memory.EqualSearch(valueType, value)).
		WithRegions(regions...).
		WithMax(20000)

	fmt.Printf("[Mem] 首次搜索：类型=%s 值=%v 区域=%v\n", searchTypeName(valueType), value, regions)
	addrs, err := memClient.SearchFastWithRequest(req)
	if err != nil {
		return 0, fmt.Errorf("搜索失败: %w", err)
	}

	memCand = addrs
	fmt.Printf("[Mem] 首次搜索完成，候选地址 %d 个\n", len(addrs))
	printCandidates(10)
	return len(addrs), nil
}

// SearchNext 二次筛选：游戏里让数值变化后调一次，候选会快速收敛。
//
// oldValue 上一次的数值，newValue 变化后的数值。两条规则一起用：
//   - 现在 = newValue  → 留下（它跟着变了，是真数据）
//   - 现在 = oldValue  → 删掉（它没跟着变，是固定常量/无关数据）
//   - 其他值           → 删掉
//
// 只按"等于新值"筛的话，那些等于旧值的常量会一直赖在候选里筛不掉。
func SearchNext(oldValue, newValue any) (int, error) {
	if memClient == nil {
		return 0, fmt.Errorf("未初始化，请先调用 MemInit")
	}
	if len(memCand) == 0 {
		return 0, fmt.Errorf("没有候选地址，请先 SearchBegin")
	}

	kept := make([]uint64, 0, len(memCand))
	unchanged := 0 // 仍是旧值（固定常量）的数量
	for _, addr := range memCand {
		v, err := readAsInt64(addr, memSearchTy)
		if err != nil {
			continue
		}
		if matchValue(v, memSearchTy, newValue) {
			kept = append(kept, addr)
		} else if matchValue(v, memSearchTy, oldValue) {
			unchanged++
		}
	}

	fmt.Printf("[Mem] 筛选：旧值=%v 新值=%v → 剩余 %d 个（剔除了 %d 个未变化的常量）\n",
		oldValue, newValue, len(kept), unchanged)
	memCand = kept
	printCandidates(10)
	return len(kept), nil
}

// AddCandidates 手动补充候选地址（比如把已知"重启不变"的地址加回来）
func AddCandidates(addrs ...uint64) {
	memCand = append(memCand, addrs...)
	fmt.Printf("[Mem] 已补充候选，当前 %d 个\n", len(memCand))
}

// SearchRead 读取当前所有候选地址上的值（通常是最后 1 个地址）。
func SearchRead() {
	if memClient == nil || len(memCand) == 0 {
		fmt.Println("[Mem] 无候选地址可读")
		return
	}
	fmt.Printf("[Mem] 读取 %d 个候选地址：\n", len(memCand))
	for i, addr := range memCand {
		if i >= 20 {
			fmt.Printf("[Mem]   ...（共 %d 个，只显示前 20）\n", len(memCand))
			break
		}
		v, err := readAsInt64(addr, memSearchTy)
		if err != nil {
			fmt.Printf("[Mem]   0x%X  读取失败: %v\n", addr, err)
			continue
		}
		fmt.Printf("[Mem]   0x%X = %d\n", addr, v)
	}
}

// SearchCount 返回当前候选地址数量
func SearchCount() int { return len(memCand) }

// SearchReset 清空候选（开始新的搜索目标时调）
func SearchReset() {
	memCand = nil
	fmt.Println("[Mem] 已清空搜索结果")
}

// ────────────────────────── 内部工具 ──────────────────────────

// readAsInt64 按类型读出一个值并统一转成 int64 便于比较
func readAsInt64(addr uint64, t Memory.SearchValueType) (int64, error) {
	switch t {
	case Memory.SearchTypeI8:
		v, err := memClient.ReadInt8(addr)
		return int64(v), err
	case Memory.SearchTypeI16:
		v, err := memClient.ReadInt16(addr)
		return int64(v), err
	case Memory.SearchTypeI32:
		v, err := memClient.ReadInt32(addr)
		return int64(v), err
	case Memory.SearchTypeI64:
		v, err := memClient.ReadInt64(addr)
		return v, err
	case Memory.SearchTypeUI8:
		v, err := memClient.ReadUint8(addr)
		return int64(v), err
	case Memory.SearchTypeUI16:
		v, err := memClient.ReadUint16(addr)
		return int64(v), err
	case Memory.SearchTypeUI32:
		v, err := memClient.ReadUint32(addr)
		return int64(v), err
	case Memory.SearchTypeUI64:
		v, err := memClient.ReadUint64(addr)
		return int64(v), err
	default:
		v, err := memClient.ReadInt32(addr)
		return int64(v), err
	}
}

// matchValue 比较读到的值与目标值（统一按 int64 比）
func matchValue(got int64, _ Memory.SearchValueType, want any) bool {
	switch w := want.(type) {
	case int:
		return got == int64(w)
	case int32:
		return got == int64(w)
	case int64:
		return got == w
	case uint32:
		return got == int64(w)
	case uint64:
		return got == int64(w)
	default:
		return false
	}
}

func printCandidates(max int) {
	n := len(memCand)
	if n == 0 {
		return
	}
	if n < max {
		max = n
	}
	for i := 0; i < max; i++ {
		fmt.Printf("[Mem]   候选[%d] 0x%X\n", i, memCand[i])
	}
}
