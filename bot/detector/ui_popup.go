package detector

import (
	"app/core"
	"time"
)

// UIPopupDetector 界面异常/弹窗检测器
// 配置位于 assets/config/detectors.json → popup 节点
//
// 支持三种检测类型:
//   - 图片(默认): 模板匹配，对应 assets/scene_场景名.png 截图
//   - 文字: OCR 文字识别
//   - 找色: 多点找色/单点找色
//
// high_priority: 高频弹窗先查，减少全量扫描开销
// threshold:      匹配阈值，0.0~1.0，默认 0.85
type UIPopupDetector struct {
	interval     time.Duration
	enabled      bool
	threshold    float64
	highPriority []PopupScene
	scenes       []PopupScene
}

// NewUIPopupDetector 从 JSON 配置创建
func NewUIPopupDetector(cfg *PopupConfig) *UIPopupDetector {
	if cfg == nil {
		cfg = &PopupConfig{IntervalSec: 5, Sim: 0.85}
	}
	interval := time.Duration(cfg.IntervalSec) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}
	threshold := cfg.Sim
	if threshold <= 0 {
		threshold = 0.85
	}
	return &UIPopupDetector{
		interval:     interval,
		enabled:      cfg.Enabled,
		threshold:    threshold,
		highPriority: []PopupScene(cfg.HighPriority),
		scenes:       []PopupScene(cfg.Scenes),
	}
}

func (d *UIPopupDetector) Name() string            { return "界面异常检测" }
func (d *UIPopupDetector) Interval() time.Duration { return d.interval }
func (d *UIPopupDetector) SetEnabled(v bool)       { d.enabled = v }

func (d *UIPopupDetector) Check() bool {
	if !d.enabled {
		return false
	}

	// 高优场景先查
	for _, scene := range d.highPriority {
		if matched := d.checkScene(scene); matched {
			core.LogWithTimef("[Detector] 界面异常(高优): %s", scene.Name)
			return true
		}
	}

	// 全量场景
	for _, scene := range d.scenes {
		if matched := d.checkScene(scene); matched {
			core.LogWithTimef("[Detector] 界面异常: %s", scene.Name)
			return true
		}
	}

	return false
}

// checkScene 统一走识别预设，跟任务引擎一样
func (d *UIPopupDetector) checkScene(scene PopupScene) bool {
	if scene.RecogRef == "" {
		core.LogWithTimef("[Detector] 场景 %s 未配置识别引用，跳过", scene.Name)
		return false
	}
	return core.Exec(scene.RecogRef).Found
}
