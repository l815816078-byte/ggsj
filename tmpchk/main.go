package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type 落花商店物品 struct {
	物品名 string `json:"物品名"`
	数量   int    `json:"数量"`
}

type scriptConfig struct {
	药品下限 int            `json:"药品下限"`
	落花商店 []落花商店物品 `json:"落花商店"`
}

func main() {
	data, err := os.ReadFile("assets/脚本配置.json")
	if err != nil {
		fmt.Println("读取失败:", err)
		return
	}
	fmt.Printf("字节数=%d\n", len(data))
	var c scriptConfig
	if err := json.Unmarshal(data, &c); err != nil {
		fmt.Println("解析失败:", err)
		return
	}
	fmt.Printf("药品下限=%d 落花商店=%d 个\n", c.药品下限, len(c.落花商店))
	for i, it := range c.落花商店 {
		fmt.Printf("  [%d] 物品名=%q 数量=%d\n", i, it.物品名, it.数量)
	}
	// 顺带看看顶层 key
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	for k := range m {
		fmt.Printf("顶层key: %q\n", k)
	}
}
