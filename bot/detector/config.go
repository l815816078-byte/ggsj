package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"app/assets"
)

// DetectorConfig 所有检测器的 JSON 配置
type DetectorConfig struct {
	Popups     PopupConfig   `json:"弹窗检测"`
	TaskRotate PopupConfig   `json:"任务轮转"`
	Freeze     FreezeConfig  `json:"冻结检测"`
	Idle       IdleConfig    `json:"空闲检测"`
	Process    ProcessConfig `json:"进程检测"`
}

type PopupConfig struct {
	Enabled      bool           `json:"启用"`
	IntervalSec  int            `json:"检测间隔秒"`
	Sim          float64        `json:"相似度"`
	HighPriority PopupSceneList `json:"高优先级场景"`
	Scenes       PopupSceneList `json:"场景列表"`
}

// PopupScene 单个弹窗场景的检测配置
// 类型支持: "图片"(模板匹配) / "文字"(OCR) / "找色"(多点找色或单点找色)
type PopupScene struct {
	Name       string   `json:"名称"`   // 场景名称（图片类型时对应 assets/scene_名称.png）
	RecogRef   string   `json:"识别引用"` // 识别条件预设引用（assets/recogs.json），转换时自动合并
	Type       string   `json:"类型"`   // "图片"(默认) / "文字" / "找色"
	Text       string   `json:"识别文字"` // OCR 文字（类型=文字 时使用）
	ColorMulti []string `json:"多点找色"` // 多点找色格式（类型=找色 时使用；数组，单个也写成一个元素）
	Colors     []string `json:"颜色数组"` // 单点找色颜色数组（类型=找色 且 多点找色为空时使用）
	Region     [4]int   `json:"截图区域"` // 截图区域 [x1,y1,x2,y2]，全为0=全屏
	Sim        float64  `json:"相似度"`  // 该场景独立相似度（覆盖全局阈值），与 recogs.json 预设字段同名
	FontName   string   `json:"字库名"`  // 字库名（类型=文字 时使用，默认 "library"）
	Count      int      `json:"找色数量"` // 找色数量（类型=找色 时使用）
	Direction  int      `json:"找色方向"` // 找色方向 1~8（类型=找色 时使用）
	// 点击目标优先级: 点击区域 > 点击偏移 > 检测坐标。三样都不写就点检测到的位置
	ClickArea   [4]int `json:"点击区域"` // 绝对点击区域 [x1,y1,x2,y2]，不为空时忽略其他，固定点这个区域
	ClickOffset [2]int `json:"点击偏移"` // 相对检测位置的位移 [dx,dy]。如 OCR 找到文字在 (500,300)，偏移 [0,200] 则点 (500,500)
	WaitAfter   int    `json:"等待秒"`  // 点击后等待，秒；0=不等（引擎用毫秒，转换时乘1000）
	ClickDelay  *int   `json:"点击延迟秒"` // 点击动作点击后的固定延迟，秒；不写=nil=走默认延迟(随机1.5~2秒)；写0=点击后零延迟
	Plugin     bool   `json:"插件模式"` // 插件模式：走插件处理，忽略识别/动作字段；名称即插件注册名（如"弹窗_判断等级复活"）
	Action     string `json:"动作"`   // 点击(默认) / 滑动。空=点击，旧配置不写即沿用点击
	SwipeStart [2]int `json:"滑动起点"` // 绝对滑动起点 [x,y]（动作=滑动 时使用）
	SwipeEnd   [2]int `json:"滑动终点"` // 绝对滑动终点 [x,y]（动作=滑动 时使用）
	SwipeDelta [2]int `json:"滑动偏移"` // 相对识别位置的滑动偏移量 [dx,dy]（动作=滑动 时使用）
	SwipeDur   int    `json:"滑动时长"` // 滑动持续时间（毫秒）
}

// PopupSceneList 场景列表，兼容字符串数组和对象数组两种 JSON 格式
type PopupSceneList []PopupScene

// UnmarshalJSON 支持旧格式 ["场景名"] 和新格式 [{"名称":"场景名","类型":"图片"}]
func (l *PopupSceneList) UnmarshalJSON(data []byte) error {
	// 尝试新格式：PopupsScene 对象数组
	var scenes []PopupScene
	if err := json.Unmarshal(data, &scenes); err == nil {
		// 如果全是空对象（旧格式字符串被错误解析），回退字符串解析
		allEmpty := len(scenes) > 0
		for _, s := range scenes {
			if s.Name != "" || s.Type != "" || s.Text != "" || len(s.ColorMulti) > 0 {
				allEmpty = false
				break
			}
		}
		if !allEmpty {
			*l = scenes
			return nil
		}
	}
	// 尝试旧格式：字符串数组
	var names []string
	if err := json.Unmarshal(data, &names); err == nil {
		for _, name := range names {
			*l = append(*l, PopupScene{Name: name, Type: "图片"})
		}
		return nil
	}
	return fmt.Errorf("场景列表格式错误: 需要字符串数组或场景对象数组")
}

type FreezeConfig struct {
	Enabled     bool     `json:"启用"`
	IntervalSec int      `json:"检测间隔秒"`
	TimeoutSec  int      `json:"超时秒"`
	Sim         float64  `json:"相似度"`
	CheckImages []string `json:"检测图片"`
}

type IdleConfig struct {
	Enabled      bool `json:"启用"`
	IntervalSec  int  `json:"检测间隔秒"`
	ThresholdSec int  `json:"空闲阈值秒"`
}

type ProcessConfig struct {
	Enabled     bool `json:"启用"`
	IntervalSec int  `json:"检测间隔秒"`
}

// LoadConfig 加载检测器配置，优先读文件系统，不存在则用嵌入默认
func LoadConfig() *DetectorConfig {
	// 尝试文件系统
	path := filepath.Dir(os.Args[0]) + "/assets/detectors.json"
	if data, err := os.ReadFile(path); err == nil {
		return parseConfig(data)
	}

	// 回退嵌入式
	data, err := assets.Detectors.ReadFile("detectors.json")
	if err != nil {
		fmt.Println("[Detector] 嵌入式配置读取失败:", err)
		return DefaultConfig()
	}
	return parseConfig(data)
}

func parseConfig(data []byte) *DetectorConfig {
	var cfg DetectorConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Println("[Detector] 配置解析失败:", err)
		return DefaultConfig()
	}
	return &cfg
}

// DefaultConfig 内置默认配置（兜底）
func DefaultConfig() *DetectorConfig {
	return &DetectorConfig{
		Popups: PopupConfig{Enabled: true, IntervalSec: 5, Sim: 0.85, HighPriority: PopupSceneList{}, Scenes: PopupSceneList{}},
		Freeze: FreezeConfig{Enabled: true, IntervalSec: 15, TimeoutSec: 300, Sim: 0.85, CheckImages: []string{}},
		Idle:   IdleConfig{Enabled: true, IntervalSec: 60, ThresholdSec: 900},
	}
}
