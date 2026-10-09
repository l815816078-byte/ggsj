package detector

import (
	"app/core"
	"time"

	"github.com/Dasongzi1366/AutoGo/app"
	"github.com/Dasongzi1366/AutoGo/utils"
)

// ProcessDetector 游戏进程检测器
// 检查目标游戏是否在前台运行，不在则触发异常
type ProcessDetector struct {
	interval    time.Duration
	gamePackage string
	kitPackage  string
	enabled     bool
}

// NewProcessDetector 创建进程检测器
func NewProcessDetector(enabled bool, interval time.Duration) *ProcessDetector {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ProcessDetector{
		interval:    interval,
		// 游戏包名统一从 app_packages.name 读取（default.json 配置）
		gamePackage: core.Config.GetString("app_packages.name"),
		kitPackage:  core.Config.GetString("app_packages.kit"),
		enabled:     enabled,
	}
}

func (d *ProcessDetector) Name() string            { return "进程检测" }
func (d *ProcessDetector) Interval() time.Duration { return d.interval }

// SetEnabled 启停检测（如日常休眠时关闭）
func (d *ProcessDetector) SetEnabled(v bool) { d.enabled = v }

func (d *ProcessDetector) Check() bool {
	if !d.enabled {
		return false
	}

	packageName := app.CurrentPackage()
	core.LogWithTimef("[Detector] 进程检测: 当前包=%s, 目标包=%s", packageName, d.gamePackage)

	if packageName != d.gamePackage {
		// 消抖：等 1 秒再查一次
		utils.Sleep(1000)
		packageName = app.CurrentPackage()
		if packageName != d.gamePackage {
			core.LogWithTimef("[Detector] 进程检测异常: %s 不在前台", d.gamePackage)
			return true
		}
	}
	return false
}
