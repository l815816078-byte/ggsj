package cardkey

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// License 本地授权内容（加密存储为 .lic 文件）
type License struct {
	Card   string `json:"card"`   // 卡号
	Expire int64  `json:"expire"` // 到期时间（Unix 秒）
	MaxWin int    `json:"maxwin"` // 窗口数
	FPHash string `json:"fp"`     // 设备指纹 hash，用于检测换设备
}

// executablePath 返回当前脚本的真实绝对路径（与 updater 同一套逻辑）。
// 不能用 os.Executable()：安卓上 /proc/self/exe 指向 app_process64。
// os.Args[0] 是实际执行路径，相对路径时基于工作目录解析成绝对路径。
func executablePath() string {
	arg0 := os.Args[0]
	if filepath.IsAbs(arg0) {
		return filepath.Clean(arg0)
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, arg0)
	}
	return arg0
}

// licFilePath 本地授权文件路径：可执行文件旁 app.lic（与热更的 app.ver 相邻）
func licFilePath() string {
	return executablePath() + ".lic"
}

// SaveLicense 加密保存本地授权
func SaveLicense(card string, expire int64, maxwin int, fp string) error {
	data, err := json.Marshal(License{Card: card, Expire: expire, MaxWin: maxwin, FPHash: fp})
	if err != nil {
		return err
	}
	enc, err := aesGCMEncrypt(data, deriveKey(fp))
	if err != nil {
		return err
	}
	return os.WriteFile(licFilePath(), enc, 0644)
}

// LoadLicense 读取并解密本地授权；文件不存在/解密失败/损坏时返回错误
// （换设备时密钥不同会解密失败 → 调用方走重新激活流程）
func LoadLicense() (*License, error) {
	data, err := os.ReadFile(licFilePath())
	if err != nil {
		return nil, err
	}
	// 先尝试当前设备指纹解密；失败时（换设备）返回明确错误
	plain, err := aesGCMDecrypt(data, deriveKey(DeviceID()))
	if err != nil {
		return nil, fmt.Errorf("授权文件解密失败（可能已更换设备）: %v", err)
	}
	var lic License
	if err := json.Unmarshal(plain, &lic); err != nil {
		return nil, err
	}
	return &lic, nil
}
