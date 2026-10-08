package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Dasongzi1366/AutoGo/imgui"

	"app/cardManager/cardkey"
	"app/core"
)

// 窗口标志位常量
const (
	UI_WindowFlagsNoTitleBar        = 1 << 0
	UI_WindowFlagsNoResize          = 1 << 1
	UI_WindowFlagsNoMove            = 1 << 2
	UI_WindowFlagsNoScrollbar       = 1 << 3
	UI_WindowFlagsNoScrollWithMouse = 1 << 4
	UI_WindowFlagsNoCollapse        = 1 << 5
)

// UI_FontScale 用于设置整个窗口内文字的缩放比例。
// 1.0 表示默认大小；小于 1.0 表示缩小；大于 1.0 表示放大。
// 例如：0.8 表示文字缩小到默认的 80%。
var UI_FontScale float32 = 1.0

// UI_WindowWidth 用于设置 UI 主窗口的默认宽度。
// 通常配合 imgui.SetNextWindowSizeV 使用。
var UI_WindowWidth float32 = 700

// UI_WindowHeight 用于设置 UI 主窗口的默认高度。
// 通常配合 imgui.SetNextWindowSizeV 使用。
var UI_WindowHeight float32 = 700

// UI_ButtonWidth 用于设置按钮的默认固定宽度。
// 当 UI_ButtonAutoSize 为 false，并且 UI_ButtonFullWidth 为 false 时生效。
var UI_ButtonWidth float32 = 110

// UI_ButtonHeight 用于设置按钮的默认固定高度。
// 固定大小按钮和占满宽度按钮都会使用这个高度。
var UI_ButtonHeight float32 = 34

// UI_InputWidth 用于设置输入框、下拉框等控件的默认宽度。
// 当 UI_AutoItemWidth 为 false 时生效。
var UI_InputWidth float32 = 300

// UI_AutoItemWidth 控制输入框、下拉框等控件是否自动占满当前可用宽度。
// true：控件宽度自适应，通常占满当前行剩余宽度。
// false：控件使用 UI_InputWidth 指定的固定宽度。
var UI_AutoItemWidth bool = false

// UI_ButtonAutoSize 控制按钮是否根据文字内容自动调整大小。
// true：按钮大小根据文字内容自适应。
// false：按钮不按文字自适应，改用固定大小或占满宽度模式。
var UI_ButtonAutoSize bool = true

// UI_ButtonFullWidth 控制按钮是否占满当前行剩余宽度。
// true：按钮宽度占满当前可用区域，通常等效于宽度设置为 -1。
// false：按钮不强制占满宽度。
// 注意：如果 UI_ButtonFullWidth 为 true，优先级高于 UI_ButtonAutoSize。
var UI_ButtonFullWidth bool = false

// uiDone 用于通知 StartUI 函数退出阻塞
var uiDone chan struct{}

// uiDoneOnce 确保 uiDone 只被关闭一次
var uiDoneOnce *sync.Once

// uiResult UI 返回结果
// ""      表示没有动作
// "run"   表示点击了运行脚本
// "close" 表示点击了关闭 UI
var uiResult string

// RunConfig 用户点击「运行脚本」时保存的配置快照，供 main 或其他包读取
var RunConfig *UIConfig

// UI_运行脚本 校验卡密/设备号 → 联网校验卡密 → 通过才保存并通知主流程
func UI_运行脚本() {
	// 1) 本地非空校验：卡密、设备号都不能为空，不通过则界面红字提示、不继续
	config.KaMi = strings.TrimSpace(config.KaMi)
	config.SheBeiHao = strings.TrimSpace(config.SheBeiHao)
	if config.KaMi == "" {
		uiTip = "请输入卡密"
		return
	}
	if config.SheBeiHao == "" {
		uiTip = "请输入设备号"
		return
	}

	// 1.5) 提前把设备号注入 cardkey！
	// main.go 里的 readUIRunConfig() 要等 StartUI 返回后才执行，
	// 而下面 VerifyCard 会立刻发激活请求——不提前注入，请求里 device_no 就是空串。
	cardkey.SetDeviceNo(config.SheBeiHao)

	// 2) 联网校验卡密（同步，成功才关界面；失败保持界面停留 + 界面红字提示）
	if _, msg, ok := cardkey.VerifyCard(core.Config.GetString("license_server"), config.KaMi); !ok {
		uiTip = "卡密校验失败 " + msg
		return
	}
	uiTip = ""

	// 3) 保存配置，通知 StartUI 返回 run
	if err := SaveConfig(ConfigFile, &config); err != nil {
		fmt.Println("运行前保存配置失败:", err)
	} else {
		fmt.Println("运行前保存配置成功")
	}

	// 保存当前配置指针，供 main 或脚本逻辑读取
	RunConfig = &config

	// 通知 StartUI 返回 run
	UI_退出("run")
}

// UI_退出脚本 保存配置后关闭UI
func UI_退出脚本() {
	err := SaveConfig(ConfigFile, &config)
	if err != nil {
		fmt.Println("退出前保存配置失败:", err)
	} else {
		fmt.Println("退出前保存配置成功")
	}

	// 通知 StartUI 返回 close
	UI_退出("close")
}

// UI_退出 关闭imgui窗口并通知StartUI返回指定结果
func UI_退出(result string) {
	uiResult = result
	showWindow = false
	// 关闭 imgui 窗口/渲染循环
	imgui.Close()
	// 通知 StartUI() 可以返回
	if uiDone != nil && uiDoneOnce != nil {
		uiDoneOnce.Do(func() {
			close(uiDone)
		})
	}
}

// ==================== 倒计时按钮 ====================

// UI_RunCountdownSeconds 运行脚本倒计时总秒数
var UI_RunCountdownSeconds int = 300

// UI_RunCountdownStartTime 倒计时开始时间
var UI_RunCountdownStartTime time.Time

// UI_RunCountdownStarted 是否已经开始倒计时
var UI_RunCountdownStarted bool = false

// UI_RunCountdownTriggered 是否已经触发过运行脚本，防止重复触发
var UI_RunCountdownTriggered bool = false

