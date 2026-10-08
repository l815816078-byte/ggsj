package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"app/assets"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	data map[string]interface{} `json:"-"`
}

// Get 获取配置值，支持多层路径如 "app.packages.kit"
func (c *ConfigManager) Get(path string) interface{} {
	if c.data == nil {
		return nil
	}

	keys := strings.Split(path, ".")
	current := c.data

	for i, key := range keys {
		if i == len(keys)-1 {
			// 最后一层，直接返回值
			return current[key]
		}

		// 中间层，需要继续向下查找
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}

// GetString 获取字符串类型的配置值
func (c *ConfigManager) GetString(path string) string {
	if value := c.Get(path); value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt 获取整数类型的配置值
func (c *ConfigManager) GetInt(path string) int {
	if value := c.Get(path); value != nil {
		if num, ok := value.(float64); ok {
			return int(num)
		}
	}
	return 0
}

// GetBool 获取布尔类型的配置值
func (c *ConfigManager) GetBool(path string) bool {
	if value := c.Get(path); value != nil {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// GetMap 获取map类型的配置值
func (c *ConfigManager) GetMap(path string) map[string]interface{} {
	if value := c.Get(path); value != nil {
		if m, ok := value.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// GetArray 获取数组类型的配置值
func (c *ConfigManager) GetArray(path string) []interface{} {
	if value := c.Get(path); value != nil {
		if arr, ok := value.([]interface{}); ok {
			return arr
		}
	}
	return nil
}

// Exists 检查路径是否存在
func (c *ConfigManager) Exists(path string) bool {
	return c.Get(path) != nil
}

// UserInfo 用户信息结构体
type UserInfo struct {
	UserName string `json:"username"`
	PassWord int    `json:"password"`
	Seq      bool   `json:"seq"`
}

// DataStore 全局数据存储结构体
type DataStore struct {
	UserInfo UserInfo `json:"user_info"`

	// 保留一个通用的map用于临时数据
	TempData map[string]interface{} `json:"temp_data"`
}

// NewDataStore 创建新的数据存储
func NewDataStore() *DataStore {
	return &DataStore{
		UserInfo: UserInfo{},
		TempData: make(map[string]interface{}),
	}
}

// SetTempData 设置临时数据
func (d *DataStore) SetTempData(key string, value interface{}) {
	d.TempData[key] = value
}

// GetTempData 获取临时数据
func (d *DataStore) GetTempData(key string) interface{} {
	return d.TempData[key]
}

// ClearTempData 清空临时数据
func (d *DataStore) ClearTempData() {
	d.TempData = make(map[string]interface{})
}

// 全局数据存储实例
var GlobalStore *DataStore

// 全局配置实例
var Config *ConfigManager

// 识别条件预设库（assets/recogs.json），任务步骤/弹窗/插件通过 识别引用 共用
var Recogs map[string]map[string]interface{}

// init 函数在包加载时自动执行
func init() {
	Config = &ConfigManager{}
	GlobalStore = NewDataStore()
	loadConfig()
	loadRecogs()
}

// loadConfig 加载配置文件（嵌入资源）
func loadConfig() {
	data, err := assets.DefaultConfig.ReadFile("default.json")
	if err != nil {
		fmt.Println("嵌入式配置读取失败:", err)
		return
	}

	var configData map[string]interface{}
	if err := json.Unmarshal(data, &configData); err != nil {
		return
	}

	Config.data = configData
	fmt.Println("配置文件加载成功（嵌入）")
}

// GetAppPackage 保持向后兼容的函数
func GetAppPackage(appName string) string {
	return Config.GetString("app_packages." + appName)
}

// loadRecogs 加载识别条件预设库（assets/recogs.json）
func loadRecogs() {
	data, err := assets.Recogs.ReadFile("recogs.json")
	if err != nil {
		fmt.Println("识别预设库读取失败:", err)
		return
	}

	var recogData map[string]map[string]interface{}
	if err := json.Unmarshal(data, &recogData); err != nil {
		fmt.Println("识别预设库解析失败:", err)
		return
	}

	Recogs = recogData
	fmt.Printf("识别预设库加载成功: %d 个预设\n", len(Recogs))
}

// GetRecog 获取识别预设（原始 map，插件/引擎可按需读取字段）
func GetRecog(name string) map[string]interface{} {
	if cfg, ok := Recogs[name]; ok {
		return cfg
	}
	return nil
}

// GetRecogString 获取预设内字符串字段
func GetRecogString(name, key string) string {
	if cfg := GetRecog(name); cfg != nil {
		if s, ok := cfg[key].(string); ok {
			return s
		}
	}
	return ""
}

// GetRecogFloat 获取预设内浮点字段
func GetRecogFloat(name, key string) float64 {
	if cfg := GetRecog(name); cfg != nil {
		if f, ok := cfg[key].(float64); ok {
			return f
		}
	}
	return 0
}

// GetRecogInt 获取预设内整数字段
func GetRecogInt(name, key string) int {
	if cfg := GetRecog(name); cfg != nil {
		if f, ok := cfg[key].(float64); ok {
			return int(f)
		}
	}
	return 0
}

// GetRecogArray 获取预设内数组字段
func GetRecogArray(name, key string) []interface{} {
	if cfg := GetRecog(name); cfg != nil {
		if arr, ok := cfg[key].([]interface{}); ok {
			return arr
		}
	}
	return nil
}

// SplitMultiColor 将完整多点找色串拆分为首色和剩余部分
// 格式: "#FF0000-0x101010,1|0|#00FF00-0x101010,..." → ("#FF0000-0x101010", "1|0|#00FF00-0x101010,...")
func SplitMultiColor(raw string) (string, string) {
	idx := strings.Index(raw, ",")
	if idx < 0 {
		return raw, ""
	}
	return raw[:idx], raw[idx+1:]
}
