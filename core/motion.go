package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dasongzi1366/AutoGo/motion"
	"github.com/Dasongzi1366/AutoGo/utils"
)

var (
	logFileMutex  sync.Mutex
	logFile       *os.File
	currentDate   string
	logDir        = "/sdcard/autogo_logs"
	logWriteCount int // 日志写入计数器，用于控制 Sync 频率
)

// initLogFile 初始化日志文件，每天一个文件
func initLogFile() error {
	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	now := time.Now().Local()
	today := now.Format("2006-01-02")

	// 检查文件是否还存在（如果文件被删除，需要重新打开）
	if logFile != nil && currentDate == today {
		logPath := filepath.Join(logDir, fmt.Sprintf("log_%s.txt", today))
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			// 文件被删除了，需要重新打开
			logFile.Close()
			logFile = nil
		} else {
			// 文件存在，不需要重新打开
			return nil
		}
	}

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, fmt.Sprintf("log_%s.txt", today))

	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	currentDate = today
	return nil
}

// writeLogToFile 同步写入日志到文件
func writeLogToFile(msg string) {
	if err := initLogFile(); err != nil {
		// 初始化日志文件失败，只打印到控制台
		fmt.Printf("[日志写入错误] 初始化日志文件失败: %v\n", err)
		return
	}

	if logFile == nil {
		// 日志文件未初始化，只打印到控制台
		fmt.Printf("[日志写入错误] 日志文件未初始化\n")
		return
	}

	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	_, err := logFile.WriteString(msg)
	if err != nil {
		// 写入失败，打印错误（但不能用 LogWithTime，避免递归）
		fmt.Printf("[日志写入错误] 写入文件失败: %v\n", err)
		// 尝试重新初始化文件
		logFile.Close()
		logFile = nil
		currentDate = ""
		return
	}

	// 每 100 条日志才刷新一次缓冲区（避免每条都 Sync 导致频繁磁盘I/O发热）
	logWriteCount++
	if logWriteCount >= 100 {
		logWriteCount = 0
		if err := logFile.Sync(); err != nil {
			fmt.Printf("[日志写入错误] 刷新缓冲区失败: %v\n", err)
			logFile.Close()
			logFile = nil
			currentDate = ""
		}
	}
}

var IsLog = false