// UI_RunCountdownEnable 是否启用自动倒计时运行
var UI_RunCountdownEnable bool = true

// UI_RunCountdownPaused 倒计时是否已暂停（点"停止"后为 true：冻结剩余秒数、不自动触发）
var UI_RunCountdownPaused bool = false

// UI_RunCountdownFrozen 暂停时冻结的剩余秒数
var UI_RunCountdownFrozen int = 0

// UI_设置倒计时按钮 创建一个带自动倒计时的按钮，倒计时结束自动触发回调
// label: 按钮文字
// bgColor: 背景色
// textColor: 文字色
// borderColor: 边框色
// width/height: 按钮尺寸
// seconds: 倒计时秒数，<=0 时退化为普通按钮
// callback: 点击或倒计时结束时的回调
func UI_设置倒计时按钮(
	label string,
	bgColor string,
	textColor string,
	borderColor string,
	width float32,
	height float32,
	seconds int,
	callback func(),
) {
	if seconds <= 0 {
		UI_设置按钮(label, bgColor, textColor, borderColor, width, height, callback)
		return
	}

	// 第一次绘制时初始化倒计时
	if !UI_RunCountdownStarted {
		UI_RunCountdownStarted = true
		UI_RunCountdownTriggered = false
		UI_RunCountdownStartTime = time.Now()
		UI_RunCountdownSeconds = seconds
	}

	remaining := UI_RunCountdownSeconds

	// 已点"停止"：冻结在剩余秒数，不再计时、不自动触发
	if UI_RunCountdownPaused {
		remaining = UI_RunCountdownFrozen
	} else if UI_RunCountdownEnable {
		elapsed := int(time.Since(UI_RunCountdownStartTime).Seconds())
		remaining = UI_RunCountdownSeconds - elapsed

		if remaining < 0 {
			remaining = 0
		}

		// 倒计时结束，自动触发回调
		if remaining <= 0 && !UI_RunCountdownTriggered {
			UI_RunCountdownTriggered = true
			if callback != nil {
				callback()
			}
			return
		}
	}

	buttonText := fmt.Sprintf("%s  %ds", label, remaining)

	UI_设置按钮(buttonText, bgColor, textColor, borderColor, width, height, func() {
		UI_RunCountdownTriggered = true
		if callback != nil {
			callback()
		}
	})
}

// UI_重置运行倒计时 给倒计时加时60秒
func UI_重置运行倒计时() {
	if !UI_RunCountdownStarted {
		UI_RunCountdownStarted = true
		UI_RunCountdownStartTime = time.Now()
	}

	UI_RunCountdownSeconds += 60
	UI_RunCountdownTriggered = false
	// 加时视为恢复计时（若之前点过"停止"）
	UI_RunCountdownPaused = false

	fmt.Println("运行脚本倒计时已加时 60 秒")
}

// UI_停止倒计时 停止倒计时：冻结当前剩余秒数，不再计时、不自动触发运行脚本。
// 之后点"继续"或"加时"会恢复计时。
func UI_停止倒计时() {
	// 已经暂停了：重复点击不重复冻结，避免用旧起点重算导致秒数跳动 / 日志刷屏
	if UI_RunCountdownPaused {
		fmt.Println("运行脚本倒计时已是停止状态")
		return
	}
	if !UI_RunCountdownStarted {
		// 还没开始计时（无剩余），直接暂停在 0
		UI_RunCountdownPaused = true
		UI_RunCountdownFrozen = 0
		fmt.Println("运行脚本倒计时已停止（无剩余）")
		return
	}
	elapsed := int(time.Since(UI_RunCountdownStartTime).Seconds())
	remaining := UI_RunCountdownSeconds - elapsed
	if remaining < 0 {
		remaining = 0
	}
	UI_RunCountdownFrozen = remaining
	UI_RunCountdownPaused = true
	fmt.Println("运行脚本倒计时已停止，剩余", remaining, "秒")
}

// UI_继续倒计时 从暂停处恢复计时：以冻结的剩余秒数为总时长、当前时间为新起点继续倒数。
// 恢复后秒数不会跳变，并允许到点后再次自动触发运行脚本。
func UI_继续倒计时() {
	if !UI_RunCountdownPaused {
		return
	}

	remaining := UI_RunCountdownFrozen
	if remaining < 0 {
		remaining = 0
	}

	// 以剩余秒数为总时长、当前时间为起点，等价于"从 remaining 继续倒数"
	UI_RunCountdownSeconds = remaining
	UI_RunCountdownStartTime = time.Now()
	UI_RunCountdownPaused = false
	UI_RunCountdownStarted = true
	// 允许到点后再次自动触发
	UI_RunCountdownTriggered = false

	fmt.Println("运行脚本倒计时已继续，剩余", remaining, "秒")
}

// UI_切换倒计时暂停 点击「停止/继续」按钮时调用：已暂停则继续，否则停止。
func UI_切换倒计时暂停() {
	if UI_RunCountdownPaused {
		UI_继续倒计时()
	} else {
		UI_停止倒计时()
	}
}

// ==================== 配置管理 ====================

/*
======== 以下为原配置槽管理（2026-09-26 起注释，依赖已移除的配置下拉框 ConfigSelect） ========

// UI_获取备用配置路径 根据下拉框选择返回对应的配置文件路径
func UI_获取备用配置路径() string {
	switch config.ConfigSelect {
	case 0:
		return "/sdcard/ui_config_0.json"
	case 1:
		return "/sdcard/ui_config_1.json"
	case 2:
		return "/sdcard/ui_config_2.json"
	default:
		return "/sdcard/ui_config_0.json"
	}
}

// UI_手动保存配置 将当前配置保存到选中的配置槽文件
func UI_手动保存配置() {
	file := UI_获取备用配置路径()
	err := SaveConfig(file, &config)
	if err != nil {
		fmt.Println("保存配置槽失败:", err)
	} else {
		fmt.Println("保存配置槽成功:", file)
	}
}

// UI_手动读取配置 从选中的配置槽文件读取配置
func UI_手动读取配置() {
	file := UI_获取备用配置路径()
	err := LoadConfig(file, &config)
	if err != nil {
		fmt.Println("读取配置槽失败:", err)
	} else {
		fmt.Println("读取配置槽成功:", file)
	}
}

======== 原配置槽管理 注释结束 ========
*/

