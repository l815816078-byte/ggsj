package ui

import (
	"fmt"
	"sync"

	"github.com/Dasongzi1366/AutoGo/device"
	"github.com/Dasongzi1366/AutoGo/imgui"
	"github.com/Dasongzi1366/AutoGo/motion"
	"github.com/Dasongzi1366/AutoGo/utils"
)

// ConfigFile 默认配置保存路径
const ConfigFile = "/sdcard/ui_config.json"

// UIConfig UI配置结构体，只保留卡密相关项（其余功能已注释，见文件末尾）
// 注意：字段名必须首字母大写（ASCII）才会被导出/参与 JSON 序列化，
// 中文驼峰字段不会被 encoding/json 处理，因此用拼音字段名 + 中文 json tag。
type UIConfig struct {
	KaMi      string `json:"卡密"`
	SheBeiHao string `json:"设备号"` // 设备号（同一卡密内唯一）
}

// config 全局配置实例，包含所有默认值
var config = UIConfig{
	KaMi:      "",
	SheBeiHao: "",
}

// uiTip 界面内红字提示（不能调引擎原生 HUD/Toast，会 SIGILL，只能界面内画）
var uiTip string

// uiLastCard / uiLastDevNo 记录上一次输入值，用于检测用户是否修改（修改则清空提示）
var (
	uiLastCard  string
	uiLastDevNo string
)

// showWindow 控制主窗口是否显示
var showWindow = true

// StartUI 启动UI界面，阻塞直到用户点击运行或退出
// 返回值: "run" 表示运行脚本, "close" 表示关闭退出
//
// 关于横屏启动崩溃（Illegal instruction）：
//   已确认（多轮日志）崩点与脚本代码无关——imgui.Init() 完成后约 170ms，
//   由 Init 触发的原生异步任务在横屏下崩溃（竖屏正常、竖屏转横屏也正常）。
//   因此这里在 Init 之前/之后做屏幕方向处理（见 ensurePortraitThenRestore）。
func StartUI() string {
	// 启动时读取已保存的配置
	_ = LoadConfig(ConfigFile, &config)

	uiDone = make(chan struct{})
	uiDoneOnce = &sync.Once{}
	uiResult = ""

	// 横屏启动验证：切到桌面后 App 自身方向是否真的变成竖屏
	if w, h, _, _ := device.GetDisplayInfo(0); w > h {
		fmt.Printf("[UI] 横屏启动(%dx%d)，按 Home 切到桌面，观察 App 方向是否改变\n", w, h)
		motion.Home(0)
		utils.Sleep(1500)
		w2, h2, _, _ := device.GetDisplayInfo(0)
		fmt.Printf("[UI] 切出后读到屏幕 %dx%d  ← 若仍是横屏说明 App 方向没变\n", w2, h2)
	}

	imgui.Init()

	imgui.Run(func() {
		PushGlobalUIStyle()
		defer PopGlobalUIStyle()

		width, height, _, _ := device.GetDisplayInfo(0)

		// 设置窗口默认大小，仅首次生效
		imgui.SetNextWindowSizeV(
			imgui.Vec2{X: UI_WindowWidth, Y: UI_WindowHeight},
			imgui.CondOnce,
		)

		// 设置窗口居中显示
		UI_设置窗口居中(width, height)

		// 窗口背景色
		imgui.PushStyleColorVec4(imgui.ColWindowBg, HexToVec4("#f8f9fa")) // 窗口背景
		defer imgui.PopStyleColor()

		if showWindow {
			// 标题文字颜色
			imgui.PushStyleColorVec4(imgui.ColText, HexToVec4("#ffffff"))
			opened := imgui.BeginV(
				"AutoGo 脚本控制面板",
				nil,
				UI_WindowFlagsNoMove|
					UI_WindowFlagsNoResize,
			)
			// BeginV 后标题已经绘制完成，立即恢复文字颜色
			imgui.PopStyleColor()
			if opened {
				imgui.SetWindowFontScale(UI_FontScale)
				UI_启用滑动滚动(1.0)
				RenderMainUI()
			}
			imgui.End()
		}

	})

	// 等待按钮通知退出
	<-uiDone
	return uiResult
}

// RenderMainUI 绘制主界面所有控件
func RenderMainUI() {
	顶部操作按钮区域()

	UI_设置标签页样式(1)
	//想要加标签页往这里添加即可（其余页已注释，见文件末尾）
	UI_设置标签栏(
		"主标签栏",
		[]string{"卡密"},
		[]func(){
			卡密页,
		},
	)
}

// 顶部操作按钮区域
func 顶部操作按钮区域() {
	UI_设置倒计时按钮("运行脚本", "#e8f5e9", "#1b5e20", "#43a047", 280, 56, 300, UI_运行脚本)
	imgui.SameLine()

	UI_设置按钮("退出脚本", "#ffebee", "#b71c1c", "#e53935", 200, 56, UI_退出脚本)
	imgui.SameLine()

	// 加时按钮暂时不用，先注释掉
	//UI_设置按钮("加时", "#e3f2fd", "#0d47a1", "#1e88e5", 70, 56, UI_重置运行倒计时)
	//imgui.SameLine()

	// 未暂停时显示「停止」，暂停后显示「继续」，点击统一走 UI_切换倒计时暂停
	if UI_RunCountdownPaused {
		UI_设置按钮("继续", "#e8f5e9", "#1b5e20", "#43a047", 70, 56, UI_切换倒计时暂停)
	} else {
		UI_设置按钮("停止", "#fff3e0", "#e65100", "#fb8c00", 70, 56, UI_切换倒计时暂停)
	}
}

