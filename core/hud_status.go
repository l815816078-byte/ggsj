package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Dasongzi1366/AutoGo/device"
	"github.com/Dasongzi1366/AutoGo/hud"
)

// Version 全局脚本版本号，来自 assets/default.json 的 version 字段。
// 发布新版时改 assets/default.json 里的 version 值（重新编译生效）。
// 所有 HUD（主 HUD/热更 HUD）和热更新判断都读它，调用方无需传参。
var Version = "2.5" // 默认值兜底，init 里会用 JSON 值覆盖

// 全局 HUD 状态显示管理器
var (
	globalHUD      *hud.HUD
	hudMutex       sync.Mutex
	scriptVersion  string
	hudStatusColor = "#00FF00"
	hudStatusText  string
	hudLogs        []string // 详细日志行（黄色），如热更新过程日志
	hudProgress    string   // 任务超时进度行，如 "login 已运行 1/300 秒"
)

// init 从配置读取版本号（config.go 的 init 先执行，Config 已加载完成）。
func init() {
	if v := Config.GetString("version"); v != "" {
		Version = v
	}
	scriptVersion = "v" + Version
}

// screenLandscape 检测当前屏幕是否横屏。
// 游戏横屏运行时屏幕分辨率会旋转（如 1920x1080），HUD 坐标也要跟着换，
// 否则按竖屏坐标定位的 HUD 会显示在屏幕外/错位。
// 获取分辨率失败时默认按竖屏处理（保持原行为）。
func screenLandscape() bool {
	w, h, _, _ := device.GetDisplayInfo(0)
	if w > 0 && h > 0 {
		return w > h
	}
	return false
}

// InitHUD 初始化全局 HUD（在 main.go 中调用一次，无需传版本号）
func InitHUD() {
	hudStatusText = "已启动"
	globalHUD = hud.New()
	if screenLandscape() {
		globalHUD.SetPosition(199, 3, 552, 33) // 横屏：屏幕顶部横条
	} else {
		globalHUD.SetPosition(133, 1, 289, 30) // 竖屏：左上角
	}
	globalHUD.SetTextSize(25)
	globalHUD.SetBackgroundColor("#80000000") // 半透明黑色背景
	hudRender()
	globalHUD.Show()
}

// hudRender 用当前状态行 + 日志行 + 进度行重绘 HUD（调用方需持有 hudMutex）
func hudRender() {
	if globalHUD == nil {
		return
	}
	items := []hud.TextItem{
		{TextColor: hudStatusColor, Text: fmt.Sprintf("[%s] %s", scriptVersion, hudStatusText)},
	}
	for _, l := range hudLogs {
		items = append(items, hud.TextItem{TextColor: "#FFFF00", Text: l})
	}
	if hudProgress != "" {
		items = append(items, hud.TextItem{TextColor: "#FFFF00", Text: hudProgress})
	}
	globalHUD.SetText(items)
}

// SetStatus 设置 HUD 状态文字（绿色，支持格式化）
func SetStatus(format string, args ...interface{}) {
	status := format
	if len(args) > 0 {
		status = fmt.Sprintf(format, args...)
	}
	hudMutex.Lock()
	defer hudMutex.Unlock()
	hudStatusColor = "#00FF00"
	hudStatusText = status
	hudRender()
}

// SetStatusColor 设置 HUD 状态文字（自定义颜色，支持格式化）
func SetStatusColor(color string, format string, args ...interface{}) {
	status := format
	if len(args) > 0 {
		status = fmt.Sprintf(format, args...)
	}
	hudMutex.Lock()
	defer hudMutex.Unlock()
	hudStatusColor = color
	hudStatusText = status
	hudRender()
}

// SetHUDLogs 设置 HUD 详细日志行（黄色，显示在状态行下方）。
// 传 nil/空则清除日志行。最多保留 8 行，每行自动截断到 40 字符以内。
func SetHUDLogs(lines []string) {
	hudMutex.Lock()
	defer hudMutex.Unlock()
	hudLogs = hudLogs[:0]
	for _, l := range lines {
		if len(l) > 40 {
			l = l[:40]
		}
		hudLogs = append(hudLogs, l)
	}
	if len(hudLogs) > 8 {
		hudLogs = hudLogs[len(hudLogs)-8:]
	}
	hudRender()
}