// ==================== 标签栏 ====================

// uiTabActiveMap 记录每个标签栏当前选中的标签下标
var uiTabActiveMap = map[string]int{}

// UI_TabFontScale 标签页字体缩放
var UI_TabFontScale float32 = 0.9

// UI_TabTextColor 标签页未选中时文字颜色
var UI_TabTextColor string = "#546e7a"

// UI_TabBgColor 普通标签背景颜色
var UI_TabBgColor string = "#eceff1"

// UI_TabHoveredColor 鼠标悬停/触摸时颜色
var UI_TabHoveredColor string = "#cfd8dc"

// UI_TabActiveColor 当前选中标签背景颜色
var UI_TabActiveColor string = "#1e88e5"

// UI_TabActiveTextColor 当前选中标签文字颜色
var UI_TabActiveTextColor string = "#ffffff"

// UI_TabBorderColor 标签页分割线颜色
var UI_TabBorderColor string = "#b0bec5"

// UI_TabHeight 标签按钮高度
var UI_TabHeight float32 = 36

// UI_TabMinWidth 标签按钮最小宽度
var UI_TabMinWidth float32 = 88

// UI_TabPaddingX 标签按钮左右额外宽度
var UI_TabPaddingX float32 = 26

// UI_设置标签页样式 设置标签页预设样式
// style:
// 0 = 绿色清爽风
// 1 = 蓝色科技风
// 2 = 紫色暗黑风
// 3 = 红色警告风
func UI_设置标签页样式(style int) {
	switch style {
	case 0:
		// 绿色清爽风
		UI_TabFontScale = 0.8
		UI_TabTextColor = "#2e7d32"
		UI_TabBgColor = "#e8f5e9"
		UI_TabHoveredColor = "#c8e6c9"
		UI_TabActiveColor = "#43a047"
		UI_TabActiveTextColor = "#ffffff"
		UI_TabBorderColor = "#a5d6a7"
		UI_TabHeight = 45
		UI_TabMinWidth = 90

	case 1:
		// 蓝色科技风
		UI_TabFontScale = 0.8
		UI_TabTextColor = "#1565c0"
		UI_TabBgColor = "#e3f2fd"
		UI_TabHoveredColor = "#bbdefb"
		UI_TabActiveColor = "#1e88e5"
		UI_TabActiveTextColor = "#ffffff"
		UI_TabBorderColor = "#90caf9"
		UI_TabHeight = 45
		UI_TabMinWidth = 90

	case 2:
		// 紫色优雅风
		UI_TabFontScale = 0.8
		UI_TabTextColor = "#4527a0"
		UI_TabBgColor = "#ede7f6"
		UI_TabHoveredColor = "#d1c4e9"
		UI_TabActiveColor = "#7c4dff"
		UI_TabActiveTextColor = "#ffffff"
		UI_TabBorderColor = "#b39ddb"
		UI_TabHeight = 45
		UI_TabMinWidth = 90

	case 3:
		// 红色警告风
		UI_TabFontScale = 0.8
		UI_TabTextColor = "#c62828"
		UI_TabBgColor = "#ffebee"
		UI_TabHoveredColor = "#ffcdd2"
		UI_TabActiveColor = "#e53935"
		UI_TabActiveTextColor = "#ffffff"
		UI_TabBorderColor = "#ef9a9a"
		UI_TabHeight = 45
		UI_TabMinWidth = 90

	default:
		UI_设置标签页样式(0)
	}
}

// ==================== 滑动滚动 ====================

// UI_启用滑动滚动
// 作用：允许在当前窗口/区域内按住拖动上下滑动，实现内容滚动。
// speed：滚动速度，建议 1.0 ~ 2.5。
// 注意：这个函数要放在窗口 Begin 之后、绘制控件之前或之后都可以。
func UI_启用滑动滚动(speed float32) {
	if speed <= 0 {
		speed = 1.2
	}

	// 只有鼠标/手指在当前窗口上时才处理
	if !imgui.IsWindowHovered() {
		return
	}

	// 鼠标左键/触摸按下拖动
	if imgui.IsMouseDragging(imgui.MouseButtonLeft) {
		delta := imgui.MouseDragDelta()

		// delta.Y > 0 表示手指往下拖，内容应该往上滚，所以 scrollY 减小
		// delta.Y < 0 表示手指往上拖，内容应该往下滚，所以 scrollY 增大
		newScrollY := imgui.ScrollY() - delta.Y*speed

		if newScrollY < 0 {
			newScrollY = 0
		}

		maxY := imgui.ScrollMaxY()
		if newScrollY > maxY {
			newScrollY = maxY
		}

		imgui.SetScrollYFloat(newScrollY)

		// 重置拖动差值，否则 delta 会累计，滚动会越来越猛
		imgui.ResetMouseDragDelta()
	}
}

// ==================== 标签栏组件 ====================

