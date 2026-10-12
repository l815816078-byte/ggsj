package plugin

import (
	"encoding/json"
	"sync"

	"app/assets"
)

// scriptConfigFile 内置脚本配置文件名（go:embed 于 assets.ScriptConfig）
const scriptConfigFile = "脚本配置.json"

// scriptConfig 脚本配置（内置 json 读取）
type scriptConfig struct {
	// 药品下限：当前药品数量低于该值就买药（默认 50）
	药品下限 int `json:"药品下限"`
}

var (
	scriptConfigOnce sync.Once
	scriptConfigData scriptConfig
)

// loadScriptConfig 读取内置【脚本配置.json】（只读一次，带默认值兜底）
func loadScriptConfig() scriptConfig {
	scriptConfigOnce.Do(func() {
		scriptConfigData = scriptConfig{
			药品下限: 50, // 默认值
		}
		if data, err := assets.ScriptConfig.ReadFile(scriptConfigFile); err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &scriptConfigData)
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
