package plugin

import (
	"encoding/json"
	"os"
	"sync"

	"app/assets"
	"app/core"
)

// scriptConfigFile 内置脚本配置文件名（go:embed 于 assets.ScriptConfig）
const scriptConfigFile = "脚本配置.json"

// 真机脚本配置文件路径（中控下发配置写在这里，优先于内置，重启后生效）
const 本机脚本配置 = "/sdcard/脚本配置.json"

// 落花商店物品 落花商店要买的物品：物品名（= recogs 预设名）+ 购买数量（默认显示 1，配置 N 就再点 N-1 次【加1】）
type 落花商店物品 struct {
	物品名 string `json:"物品名"`
	数量   int    `json:"数量"`
}

// scriptConfig 脚本配置（内置 json 读取）
type scriptConfig struct {
	// 药品下限：当前药品数量低于该值就买药（默认 50）
	药品下限 int `json:"药品下限"`
	// 落花商店：要买的物品列表（按顺序买）
	落花商店 []落花商店物品 `json:"落花商店"`
}

var (
	scriptConfigOnce sync.Once
	scriptConfigData scriptConfig
)

// loadScriptConfig 读取【脚本配置.json】（只读一次，带默认值兜底）。
// 优先读真机 /sdcard/脚本配置.json（中控下发的配置写在这里），读不到再退回内置 embed 的那份。
func loadScriptConfig() scriptConfig {
	scriptConfigOnce.Do(func() {
		scriptConfigData = scriptConfig{
			药品下限: 50, // 默认值
		}
		if data, err := os.ReadFile(本机脚本配置); err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, &scriptConfigData); err != nil {
				core.LogWithTimef("[脚本配置] %s 解析失败: %v → 改用内置配置", 本机脚本配置, err)
			} else {
				core.LogWithTimef("[脚本配置] 已加载 %s（%d 字节，落花商店 %d 个物品）",
					本机脚本配置, len(data), len(scriptConfigData.落花商店))
			}
		} else if data, err := assets.ScriptConfig.ReadFile(scriptConfigFile); err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, &scriptConfigData); err != nil {
				core.LogWithTimef("[脚本配置] 内置配置解析失败: %v", err)
			} else {
				core.LogWithTimef("[脚本配置] 已加载内置配置（%d 字节，落花商店 %d 个物品）",
					len(data), len(scriptConfigData.落花商店))
			}
		} else {
			core.LogWithTimef("[脚本配置] %s 与内置配置都读不到 → 只用默认值", 本机脚本配置)
		}
		// 兜底：json 里没配或配成 0 时用默认 50
		if scriptConfigData.药品下限 <= 0 {
			scriptConfigData.药品下限 = 50
		}
	})
	return scriptConfigData
}

// GetScriptConfig 获取脚本配置
func GetScriptConfig() scriptConfig {
	return loadScriptConfig()
}