// UI_设置标签栏 创建一个自定义标签栏
// id：标签栏唯一 ID，多个标签栏时 ID 不能重复
// titles：标签标题列表
// pages：每个标签对应的页面绘制函数
func UI_设置标签栏(id string, titles []string, pages []func()) {
	if len(titles) == 0 || len(pages) == 0 {
		return
	}

	if len(titles) != len(pages) {
		imgui.Text("标签标题数量和页面数量不一致")
		return
	}

	// 获取当前选中的标签下标
	activeIndex, ok := uiTabActiveMap[id]
	if !ok {
		activeIndex = 0
		uiTabActiveMap[id] = 0
	}

	if activeIndex < 0 || activeIndex >= len(titles) {
		activeIndex = 0
		uiTabActiveMap[id] = 0
	}

	// 临时切换标签页字体缩放
	oldScale := UI_FontScale
	if UI_TabFontScale > 0 {
		imgui.SetWindowFontScale(UI_TabFontScale)
	}
	defer imgui.SetWindowFontScale(oldScale)

	// 绘制每个标签按钮
	for i, title := range titles {
		isActive := i == activeIndex

		bgColor := UI_TabBgColor
		textColor := UI_TabTextColor

		if isActive {
			bgColor = UI_TabActiveColor
			textColor = UI_TabActiveTextColor
		}

		imgui.PushStyleColorVec4(imgui.ColButton, HexToVec4(bgColor))
		imgui.PushStyleColorVec4(imgui.ColButtonHovered, HexToVec4(UI_TabHoveredColor))
		imgui.PushStyleColorVec4(imgui.ColButtonActive, HexToVec4(UI_TabActiveColor))
		imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))

		// 根据文字长度自适应宽度
		tabWidth := float32(len([]rune(title)))*14 + UI_TabPaddingX
		if tabWidth < UI_TabMinWidth {
			tabWidth = UI_TabMinWidth
		}

		if imgui.ButtonV(title+"##"+id, imgui.Vec2{X: tabWidth, Y: UI_TabHeight}) {
			uiTabActiveMap[id] = i
			activeIndex = i
		}

		imgui.PopStyleColorV(4)

		if i != len(titles)-1 {
			imgui.SameLine()
		}
	}

	// 标签下方分割线
	imgui.PushStyleColorVec4(imgui.ColSeparator, HexToVec4(UI_TabBorderColor))
	imgui.Separator()
	imgui.PopStyleColor()

	imgui.Spacing()

	// 绘制当前选中标签的页面内容
	if activeIndex >= 0 && activeIndex < len(pages) {
		if pages[activeIndex] != nil {
			pages[activeIndex]()
		}
	}
}

// UI_设置标签页 创建一个标签页（配合 imgui 原生 TabBar 使用）
// title：标签页显示标题
// content：当前标签页被选中时绘制的控件
func UI_设置标签页(title string, content func()) {
	if imgui.BeginTabItem(title) {
		if content != nil {
			content()
		}
		imgui.EndTabItem()
	}
}

// ==================== 折叠控件 ====================

// uiCollapseOpenMap 记录每个折叠控件当前是否展开
var uiCollapseOpenMap = map[string]bool{}

// uiCollapseInitMap 记录每个折叠控件是否已经初始化默认状态
var uiCollapseInitMap = map[string]bool{}

// UI_设置折叠 绘制一个无指针折叠控件
//
// title: 标题，同时也作为内部唯一ID，建议不同折叠控件不要重名
// bgColor: 背景颜色
// textColor: 文字颜色
// defaultOpen: 默认是否展开，仅第一次初始化时生效
// width: 标题按钮宽度，<=0 自动宽度
// height: 标题按钮高度，<=0 自动高度
// fontScale: 字体缩放，<=0 使用全局 UI_FontScale
// paddingX: 横向内边距
// paddingY: 纵向内边距
// content: 展开时绘制的内容
func UI_设置折叠(
	title string,
	bgColor string,
	textColor string,
	defaultOpen bool,
	width float32,
	height float32,
	fontScale float32,
	paddingX float32,
	paddingY float32,
	content func(),
) bool {
	if title == "" {
		return false
	}

	if bgColor == "" {
		bgColor = "#ffffff"
	}

	if textColor == "" {
		textColor = "#1565c0"
	}

	if fontScale <= 0 {
		fontScale = UI_FontScale
	}

	if paddingX <= 0 {
		paddingX = 8
	}

	if paddingY <= 0 {
		paddingY = 6
	}

	// 用 title 作为状态 key
	key := title

	// 第一次创建时设置默认展开状态
	if !uiCollapseInitMap[key] {
		uiCollapseOpenMap[key] = defaultOpen
		uiCollapseInitMap[key] = true
	}

	open := uiCollapseOpenMap[key]

	arrow := "▶"
	if open {
		arrow = "▼"
	}

	label := arrow + " " + title + "##COLLAPSE_" + key

	// 临时字体缩放
	imgui.SetWindowFontScale(fontScale)
	defer imgui.SetWindowFontScale(UI_FontScale)

	// 标题样式
	imgui.PushStyleVarVec2(
		imgui.StyleVarFramePadding,
		imgui.Vec2{X: paddingX, Y: paddingY},
	)
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 8)
	defer imgui.PopStyleVarV(2)

	imgui.PushStyleColorVec4(imgui.ColButton, HexToVec4(bgColor))
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, LightenColor(bgColor, 0.12))
	imgui.PushStyleColorVec4(imgui.ColButtonActive, DarkenColor(bgColor, 0.10))
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))
	defer imgui.PopStyleColorV(4)

	clicked := false

	if width > 0 || height > 0 {
		clicked = imgui.ButtonV(label, imgui.Vec2{X: width, Y: height})
	} else {
		clicked = imgui.Button(label)
	}

	if clicked {
		open = !open
		uiCollapseOpenMap[key] = open
	}

	// 展开时绘制内容
	if open && content != nil {
		imgui.Spacing()
		content()
		imgui.Spacing()
	}

	return open
}

// ==================== 基础控件 ====================

// UI_设置文本框Ex 绘制一个可自定义样式的文本标签块
// innerName: 内部唯一ID
// text: 显示文字
// bgColor: 背景色
// textColor: 文字颜色
// sameLine: 是否和下一个控件横向排列
// width: 宽度，<=0 自动宽度
// height: 高度，<=0 自动高度
// fontScale: 字体缩放，<=0 使用全局 UI_FontScale
// paddingX: 横向内边距
// paddingY: 纵向内边距
func UI_设置文本(
	innerName string,
	text string,
	bgColor string,
	textColor string,
	sameLine bool,
	width float32,
	height float32,
	fontScale float32,
	paddingX float32,
	paddingY float32,
) {
	if text == "" {
		return
	}

	if innerName == "" {
		innerName = text
	}

	if bgColor == "" {
		bgColor = "#e3f2fd"
	}

	if textColor == "" {
		textColor = "#1565c0"
	}

	if fontScale <= 0 {
		fontScale = UI_FontScale
	}

	if paddingX <= 0 {
		paddingX = 8
	}

	if paddingY <= 0 {
		paddingY = 6
	}

	// 临时字体缩放
	imgui.SetWindowFontScale(fontScale)
	defer imgui.SetWindowFontScale(UI_FontScale)

	// 内边距，和复选框保持一致
	imgui.PushStyleVarVec2(
		imgui.StyleVarFramePadding,
		imgui.Vec2{X: paddingX, Y: paddingY},
	)
	defer imgui.PopStyleVarV(1)

	// 背景色
	imgui.PushStyleColorVec4(imgui.ColButton, HexToVec4(bgColor))

	// 悬停和点击也用同色，避免看起来像按钮
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, HexToVec4(bgColor))
	imgui.PushStyleColorVec4(imgui.ColButtonActive, HexToVec4(bgColor))

	// 文字颜色
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))

	defer imgui.PopStyleColorV(4)

	label := text + "##TEXT_" + innerName

	if width > 0 || height > 0 {
		imgui.ButtonV(
			label,
			imgui.Vec2{
				X: width,
				Y: height,
			},
		)
	} else {
		imgui.Button(label)
	}

	if sameLine {
		imgui.SameLine()
	}
}

