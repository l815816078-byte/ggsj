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
// 新增一项的步骤: ①脚本配置.json 加字段 ②下面 scriptConfig 结构加对应字段
// ③加一个取值函数 ④Go 里 var X = scriptXxx()

import (
	"encoding/json"

	"app/assets"
	"app/core"
)

// scriptConfigFile 脚本配置.json 的文件名(嵌在 assets 里)
const scriptConfigFile = "脚本配置.json"

// scriptConfig 脚本配置.json 的整体结构（字段名用导出名，JSON 键靠 tag 映射）
type scriptConfig struct {
	// 药品下限：当前药品数量低于该值就买药
	DrugLimit int `json:"药品下限"`

	// 副本：任务名列表，这些任务跑在副本里（挂机时不能清【退出副本】，否则会被踢出副本）
	Dungeons []string `json:"副本"`

	// 落花商店：要买的物品列表（按顺序买）
	FlowerShop []scriptFlowerShopItem `json:"落花商店"`
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
	return scriptCfg.DrugLimit
}

// ───── 副本 ─────

// isDungeonTask 判断任务是不是副本任务（任务名在"副本"名单里就算）
func isDungeonTask(taskName string) bool {
	for _, name := range scriptCfg.Dungeons {
		if name == taskName {
			return true
		}
	}
	return false
}

// ───── 落花商店 ─────

// scriptFlowerShop 落花商店物品名单(只认 JSON): 没配/空 = 不买任何东西
func scriptFlowerShop() []scriptFlowerShopItem {
	c := scriptCfg.FlowerShop
	if len(c) == 0 {
		core.LogWithTimef("[脚本配置] '落花商店' 没配 → 落花商店不买任何物品")
	}
	return c
}
