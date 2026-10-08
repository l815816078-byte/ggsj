package core

// 重启测试：确认游戏重启后金币地址是否变化。
//
// 用法：
//   1) 游戏重启前，跑一次，记下日志里的候选地址
//   2) 重启游戏（完全退出再打开，金币值不变）
//   3) 再跑一次，对比两次的地址
//      - 地址相同 → 不用做指针链，直接读
//      - 地址不同 → 必须做指针链（场景 B）

import (
	"fmt"
	"time"

	"Memory"
)

// MemRestartTest 搜金币值并打印所有候选地址。
//   gold: 当前金币值
func MemRestartTest(gold int32) {
	fmt.Println("[MemTest2] ===== 重启测试：搜金币地址 =====")

	pid, err := MemInit("")
	if err != nil {
		fmt.Printf("[MemTest2] 初始化失败: %v\n", err)
		return
	}
	fmt.Printf("[MemTest2] 目标进程 PID = %d\n", pid)

	n, err := SearchBegin(Memory.SearchTypeI32, gold,
		Memory.RegionA, Memory.RegionOther, Memory.RegionCH)
	if err != nil {
		fmt.Printf("[MemTest2] 搜索失败: %v\n", err)
		return
	}
	fmt.Printf("[MemTest2] 金币 %d 搜到 %d 个候选：\n", gold, n)

	// 打印全部候选地址（方便和上次对比）
	for i, addr := range memCand {
		v, _ := readAsInt64(addr, memSearchTy)
		fmt.Printf("[MemTest2]   [%d] 0x%X = %d\n", i, addr, v)
	}

	fmt.Println("[MemTest2] ===== 结束 =====")
	fmt.Println("[MemTest2] 请记录以上地址；重启游戏后再跑一次，对比地址是否相同")
}

// MemWatchGold 搜金币值并持续监控所有候选地址的变化。
//   gold: 搜索用的初始金币值（游戏里当前的金币数）
//   rounds: 监控轮数（0 = 一直监控）
func MemWatchGold(gold int32, rounds int) {
	fmt.Println("[MemWatch] ===== 金币地址监控开始 =====")

	pid, err := MemInit("")
	if err != nil {
		fmt.Printf("[MemWatch] 初始化失败: %v\n", err)
		return
	}
	fmt.Printf("[MemWatch] 目标进程 PID = %d\n", pid)

	n, err := SearchBegin(Memory.SearchTypeI32, gold,
		Memory.RegionA, Memory.RegionOther, Memory.RegionCH)
	if err != nil {
		fmt.Printf("[MemWatch] 搜索失败: %v\n", err)
		return
	}
	fmt.Printf("[MemWatch] 初值 %d 搜到 %d 个候选地址\n", gold, n)
	if n == 0 {
		fmt.Println("[MemWatch] 没有候选，检查：金币值是否填对、游戏是否在前台")
		return
	}

	// 进入监控：每轮读所有候选，值变化的标记出来
	last := make(map[uint64]int64)
	fmt.Println("[MemWatch] 开始监控（每 3 秒刷新一次）：")
	fmt.Println("[MemWatch] 请在游戏里让金币变化，观察哪个地址跟着变")

	round := 0
	for {
		round++
		fmt.Printf("[MemWatch] ---------- 第 %d 轮 ----------\n", round)

		changed := 0
		for _, addr := range memCand {
			v, err := readAsInt64(addr, memSearchTy)
			if err != nil {
				fmt.Printf("[MemWatch]   0x%X  读取失败\n", addr)
				continue
			}
			old, seen := last[addr]
			mark := ""
			if seen && old != v {
				mark = fmt.Sprintf("  ← ★ 变化 %d → %d", old, v)
				changed++
			}
			fmt.Printf("[MemWatch]   0x%X = %d%s\n", addr, v, mark)
			last[addr] = v
		}

		if changed > 0 {
			fmt.Printf("[MemWatch] 本轮有 %d 个地址变化 —— 变了的就是金币所在\n", changed)
		} else {
			fmt.Println("[MemWatch] 本轮无地址变化（还没花金币？或游戏没在前台）")
		}

		if rounds > 0 && round >= rounds {
			break
		}
		time.Sleep(3 * time.Second)
	}

	fmt.Println("[MemWatch] ===== 监控结束 =====")
}