// UI_换行线 绘制一条分割线并加一点间距
func UI_换行线() {
	imgui.Separator()
	imgui.Spacing()
}

// UI_间距 添加两行垂直间距
func UI_间距() {
	imgui.Spacing()
	imgui.Spacing()
}

// UI_设置单选框 绘制一组“复选框外观”的单选框
// innerName: 内部唯一ID，避免多个单选框组冲突
// items: 选项文字列表
// value: 绑定的 int32 指针，选中项下标
// layout: "水平" 表示横向排列，其他值为纵向排列
// bgColor: 方框背景色
// textColor: 文字颜色
// checkColor: 勾选标记颜色
// fontScale: 字体缩放，<=0 使用全局 UI_FontScale
// paddingX: 横向内边距
// paddingY: 纵向内边距
func UI_设置单选框(
	innerName string,
	items []string,
	value *int32,
	layout string,
	bgColor string,
	textColor string,
	checkColor string,
	fontScale float32,
	paddingX float32,
	paddingY float32,
) {
	if value == nil {
		return
	}

	if len(items) == 0 {
		return
	}

	if innerName == "" {
		innerName = "SquareRadioGroup"
	}

	if bgColor == "" {
		bgColor = "#e8eaf6"
	}

	if textColor == "" {
		textColor = "#283593"
	}

	if checkColor == "" {
		checkColor = "#43a047"
	}

	if fontScale <= 0 {
		fontScale = UI_FontScale
	}

	if paddingX <= 0 {
		paddingX = 8
	}

	if paddingY <= 0 {
		paddingY = 6
	}

	// 防止 value 越界
	if *value < 0 || int(*value) >= len(items) {
		*value = 0
	}

	// 临时字体缩放
	imgui.SetWindowFontScale(fontScale)
	defer imgui.SetWindowFontScale(UI_FontScale)

	// 控件内边距，和你的复选框保持一致
	imgui.PushStyleVarVec2(
		imgui.StyleVarFramePadding,
		imgui.Vec2{X: paddingX, Y: paddingY},
	)
	defer imgui.PopStyleVarV(1)

	// 方框背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBg, HexToVec4(bgColor))

	// 鼠标悬停背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBgHovered, LightenColor(bgColor, 0.12))

	// 点击时背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBgActive, LightenColor(bgColor, 0.20))

	// 文字颜色
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))

	// 勾选颜色
	imgui.PushStyleColorVec4(imgui.ColCheckMark, HexToVec4(checkColor))

	defer imgui.PopStyleColorV(5)

	for i, item := range items {
		index := int32(i)

		// 当前项是否选中
		checked := *value == index

		// label 显示文字 + 内部唯一ID
		label := item + "##" + innerName + "_" + strconv.Itoa(i)

		// 用 Checkbox 画外观，但点击后只设置当前项为选中
		if imgui.Checkbox(label, &checked) {
			*value = index
		}

		if layout == "水平" && i != len(items)-1 {
			imgui.SameLine()
		}
	}
}

// ==================== 下拉框 ====================

// UI_ComboTextColor 下拉框文字颜色
var UI_ComboTextColor string = "#2d3436"

// UI_ComboBgColor 下拉框未展开时的背景颜色
var UI_ComboBgColor string = "#eceff1"

// UI_ComboHoveredColor 下拉框悬停时背景颜色
var UI_ComboHoveredColor string = "#cfd8dc"

// UI_ComboActiveColor 下拉框点击/激活时背景颜色
var UI_ComboActiveColor string = "#bbdefb"

// UI_ComboPopupBgColor 下拉框展开后的弹出列表背景颜色
var UI_ComboPopupBgColor string = "#ffffff"

// UI_ComboItemColor 下拉框展开后普通选项背景颜色
var UI_ComboItemColor string = "#f5f5f5"

// UI_ComboItemHoveredColor 下拉框展开后选项悬停颜色
var UI_ComboItemHoveredColor string = "#e3f2fd"

// UI_ComboItemActiveColor 下拉框展开后选项选中/点击颜色
var UI_ComboItemActiveColor string = "#1e88e5"

