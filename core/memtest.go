package core

// Memory 库（libmem.so）连通性测试。
//
// 用途：验证项目能否正常加载 libmem.so 并读写内存。
// 放在 core 包而不是 test 包：test 包依赖 imageHandler（CGO 库 libranyiyu.so），
// 在部分环境下无法编译，会连累同目录下的文件一起报"未解析的引用"。
//
// 注意：
//  1. 需要 root（或内核后端 + KPM），普通权限下打开进程会失败。
//  2. 只做「打开自身进程 → 读自己的变量 → 写回并校验」这种自证式测试，
//     不依赖任何外部游戏进程；能跑通即说明 CGO 链接、so 加载、读写链路都正常。

import (
	"fmt"
	"os"
	"unsafe"

	"Memory"
)

// MemSelfTest 对自身进程做一次读内存/写内存回环测试。
// 依次尝试三种读后端（syscall / kernel / procfs），定位哪种在当前设备可用。
// 返回 true 表示 Memory 库已正确接入且链路可用。
func MemSelfTest() bool {
	fmt.Println("[MemTest] ===== Memory 库连通性测试开始 =====")

	pid := os.Getpid()
	if pid <= 0 {
		fmt.Println("[MemTest] 获取自身 PID 失败")
		return false
	}
	fmt.Printf("[MemTest] 自身 PID = %d\n", pid)

	// 内核后端需要先初始化 KPM
	kernelOK := Memory.InitKernel()
	fmt.Printf("[MemTest] InitKernel = %v\n", kernelOK)

	client, err := Memory.New(pid)
	if err != nil {
		fmt.Printf("[MemTest] 打开进程失败: %v\n", err)
		return false
	}
	fmt.Println("[MemTest] 打开进程成功")

	// 用一个 Go 变量地址做回环
	var probe int32 = 123456
	addr := uint64(uintptr(unsafe.Pointer(&probe)))
	fmt.Printf("[MemTest] 测试地址 = 0x%X，原值 = %d\n", addr, probe)

	backends := []struct {
		name string
		v    Memory.ReadBackend
	}{
		{"syscall", Memory.BackendSyscall},
		{"kernel", Memory.BackendKernel},
		{"procfs", Memory.BackendProcfs},
	}

	for _, b := range backends {
		fmt.Printf("[MemTest] --- 尝试后端 %s ---\n", b.name)
		client.SetReadBackend(b.v)

		before, err := client.ReadInt32(addr)
		if err != nil {
			fmt.Printf("[MemTest] [%s] 读失败: %v\n", b.name, err)
			continue
		}
		fmt.Printf("[MemTest] [%s] 读成功，值 = %d（期望 123456）\n", b.name, before)

		// 写回环
		if err := client.WriteInt32(addr, 654321); err != nil {
			fmt.Printf("[MemTest] [%s] 写失败: %v\n", b.name, err)
			continue
		}
		after, err := client.ReadInt32(addr)
		if err != nil {
			fmt.Printf("[MemTest] [%s] 写入后回读失败: %v\n", b.name, err)
			continue
		}
		fmt.Printf("[MemTest] [%s] 写入后回读 = %d（期望 654321）\n", b.name, after)

		if after == 654321 {
			fmt.Printf("[MemTest] ===== 通过：后端 %s 可读可写 =====\n", b.name)
			probe = 123456 // 复原
			return true
		}
	}

	fmt.Println("[MemTest] ===== 三种后端均失败 =====")
	return false
}