// LogWithTimef 带时间前缀的格式化日志函数
func LogWithTimef(format string, args ...interface{}) {
	now := time.Now().Local()
	timestamp := now.Format("2006/01/02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logMsg := timestamp + " " + message + "\n"

	fmt.Print(logMsg)
	if IsLog {
		// 同步写入文件
		writeLogToFile(logMsg)
	}
}

// 全局变量记录上次鼠标位置
func Click(x, y int) {
	fmt.Println("精准点击位置", x, y)
	motion.Click(x, y, 0, 0)
}

// 在指定区域内随机点击一个坐标
func RandomClickInArea(x1, y1, x2, y2 int) {
	// 确保坐标顺序正确
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	// 计算区域宽度和高度
	width := x2 - x1
	height := y2 - y1

	// 在范围内生成随机坐标
	var randomX, randomY int
	if width > 0 {
		randomX = x1 + utils.Random(0, width)
	} else {
		randomX = x1
	}

	if height > 0 {
		randomY = y1 + utils.Random(0, height)
	} else {
		randomY = y1
	}

	// 点击随机位置
	Click(randomX, randomY)
}

// RandomClickInAreaWithDelay 在指定区域内随机点击，点击后随机延迟
// 参数:
//   - x1, y1, x2, y2: 点击区域坐标
//   - delay: 可选，1~2个延迟参数（毫秒）。不传默认 1000；传 1 个表示固定该值；传 2 个表示随机区间
func RandomClickInAreaWithDelay(x1, y1, x2, y2 int, delay ...int) {
	// 先执行点击
	RandomClickInArea(x1, y1, x2, y2)
	// 点击后随机延迟
	minDelay, maxDelay := 2000, 2500
	switch len(delay) {
	case 1:
		minDelay, maxDelay = delay[0], delay[0]
	case 2:
		minDelay, maxDelay = delay[0], delay[1]
	}
	utils.Sleep(utils.Random(minDelay, maxDelay))
}

// 毫秒
func RandomSleep(min, max int) {
	randNum := utils.Random(min, max)
	utils.Sleep(randNum)
}

// RandomSwipe 通用滑动：起终点四个坐标各随机偏移 ±5px，滑动后自动延时。
// 以后所有滑动统一用这个方法。
//
// 说明:
//   - x1,y1,x2,y2 每个坐标都叠加 [-5,5] 的随机偏移（避免固定点被识别）
//   - duration 为滑动持续时间（毫秒）
//   - delay 可选：滑动后延时（毫秒）。不传 = 默认等于"滑动秒数"（即 duration，单位换算后同值）
//
// 参数:
//   - x1, y1, x2, y2: 滑动起终点基础坐标
//   - duration: 滑动持续时间（毫秒）
//   - delay: 可选，滑动后延时（毫秒）；不传则默认 = duration（即"滑动秒数"）
func RandomSwipe(x1, y1, x2, y2, duration int, delay ...int) {
	// 四个坐标各随机 ±5px
	sx := x1 + utils.Random(-5, 5)
	sy := y1 + utils.Random(-5, 5)
	ex := x2 + utils.Random(-5, 5)
	ey := y2 + utils.Random(-5, 5)
	// 时长 ±10% 微随机（模拟人手滑动速度不恒定，避免固定时长特征）
	if duration > 0 {
		margin := duration / 10
		duration = duration + utils.Random(-margin, margin)
	}
	motion.Swipe(sx, sy, ex, ey, duration, 0, 0)

	// 滑动后延时：默认 = 滑动秒数（duration 毫秒 → 同值毫秒）
	d := duration
	if len(delay) > 0 {
		d = delay[0]
	}
	LogWithTimef("[Swipe] 随机滑动 基础(%d,%d)→(%d,%d) 实际(%d,%d)→(%d,%d) 时长=%dms 延时=%dms",
		x1, y1, x2, y2, sx, sy, ex, ey, duration, d)
	if d > 0 {
		utils.Sleep(d)
	}
}

// SwipeStraight 精确直线滑动：起终点坐标完全一致，无任何随机偏移
// 说明:
//   - 滑动路径是严格直线，起终点精确等于传入坐标
//   - 用于"偏差多少像素就滑多少像素"的精确对准场景
//
// 参数:
//   - x1, y1, x2, y2: 滑动起终点坐标
//   - duration: 滑动持续时间（毫秒）
func SwipeStraight(x1, y1, x2, y2, duration int) {
	motion.Swipe(x1, y1, x2, y2, duration, 0, 0)
}

// RandomSwipe2 直线滑动（起点→终点），仅 X 坐标随机偏移、Y 坐标保持原值
// 说明:
//   - 滑动始终是一条直线，上下方向绝不偏移（保证"直着滑"）
//   - 左右方向随机偏移 1~randomOffset 像素，避免被识别为点击
//
// 参数:
//   - x1, y1, x2, y2: 基础坐标
//   - duration: 滑动持续时间（毫秒）
//   - randomOffset: 左右随机偏移范围（像素），例如 5 表示 X 随机偏移 1-5 像素
func RandomSwipe2(x1, y1, x2, y2, duration, randomOffset int) {
	// 仅 X 坐标添加随机偏移（左右），Y 坐标保持原值 → 直线滑动不歪
	offset1 := utils.Random(1, randomOffset+1)
	offset3 := utils.Random(1, randomOffset+1)

	// 随机决定 X 偏移方向（正负）
	if utils.Random(0, 2) == 0 {
		offset1 = -offset1
	}
	if utils.Random(0, 2) == 0 {
		offset3 = -offset3
	}
	// 使用 Swipe 直线滑动（起终点直线插值，避免贝塞尔曲线的弧度导致滑动不直、游戏不识别）
	LogWithTimef("[Swipe] 直线滑动 基础(%d,%d)→(%d,%d) X随机偏移(%d,%d) 实际(%d,%d)→(%d,%d) 时长=%dms",
		x1, y1, x2, y2, offset1, offset3, x1+offset1, y1, x2+offset3, y2, duration)
	motion.Swipe(x1+offset1, y1, x2+offset3, y2, duration, 0, 0)
}

// PinchZoom 双指缩放手势（游戏内镜头缩放）
// 参数:
//   - centerX, centerY: 缩放中心点坐标
//   - distance: 缩放距离（像素），正数表示两指向外张开，负数表示两指向内捏合
//   - duration: 手势持续时间（毫秒）
//
// 说明:
//   - 使用「水平方向」双指捏合/张开，避免对角线双指被游戏误判为平移/旋转（像单指滑动）。
//   - 正数：两指从靠近中心处向左右两侧移开（张开）
//   - 负数：两指从左右两侧向中心靠拢（捏合）
func PinchZoom(centerX, centerY, distance, duration int) {
	// 确保距离不为0
	if distance == 0 {
		return
	}

	var finger1StartX, finger1StartY, finger1EndX, finger1EndY int
	var finger2StartX, finger2StartY, finger2EndX, finger2EndY int

	// 屏幕边界（与游戏分辨率一致）
	screenWidth := 1280
	screenHeight := 720

	// 水平捏合：单侧最大可移动距离（保证两指 X 在屏幕内）
	maxHalf := centerX
	if screenWidth-centerX < maxHalf {
		maxHalf = screenWidth - centerX
	}
	if maxHalf < 1 {
		maxHalf = 1
	}

	if distance > 0 {
		// 张开：从略靠近中心到更远离中心（左右分开）
		d := distance
		if d > maxHalf {
			d = maxHalf
		}
		finger1StartY = centerY
		finger2StartY = centerY
		finger1EndY = centerY
		finger2EndY = centerY

		finger1StartX = centerX - d/2
		finger1EndX = centerX - d
		finger2StartX = centerX + d/2
		finger2EndX = centerX + d
	} else {
		// 捏合：从左右外侧向中心靠拢
		absDistance := -distance
		if absDistance > maxHalf {
			absDistance = maxHalf
		}

		finger1StartY = centerY
		finger2StartY = centerY
		finger1EndY = centerY
		finger2EndY = centerY

		finger1StartX = centerX - absDistance
		finger1EndX = centerX - absDistance/2
		finger2StartX = centerX + absDistance
		finger2EndX = centerX + absDistance/2
	}

	// Y 方向钳制在屏幕内（避免越界）
	cy := centerY
	if cy < 0 {
		cy = 0
	}
	if cy > screenHeight {
		cy = screenHeight
	}
	finger1StartY = cy
	finger1EndY = cy
	finger2StartY = cy
	finger2EndY = cy

	// 计算移动步数
	steps := duration / 16 // 假设每16ms移动一步（约60fps）
	if steps < 1 {
		steps = 1
	}

	// 计算每步的移动距离
	stepX1 := (finger1EndX - finger1StartX) / steps
	stepY1 := (finger1EndY - finger1StartY) / steps
	stepX2 := (finger2EndX - finger2StartX) / steps
	stepY2 := (finger2EndY - finger2StartY) / steps

	// 同时按下两个手指
	motion.TouchDown(finger1StartX, finger1StartY, 0, 0)
	motion.TouchDown(finger2StartX, finger2StartY, 1, 0)

	// 逐步移动两个手指
	stepDelay := duration / steps
	if stepDelay < 1 {
		stepDelay = 1
	}
	for i := 1; i <= steps; i++ {
		utils.Sleep(stepDelay)
		currentX1 := finger1StartX + stepX1*i
		currentY1 := finger1StartY + stepY1*i
		currentX2 := finger2StartX + stepX2*i
		currentY2 := finger2StartY + stepY2*i

		motion.TouchMove(currentX1, currentY1, 0, 0)
		motion.TouchMove(currentX2, currentY2, 1, 0)
	}

	// 同时抬起两个手指
	motion.TouchUp(finger1EndX, finger1EndY, 0, 0)
	motion.TouchUp(finger2EndX, finger2EndY, 1, 0)

	LogWithTimef("双指缩放完成: 中心点(%d,%d), 距离%d, 持续时间%dms", centerX, centerY, distance, duration)
}

// PressDPAD 按下方向键
// direction: "up", "down", "left", "right" 之一
func PressDPAD(direction string) {
	var keyCode int
	switch direction {
	case "up":
		keyCode = motion.KEYCODE_DPAD_UP
	case "down":
		keyCode = motion.KEYCODE_DPAD_DOWN
	case "left":
		keyCode = motion.KEYCODE_DPAD_LEFT
	case "right":
		keyCode = motion.KEYCODE_DPAD_RIGHT
	default:
		LogWithTimef("PressDPAD: 无效的方向参数: %s，支持的方向: up, down, left, right", direction)
		return
	}
	motion.KeyAction(keyCode, 0)
	LogWithTimef("PressDPAD: 按下方向键 %s (KEYCODE=%d)", direction, keyCode)
}

// ClickJumpThenUp 越障操作：先左手从上往下直滑50px(slideCount次)，再点击跳跃键(count次)，最后固定位置滑动(slideCount次，模拟推摇杆前进)
// 参数:
//   - jx, jy: 跳跃键坐标
//   - count: 跳跃次数
//   - slideCount: 滑动次数（前后两个滑动手势各重复的次数，等级越高可传越多；<1 按 1 处理）
func ClickJumpThenUp(jx, jy, count, slideCount int) {
	if slideCount < 1 {
		slideCount = 1
	}
	// 1. 跳跃前：左手从上往下直滑50px（X=274, Y 479→529）
	for i := 0; i < slideCount; i++ {
		motion.Swipe(274, 479, 274, 500, 500, 1, 0)
	}

	// 2. 点击跳跃键
	for i := 0; i < count; i++ {
		Click(jx, jy)
	}

	// 3. 固定位置滑动（模拟推摇杆前进）
	for i := 0; i < slideCount; i++ {
		motion.Swipe(274, 554, 274, 479, 500, 1, 0)
	}

	LogWithTimef("越障完成: Swipe(274,479→529)×%d + Click(%d,%d)×%d + Swipe(274,554→479)×%d", slideCount, jx, jy, count, slideCount)
}