// UI_设置下拉框Ex 绘制一个可自定义样式的下拉选择框
// label: 标签文字
// value: 绑定的 int32 指针，当前选中项下标
// items: 选项列表
// bgColor: 下拉框背景色，传空使用 UI_ComboBgColor
// textColor: 文字颜色，传空使用 UI_ComboTextColor
// width: 宽度，<=0 使用默认宽度
// height: 高度，<=0 不强制高度，通过 paddingY 控制
// fontScale: 字体缩放，<=0 使用 UI_FontScale
// paddingX: 横向内边距，<=0 默认 8
// paddingY: 纵向内边距，<=0 默认 6
func UI_设置下拉框(
	label string,
	value *int32,
	items []string,
	bgColor string,
	textColor string,
	width float32,
	height float32,
	fontScale float32,
	paddingX float32,
	paddingY float32,
) {
	if value == nil {
		return
	}

	if len(items) == 0 {
		return
	}

	if bgColor == "" {
		bgColor = UI_ComboBgColor
	}

	if textColor == "" {
		textColor = UI_ComboTextColor
	}

	if fontScale <= 0 {
		fontScale = UI_FontScale
	}

	if paddingX <= 0 {
		paddingX = 8
	}

	if paddingY <= 0 {
		paddingY = 6
	}

	// 防止下标越界
	if *value < 0 {
		*value = 0
	}

	if int(*value) >= len(items) {
		*value = int32(len(items) - 1)
	}

	// 字体缩放
	imgui.SetWindowFontScale(fontScale)
	defer imgui.SetWindowFontScale(UI_FontScale)

	// 下拉框高度没有直接设置接口，通常通过 FramePadding.Y 控制高度
	// 如果传入 height，则根据高度粗略计算 paddingY
	if height > 0 {
		// 这里 16 是 ImGui 默认字体高度的近似值
		textHeight := float32(16) * fontScale
		autoPaddingY := (height - textHeight) / 2

		if autoPaddingY > 0 {
			paddingY = autoPaddingY
		}
	}

	// 横向、纵向内边距
	imgui.PushStyleVarVec2(
		imgui.StyleVarFramePadding,
		imgui.Vec2{
			X: paddingX,
			Y: paddingY,
		},
	)
	defer imgui.PopStyleVarV(1)

	// 文字颜色
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))

	// 下拉框背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBg, HexToVec4(bgColor))

	// 其他颜色使用全局变量
	imgui.PushStyleColorVec4(imgui.ColFrameBgHovered, HexToVec4(UI_ComboHoveredColor))
	imgui.PushStyleColorVec4(imgui.ColFrameBgActive, HexToVec4(UI_ComboActiveColor))
	imgui.PushStyleColorVec4(imgui.ColPopupBg, HexToVec4(UI_ComboPopupBgColor))
	imgui.PushStyleColorVec4(imgui.ColHeader, HexToVec4(UI_ComboItemColor))
	imgui.PushStyleColorVec4(imgui.ColHeaderHovered, HexToVec4(UI_ComboItemHoveredColor))
	imgui.PushStyleColorVec4(imgui.ColHeaderActive, HexToVec4(UI_ComboItemActiveColor))

	defer imgui.PopStyleColorV(8)

	// 宽度
	if width > 0 {
		imgui.SetNextItemWidth(width)
	} else {
		if UI_AutoItemWidth {
			imgui.SetNextItemWidth(-1)
		} else {
			imgui.SetNextItemWidth(UI_InputWidth)
		}
	}

	imgui.ComboStrarr(label, value, items, int32(len(items)))
}

// ==================== 复选框扩展 ====================

// UI_设置复选框Ex 绘制一个可自定义样式的复选框
// innerName: 内部唯一ID
// showName: 界面显示名字
// defaultChecked: 默认值（仅文档说明）
// value: 绑定的 bool 指针
// sameLine: 是否和下一个控件横向排列
// bgColor: 复选框方块背景色
// textColor: 文字颜色
// checkColor: 勾选标记颜色
// fontScale: 字体缩放，<=0 使用全局 UI_FontScale
// paddingX: 横向内边距
// paddingY: 纵向内边距
func UI_设置复选框(
	innerName string,
	showName string,
	defaultChecked bool,
	value *bool,
	sameLine bool,
	bgColor string,
	textColor string,
	checkColor string,
	fontScale float32,
	paddingX float32,
	paddingY float32,
) {
	if value == nil {
		return
	}

	_ = defaultChecked

	label := showName + "##" + innerName

	// 临时字体缩放
	if fontScale <= 0 {
		fontScale = UI_FontScale
	}
	imgui.SetWindowFontScale(fontScale)
	defer imgui.SetWindowFontScale(UI_FontScale)

	// 控件内边距
	imgui.PushStyleVarVec2(
		imgui.StyleVarFramePadding,
		imgui.Vec2{X: paddingX, Y: paddingY},
	)
	defer imgui.PopStyleVarV(1)

	// 复选框方块背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBg, HexToVec4(bgColor))
	// 鼠标悬停背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBgHovered, LightenColor(bgColor, 0.12))
	// 点击时背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBgActive, LightenColor(bgColor, 0.20))
	// 文字颜色
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))
	// 勾选颜色
	imgui.PushStyleColorVec4(imgui.ColCheckMark, HexToVec4(checkColor))
	defer imgui.PopStyleColorV(5)

	imgui.Checkbox(label, value)

	if sameLine {
		imgui.SameLine()
	}
}

// ==================== 窗口居中 ====================

// UI_设置窗口居中 将下一个窗口设置为屏幕居中
// width/height: 屏幕宽高
func UI_设置窗口居中(width int, height int) {
	screenW := float32(width)
	screenH := float32(height)

	// 以窗口中心为锚点，定位到屏幕中心。
	// 用 CondAlways：横竖屏切换时屏幕宽高变化，窗口会自动重新居中；
	// 若用 CondOnce 则只在首帧定位一次，转屏后会停在旧位置（偏左）。
	imgui.SetNextWindowPosV(
		imgui.Vec2{
			X: screenW / 2,
			Y: screenH / 2,
		},
		imgui.CondAlways,
		imgui.Vec2{
			X: 0.5,
			Y: 0.5,
		},
	)
}

// =========UI_设置输入框 绘制一个输入框==================
// UI_InputHintColor 输入框占位提示文字颜色
var UI_InputHintColor string = "#90a4ae"