// 卡密页
func 卡密页() {
	UI_设置文本("卡密设置", "卡密设置", "#ffffff", "#1565c0", false, 120, 45, 0, 8, 6)
	UI_换行线()

	UI_设置输入框("卡密", "请输入卡密", &config.KaMi, false, 300, 45, false)
	UI_间距()
	UI_设置输入框("设备号", "请输入设备号（同一卡密内唯一）", &config.SheBeiHao, false, 300, 45, false)

	// 用户修改任一输入后清空提示
	if config.KaMi != uiLastCard || config.SheBeiHao != uiLastDevNo {
		uiTip = ""
		uiLastCard = config.KaMi
		uiLastDevNo = config.SheBeiHao
	}

	// 校验提示：界面内画红字（不能调引擎原生 HUD/Toast，会 SIGILL）
	if uiTip != "" {
		UI_间距()
		UI_设置文本("校验提示", uiTip, "#ffffff", "#ff0000", false, 500, 45, 0, 8, 6)
	}
}

/*
==================== 以下为原有多页签内容（2026-09-26 起注释，仅保留卡密页） ====================

// 首页
func 首要页() {
	UI_设置文本("首页标题", "首页", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
}

// 功能页
func 功能页() {
	UI_设置文本("功能设置", "功能设置", "#ffffff", "#1565c0", false, 120, 36, 0.8, 8, 6)
	UI_换行线()

	UI_设置单选框("MainFunc", []string{"批量注册", "批量日常", "备用功能"}, &config.MainFunc, "水平", "#e8eaf6", "#283593", "#43a047", 0.9, 8, 6)

	UI_间距()

	UI_设置复选框("批量测试", "批量测试", true, &config.DailyFuncs1, true, "#e8eaf6", "#283593", "#43a047", 0.9, 8, 6)
	UI_设置复选框("A测试", "测试测试", true, &config.DailyFuncs2, true, "#e8eaf6", "#283593", "#43a047", 0.9, 8, 6)
	UI_设置复选框("啦啦啦", "啦啦啦啦", true, &config.DailyFuncs3, true, "#e8eaf6", "#283593", "#43a047", 0.9, 8, 6)

	UI_间距()

	UI_设置下拉框("模式选择", &config.ModeSelect, []string{"任务模式", "无尽模式", "卡顿模式"}, "#e8eaf6", "#283593", 200, 45, 0.9, 8, 6)
}

// 配置页
func 配置页() {
	UI_设置文本("配置管理", "配置管理", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
	UI_换行线()

	UI_设置下拉框("模式选择", &config.ConfigSelect, []string{"配置0", "配置1", "配置2"}, "#e8eaf6", "#283593", 200, 45, 0.9, 8, 6)

	imgui.SameLine()
	UI_设置按钮("保存配置", "#e3f2fd", "#0d47a1", "#1e88e5", 115, 45, UI_手动保存配置)

	imgui.SameLine()
	UI_设置按钮("读取配置", "#e3f2fd", "#0d47a1", "#1e88e5", 115, 45, UI_手动读取配置)

	UI_换行线()

	UI_设置按钮("打印当前配置", "#fff3e0", "#e65100", "#fb8c00", 160, 45, PrintCurrentConfig)

	UI_换行线()

	当前配置预览()
}

// 当前配置预览
func 当前配置预览() {
	UI_设置文本("配置预览", "配置预览", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
	imgui.Text(fmt.Sprintf("主功能设置: %d", config.MainFunc))
	imgui.Text(fmt.Sprintf("其他单选框: %d", config.OtherRadio))
	imgui.Text(fmt.Sprintf("模式选择: %d", config.ModeSelect))
	imgui.Text(fmt.Sprintf("账号文本: %s", config.AccountText))
	imgui.Text(fmt.Sprintf("备注输入: %s", config.RemarkText))
	imgui.Text(fmt.Sprintf("启用开关: %v", config.EnableSwitch))
}

// 账号页
func 账号页() {
	UI_设置文本("账号设置", "账号设置", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
	UI_换行线()

	UI_设置输入框("账号", "请输入账号", &config.AccountText, false, 300, 0, false)
	UI_设置输入框("备注", "请输入密码", &config.RemarkText, false, 300, 0, false)
	UI_设置输入框("我是多行输入", "我是多行输入", &config.我是多行输入, true, 300, 0, false)

	UI_设置输入框("数字", "我是数字", &config.我是数字, false, 295, 0, true)
	UI_设置开关("enable_switch", "启用功能", &config.EnableSwitch, true, 120, 45)
}

// 原卡密页（含折叠演示）
func 卡密页_原版() {
	UI_设置文本("卡密设置", "卡密设置", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
	UI_换行线()

	UI_设置输入框("卡密", "卡密", &config.卡密, false, 300, 45, false)

	UI_设置折叠("主功能选择", "#b2bae0", "#ffffff", false, 300, 45, 0.8, 8, 6, func() {
		UI_设置文本("折叠1", "折叠1", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
		UI_设置文本("折叠2", "折叠2", "#ffffff", "#1565c0", false, 120, 45, 0.8, 8, 6)
		UI_设置按钮("测试", "#fff3e0", "#e65100", "#fb8c00", 160, 45, PrintCurrentConfig)
		UI_设置按钮("测试", "#fff3e0", "#e65100", "#fb8c00", 160, 45, PrintCurrentConfig)
	},
	)
}

==================== 原多页签内容注释结束 ====================
*/
