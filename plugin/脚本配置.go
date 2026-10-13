package plugin

// 脚本配置集中加载 —— 把几个"常用要改的配置"(药品下限、落花商店)搬到 assets/脚本配置.json,
// 改配置不用再动 Go 代码(改完重新编译即可, 和 recogs.json / tasks 一样是打进程序的)。
//
// 覆盖规则: 这些配置【只认这份 JSON】, Go 里没有兜底默认值 ——
//   - JSON 里配了 → 直接用
//   - 没配 / 空数组 / 文件不存在 / 解析失败 → 该项不生效(数组空=不做, 数量 0=不限),
//     并在加载时打日志提示(不会因为配置缺失直接崩)。
//
// 注意: 下面给 encoding/json 跨包解析用的 struct 字段名【必须是导出名(首字符大写)】,
// JSON 键名本身靠 `json:"中文"` tag 映射, 所以字段名可随意, 但绝不能用中文首字符
// (中文字符不属于 Unicode 大写字母, Go 视为未导出, json 包会静默丢弃该字段)。
//
// 新增一项的步骤: ①脚本配置.json 加 { "_说明":..., "值":... }
// ②下面 scriptConfig 结构加对应字段 ③加一个取值函数 ④Go 里 var X = scriptXxx()

import (
	"encoding/json"

	"app/assets"
	"app/core"
)

// scriptConfigFile 脚本配置.json 的文件名(嵌在 assets 里)
const scriptConfigFile = "脚本配置.json"

// scriptConfig 脚本配置.json 的整体结构。
// 每项都是 { "_说明": ..., "值": ... } 的形式: "_说明" 只给人看, 代码只读 "值"。
type scriptConfig struct {
	DrugLimit struct {
		Val int `json:"值"`
	} `json:"药品下限"`

	FlowerShop struct {
		Val []scriptFlowerShopItem `json:"值"`
	} `json:"落花商店"`
}

// scriptFlowerShopItem 落花商店要买的一个物品
type scriptFlowerShopItem struct {
	Name string `json:"物品名"`
	Qty  int    `json:"数量"`
}

// scriptCfg 全局脚本配置(包级变量, 最先初始化)
var scriptCfg = loadScriptConfig()

// loadScriptConfig 读 assets/脚本配置.json。
// 读不到/解析失败 → 返回零值(各项为空), 调用方保留 Go 里的默认值。
func loadScriptConfig() scriptConfig {
	var cfg scriptConfig
	data, err := assets.ScriptConfig.ReadFile(scriptConfigFile)
	if err != nil {
		core.LogWithTimef("[脚本配置] 读取 %s 失败 → 用 Go 里的默认配置: %v", scriptConfigFile, err)
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		core.LogWithTimef("[脚本配置] %s 解析失败 → 相关配置不生效(数组空=不做, 数量0=不限): %v",
			scriptConfigFile, err)
		return scriptConfig{}
	}
	return cfg
}

// ───── 药品下限 ─────

// scriptDrugLimit 药品下限(只认 JSON): 没配/0 → 调用方用默认 50
func scriptDrugLimit() int {
	return scriptCfg.DrugLimit.Val
}

// ───── 落花商店 ─────

// scriptFlowerShop 落花商店物品名单(只认 JSON): 没配/空 = 不买任何东西
func scriptFlowerShop() []scriptFlowerShopItem {
	c := scriptCfg.FlowerShop.Val
	if len(c) == 0 {
		core.LogWithTimef("[脚本配置] '落花商店' 没配 → 落花商店不买任何物品")
	}
	return c
}