// label: 标签文字
// hint: 占位提示文字
// value: 绑定的字符串指针
// multiline: 是否多行输入
// width: 宽度，<=0 时使用默认宽度
// height: 多行模式下的高度，<=0 时默认100
// number: 是否为数字模式，单行有效，显示加减按钮
func UI_设置输入框(label string, hint string, value *string, multiline bool, width float32, height float32, number bool) {
	if value == nil {
		return
	}

	imgui.PushStyleColorVec4(imgui.ColFrameBg, HexToVec4("#eceff1"))
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4("#2d3436"))
	defer imgui.PopStyleColorV(2)

	if width <= 0 {
		if UI_AutoItemWidth {
			width = -1
		} else {
			width = UI_InputWidth
		}
	}

	// 用指针生成唯一 ID，避免 label 重复时控件冲突
	id := fmt.Sprintf("%p", value)

	if multiline {
		if height <= 0 {
			height = 100
		}

		if label != "" {
			imgui.Text(label)

			if *value == "" && hint != "" {
				imgui.SameLine()
				imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(UI_InputHintColor))
				imgui.Text(hint)
				imgui.PopStyleColor()
			}
		} else if *value == "" && hint != "" {
			imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(UI_InputHintColor))
			imgui.Text(hint)
			imgui.PopStyleColor()
		}

		imgui.InputTextMultiline(
			"##MULTILINE_"+id,
			value,
			imgui.Vec2{X: width, Y: height},
			imgui.InputTextFlags(0),
			nil,
		)

		return
	}

	if number {
		// 数字模式：[输入框] [-] [+] label
		const btnWidth float32 = 28
		const minInputWidth float32 = 60
		const spacing float32 = 8

		reserveWidth := btnWidth*2 + spacing*2

		if width < 0 {
			// width 为负数时，让输入框自动占满剩余区域，并给右侧按钮留空间
			imgui.SetNextItemWidth(width - reserveWidth)
		} else {
			inputWidth := width - reserveWidth
			if inputWidth < minInputWidth {
				inputWidth = minInputWidth
			}
			imgui.SetNextItemWidth(inputWidth)
		}

		imgui.InputTextWithHint(
			"##NUM_"+id,
			hint,
			value,
			imgui.InputTextFlagsCharsDecimal,
			nil,
		)

		imgui.SameLine()

		if imgui.ButtonV("-##dec_"+id, imgui.Vec2{X: btnWidth, Y: 0}) {
			n, _ := strconv.Atoi(*value)
			n--
			*value = strconv.Itoa(n)
		}

		imgui.SameLine()

		if imgui.ButtonV("+##inc_"+id, imgui.Vec2{X: btnWidth, Y: 0}) {
			n, _ := strconv.Atoi(*value)
			n++
			*value = strconv.Itoa(n)
		}

		if label != "" {
			imgui.SameLine()
			imgui.Text(label)
		}

		return
	}

	inputLabel := label
	if inputLabel == "" {
		inputLabel = "##INPUT_" + id
	}

	imgui.SetNextItemWidth(width)
	imgui.InputTextWithHint(
		inputLabel,
		hint,
		value,
		imgui.InputTextFlags(0),
		nil,
	)
}

// ==================== 开关 ====================

// UI_SwitchOnColor 开关开启时背景色
var UI_SwitchOnColor string = "#43a047"

// UI_SwitchOnTextColor 开关开启时文字颜色
var UI_SwitchOnTextColor string = "#ffffff"

// UI_SwitchLabelColor 开关左侧说明文字颜色
var UI_SwitchLabelColor string = "#2d3436"

// UI_SwitchRounding 开关圆角，不再使用 height/2，避免胶囊感太强
var UI_SwitchRounding float32 = 10

// UI_设置开关 创建一个普通按钮风格的开关控件
func UI_设置开关(innerName string, showName string, value *bool, sameLine bool, width float32, height float32) {
	if value == nil {
		return
	}

	if width <= 0 {
		width = 64
	}
	if height <= 0 {
		height = 28
	}

	// 左侧说明文字
	if showName != "" {
		// 让文字和按钮控件的基线对齐
		imgui.AlignTextToFramePadding()

		imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(UI_SwitchLabelColor))
		imgui.Text(showName)
		imgui.PopStyleColor()

		imgui.SameLine()
	}

	switchText := "关闭"
	if *value {
		switchText = "开启"
	}

	// 关键修改：
	// 1. 圆角不再用 height/2，改成小圆角
	// 2. FramePadding.Y 设置为 0，避免固定高度按钮上下视觉不均
	// 3. ButtonTextAlign 保证按钮文字居中
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, UI_SwitchRounding)
	imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, imgui.Vec2{
		X: 8,
		Y: 0,
	})
	imgui.PushStyleVarVec2(imgui.StyleVarButtonTextAlign, imgui.Vec2{
		X: 0.5,
		Y: 0.5,
	})
	defer imgui.PopStyleVarV(3)

	// 开启状态：绿色背景
	// 关闭状态：不设置颜色，使用 ImGui 默认按钮背景色
	if *value {
		imgui.PushStyleColorVec4(imgui.ColButton, HexToVec4(UI_SwitchOnColor))
		imgui.PushStyleColorVec4(imgui.ColButtonHovered, LightenColor(UI_SwitchOnColor, 0.08))
		imgui.PushStyleColorVec4(imgui.ColButtonActive, DarkenColor(UI_SwitchOnColor, 0.08))
		imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(UI_SwitchOnTextColor))
		defer imgui.PopStyleColorV(4)
	}

	if imgui.ButtonV(switchText+"##SWITCH_"+innerName, imgui.Vec2{
		X: width,
		Y: height,
	}) {
		*value = !*value
	}

	if sameLine {
		imgui.SameLine()
	}
}

// ==================== 按钮 ====================

// UI_ButtonTextColor 按钮文字颜色
var UI_ButtonTextColor string = "#2d3436"

// UI_ButtonBorderColor 按钮边框颜色
var UI_ButtonBorderColor string = "#1e88e5"

// UI_ButtonBorderSize 按钮边框粗细
var UI_ButtonBorderSize float32 = 1.5

// UI_ButtonRounding 按钮圆角
var UI_ButtonRounding float32 = 8

// UI_ButtonFramePaddingX 按钮左右内边距
var UI_ButtonFramePaddingX float32 = 12

// UI_ButtonFramePaddingY 按钮上下内边距
var UI_ButtonFramePaddingY float32 = 7

