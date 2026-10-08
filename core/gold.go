package core

// 金币读取（基于已定位的固定地址）。
//
// 定位过程：搜值 + 重启对比，确认下列地址在游戏重启后不变（本机实测）。
// 注意：地址有效性依赖「游戏版本 + 设备环境」不变。若游戏更新或换设备，
//       需重新搜索定位，再改下面的常量。
//
// 读取策略：多个候选地址都读一遍，取第一个成功的；都失败说明地址失效。

import (
	"fmt"
)

// 金币候选地址（本机实测重启不变）。如失效，重新搜索后更新。
var goldAddrs = []uint64{
	0x77C2D4F75708,
	0x72116234,
}

// MemoryReady Memory 是否已初始化
func MemoryReady() bool { return memClient != nil }

// InitGoldReader 初始化金币读取：连接当前前台应用并设为 procfs 后端。
// 在 main 里调一次即可。
func InitGoldReader() error {
	_, err := MemInit("")
	return err
}

// GetGold 读取当前金币数。第二个返回值表示是否读取成功。
func GetGold() (int64, bool) {
	if memClient == nil {
		return 0, false
	}
	for _, addr := range goldAddrs {
		v, err := memClient.ReadInt32(addr)
		if err != nil {
			continue
		}
		// 合法金币应为非负；读到负数说明地址已被复用，跳过
		if v < 0 {
			continue
		}
		return int64(v), true
	}
	return 0, false
}

// DumpGold 打印各候选地址的当前值（调试用）
func DumpGold() {
	if memClient == nil {
		fmt.Println("[Gold] 未初始化")
		return
	}
	for i, addr := range goldAddrs {
		v, err := memClient.ReadInt32(addr)
		if err != nil {
			fmt.Printf("[Gold] 候选[%d] 0x%X 读取失败: %v\n", i, addr, err)
			continue
		}
		fmt.Printf("[Gold] 候选[%d] 0x%X = %d\n", i, addr, v)
	}
}
