package util

import "github.com/Dasongzi1366/AutoGo/utils"

// RandomSleep 在最小和最大之间随机休眠指定的毫秒数。
// 参数:
// - min: 最小休眠时间，单位为毫秒。
// - max: 最大休眠时间，单位为毫秒。
// 返回:
// - 无返回值。
func RandomSleep(min, max int) {
	randNum := utils.Random(min, max)
	utils.Sleep(randNum)
}
