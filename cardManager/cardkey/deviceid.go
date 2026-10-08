package cardkey

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Dasongzi1366/AutoGo/device"
)

// DeviceID 计算设备指纹：AndroidId + Serial + Model + Brand 拼接后 SHA-256 hex。
// 用于区分同一张卡在不同设备上的占用，以及 .lic 换设备检测（方案 A 不绑设备，
// 但服务器要记录是哪个设备在占用窗口）。
func DeviceID() string {
	raw := device.GetAndroidId() + "|" + device.Serial + "|" + device.Model + "|" + device.Brand
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