// UI_设置按钮 绘制一个自定义样式的按钮
// label: 按钮文字
// bgColor: 背景色
// textColor: 文字色
// borderColor: 边框色
// width/height: 按钮尺寸
// callback: 点击回调
func UI_设置按钮(
	label string,
	bgColor string,
	textColor string,
	borderColor string,
	width float32,
	height float32,
	callback func(),
) {
	imgui.PushStyleColorVec4(imgui.ColButton, HexToVec4(bgColor))
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, LightenColor(bgColor, 0.15))
	imgui.PushStyleColorVec4(imgui.ColButtonActive, DarkenColor(bgColor, 0.15))
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4(textColor))
	imgui.PushStyleColorVec4(imgui.ColBorder, HexToVec4(borderColor))
	defer imgui.PopStyleColorV(5)

	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, UI_ButtonRounding)
	imgui.PushStyleVarVec2(
		imgui.StyleVarFramePadding,
		imgui.Vec2{X: UI_ButtonFramePaddingX, Y: UI_ButtonFramePaddingY},
	)
	defer imgui.PopStyleVarV(2)

	if imgui.ButtonV(label, imgui.Vec2{X: width, Y: height}) {
		if callback != nil {
			callback()
		}
	}
}

// ==================== 配置持久化 ====================

// SaveConfig 将配置序列化为JSON并保存到文件
func SaveConfig(file string, cfg *UIConfig) error {
	dir := filepath.Dir(file)

	if dir != "." && dir != "/" {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile(file, data, 0644)
	if err != nil {
		return err
	}

	fmt.Println("配置已保存到:", file)
	return nil
}

// LoadConfig 从文件读取JSON并反序列化到配置结构体
// 如果文件不存在则自动创建默认配置
func LoadConfig(file string, cfg *UIConfig) error {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("配置文件不存在，创建默认配置:", file)
			return SaveConfig(file, cfg)
		}
		return err
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		return err
	}

	fmt.Println("配置已读取:", file)
	return nil
}

// PrintCurrentConfig 打印当前配置到控制台（调试用）
func PrintCurrentConfig() {
	data, _ := json.MarshalIndent(config, "", "    ")
	fmt.Println(string(data))
}

// ==================== 颜色工具函数 ====================

// HexToVec4 将十六进制颜色字符串转换为 imgui.Vec4
// 支持 "#RRGGBB" 和 "#RRGGBBAA" 格式
func HexToVec4(hex string) imgui.Vec4 {
	hex = strings.TrimSpace(hex)
	hex = strings.TrimPrefix(hex, "#")

	if len(hex) != 6 && len(hex) != 8 {
		return imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1}
	}

	var r, g, b, a uint8 = 255, 255, 255, 255

	if len(hex) == 6 {
		fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	} else {
		fmt.Sscanf(hex, "%02x%02x%02x%02x", &r, &g, &b, &a)
	}

	return imgui.Vec4{
		X: float32(r) / 255.0,
		Y: float32(g) / 255.0,
		Z: float32(b) / 255.0,
		W: float32(a) / 255.0,
	}
}

// LightenColor 将十六进制颜色提亮指定量
// amount: 提亮幅度，0~1 之间
func LightenColor(hex string, amount float32) imgui.Vec4 {
	c := HexToVec4(hex)
	c.X = clamp01(c.X + amount)
	c.Y = clamp01(c.Y + amount)
	c.Z = clamp01(c.Z + amount)
	return c
}

// DarkenColor 将十六进制颜色加深指定量
// amount: 加深幅度，0~1 之间
func DarkenColor(hex string, amount float32) imgui.Vec4 {
	c := HexToVec4(hex)
	c.X = clamp01(c.X - amount)
	c.Y = clamp01(c.Y - amount)
	c.Z = clamp01(c.Z - amount)
	return c
}

// clamp01 将浮点数限制在 0~1 范围内
func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ==================== 全局样式 ====================

// PushGlobalUIStyle 压入全局 UI 样式
//
// 设置内容：
// - 窗口、按钮、输入框、滚动条等控件的圆角
// - 窗口内边距、控件内边距、控件之间的间距
// - 全局文字颜色、输入框背景色、选中颜色等
//
// 当前压入：8 个 StyleVar + 8 个 StyleColor
// 必须配套调用 PopGlobalUIStyle 恢复
func PushGlobalUIStyle() {
	// 窗口圆角
	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 12)
	// 控件圆角（按钮、输入框、下拉框等）
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 8)
	// 滑块拖拽点圆角
	imgui.PushStyleVarFloat(imgui.StyleVarGrabRounding, 8)
	// 弹窗圆角
	imgui.PushStyleVarFloat(imgui.StyleVarPopupRounding, 8)
	// 滚动条圆角
	imgui.PushStyleVarFloat(imgui.StyleVarScrollbarRounding, 8)
	// 窗口内边距
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{X: 18, Y: 18})
	// 控件内边距（字号放大后同步加大，避免文字贴边）
	imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, imgui.Vec2{X: 12, Y: 9})
	// 控件间距（垂直间距加大，避免行与行太挤）
	imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: 12, Y: 12})

	// 全局文字颜色
	imgui.PushStyleColorVec4(imgui.ColText, HexToVec4("#2d3436"))
	// 输入框/复选框等 Frame 控件背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBg, HexToVec4("#eceff1"))
	// Frame 控件悬停背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBgHovered, HexToVec4("#cfd8dc"))
	// Frame 控件激活背景色
	imgui.PushStyleColorVec4(imgui.ColFrameBgActive, HexToVec4("#bbdefb"))
	// 复选框/单选框勾选标记颜色
	imgui.PushStyleColorVec4(imgui.ColCheckMark, HexToVec4("#1e88e5"))
	// Header 类控件背景色
	imgui.PushStyleColorVec4(imgui.ColHeader, HexToVec4("#e3f2fd"))
	// Header 悬停背景色
	imgui.PushStyleColorVec4(imgui.ColHeaderHovered, HexToVec4("#bbdefb"))
	// Header 选中背景色
	imgui.PushStyleColorVec4(imgui.ColHeaderActive, HexToVec4("#90caf9"))
}

// PopGlobalUIStyle 恢复 PushGlobalUIStyle 压入的全局样式
// 必须与 PushGlobalUIStyle 配对使用
func PopGlobalUIStyle() {
	imgui.PopStyleColorV(8)
	imgui.PopStyleVarV(8)
}