// AppendHUDLog 在 HUD 追加一行详细日志（黄色），最多保留 8 行。
func AppendHUDLog(line string) {
	if len(line) > 40 {
		line = line[:40]
	}
	hudMutex.Lock()
	defer hudMutex.Unlock()
	hudLogs = append(hudLogs, line)
	if len(hudLogs) > 8 {
		hudLogs = hudLogs[len(hudLogs)-8:]
	}
	hudRender()
}

// SetTaskProgress 设置/更新 HUD 上的任务超时进度行（黄色第二行），只显示 "(1/300)"。
// timeout <= 0 时清除进度行（任务结束调用）。
func SetTaskProgress(name string, elapsed, timeout time.Duration) {
	hudMutex.Lock()
	defer hudMutex.Unlock()
	if timeout <= 0 {
		hudProgress = ""
	} else {
		e := int64(elapsed.Seconds())
		t := int64(timeout.Seconds())
		if t <= 0 {
			t = 1
		}
		hudProgress = fmt.Sprintf("(%d/%d)", e, t)
	}
	hudRender()
}

// ===== 热更新专用 HUD（不影响主 HUD） =====
// 热更新日志用独立 HUD：大字号、独立区域 (146,799,571,1262)。
// 引擎 SetText 文本内 \n 可换行，所以单实例 + \n 拼接即可实现多行。

var (
	updateHUD      *hud.HUD
	updateHUDMutex sync.Mutex
)

// InitUpdateHUD 初始化热更新专用 HUD（大字号，区域 146,799,571,1262）。
// 只影响热更新日志，不影响主 HUD 的状态/进度/日志显示。
func InitUpdateHUD() {
	updateHUDMutex.Lock()
	defer updateHUDMutex.Unlock()
	initUpdateHUDLocked()
}

// initUpdateHUDLocked 创建热更 HUD（幂等；调用方需持有 updateHUDMutex）
func initUpdateHUDLocked() {
	if updateHUD != nil {
		return
	}
	updateHUD = hud.New()
	// 左边界 50（比原 40 右移 10px，给文字留左内边距；hud 无 padding 接口只能靠坐标）
	if screenLandscape() {
		// 横屏：按用户给的坐标
		updateHUD.SetPosition(130, 41, 1133, 87)
	} else {
		// 竖屏：屏幕顶部
		updateHUD.SetPosition(50, 129, 682, 180)
	}
	// 区域固定 40,129,682,180：宽 642 / 高 51。
	// 字号 44：竖向留余量不裁切，横向也能容纳当前最长文案（约 18 字符）。
	updateHUD.SetTextSize(44)
	updateHUD.SetBackgroundColor("#FFFFFF") // 纯白背景
	updateHUD.Show()
}

// SetUpdateLogs 设置热更新日志（绿色，只显示一行：多行输入只取第一行）。
func SetUpdateLogs(text string) {
	updateHUDMutex.Lock()
	defer updateHUDMutex.Unlock()
	if updateHUD == nil {
		return
	}
	// 区域只有 51px 高，只显示一行；多行内容取第一行关键信息
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	updateHUD.SetText([]hud.TextItem{{TextColor: "#00CC00", Text: text}})
}

// SetUpdateLogsColor 用指定颜色在热更 HUD 上显示文本；HUD 未初始化时自动创建。
// 用于卡密等验证异常的红字提示（与热更失败共用同一块 HUD，用户能看到原因再处理）。
func SetUpdateLogsColor(color, text string) {
	updateHUDMutex.Lock()
	defer updateHUDMutex.Unlock()
	initUpdateHUDLocked()
	updateHUD.SetText([]hud.TextItem{{TextColor: color, Text: text}})
}

// DestroyUpdateHUD 销毁热更新专用 HUD（热更新流程结束、确认已是最新后调用）。
func DestroyUpdateHUD() {
	updateHUDMutex.Lock()
	defer updateHUDMutex.Unlock()
	if updateHUD != nil {
		updateHUD.Destroy()
		updateHUD = nil
	}
}
