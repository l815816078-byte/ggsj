// Package updater 实现程序整包热更新。
// 流程：启动时检查服务器版本号 → 有新版则下载新程序 → 覆盖自身 → 重启。
// 热更新日志统一用 core.SetUpdateLogs 打印到热更 HUD（同时输出控制台日志）。
// 用法：在 main() 最前面调用 CheckAndUpdate，按返回状态码决定后续流程
// （UpdateStateRestart 退出、UpdateStateFailed 跳过卡密激活、UpdateStateUpToDate 继续）。
package updater

import (
	"fmt"

	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Dasongzi1366/AutoGo/files"
	"github.com/Dasongzi1366/AutoGo/system"

	"app/core"
	"app/util"
)

// CheckAndUpdate 的返回状态码
const (
	UpdateStateUpToDate = iota // 已是最新版本，无需更新（调用方继续后续流程）
	UpdateStateRestart         // 已下载新版本并触发重启自身（调用方应退出）
	UpdateStateFailed          // 热更新失败（服务器不可达/下载失败，调用方应跳过卡密激活）
)

// CheckAndUpdate 检查并执行热更新。
// baseURL: 更新服务器地址，如 "http://127.0.0.1:8080"
// 服务器需提供: <baseURL>/version.txt (纯文本版本号)、<baseURL>/app (新版本程序)、<baseURL>/size (可选，分片用)
// 当前版本号统一读 core.Version（值来自 assets/default.json 的 version 字段，调用方无需传参）。
// 返回 UpdateStateRestart 表示已下载新版本并触发重启自身（调用方应停止后续逻辑）；
// 返回 UpdateStateFailed 表示热更新失败（调用方应跳过卡密激活，避免无意义的弹窗）。
func CheckAndUpdate(baseURL string) int {
	// 归一化 baseURL：去掉尾部斜杠，避免拼出 "//version.txt" 双斜杠路径导致 404
	baseURL = strings.TrimRight(baseURL, "/")

	// 开启日志落盘：HUD 万一显示不出来，关键信息仍可在
	// /sdcard/autogo_logs/log_日期.txt 里查到
	core.IsLog = true

	fmt.Println("========== 热更新检查 ==========")
	fmt.Println("当前版本: " + core.Version)
	core.LogWithTimef("[热更] 开始检查 base=%s 当前版本=%s", baseURL, core.Version)
	core.InitUpdateHUD()
	core.SetUpdateLogs("热更新检查中... 当前 v" + core.Version)

	// 1. 拉取服务器版本号，超时 5 秒
	code, data := util.HttpRequest.Get(baseURL+"/version.txt", 5000)
	if code != 200 {
		fmt.Println("版本检查失败, HTTP code:", code)
		core.SetUpdateLogs("热更新失败: HTTP " + strconv.Itoa(code) + " 服务器未启动?")
		core.LogWithTimef("[热更] 版本检查失败 HTTP=%d url=%s", code, baseURL+"/version.txt")
		return UpdateStateFailed
	}

	remoteVer := strings.TrimSpace(string(data))
	fmt.Println("服务器版本: " + remoteVer)
	if remoteVer == "" {
		fmt.Println("服务器版本号为空, 跳过更新")
		core.SetUpdateLogs("服务器版本号为空")
		core.LogWithTimef("[热更] 服务器版本号为空")
		return UpdateStateFailed
	}
	core.LogWithTimef("[热更] 服务器版本=%s 当前版本=%s", remoteVer, core.Version)

	// 防循环兜底：优先读本地标记文件（可执行文件旁 .ver，更新过一次就有）。
	// 只要本地标记 == 服务器版本，就认为已是最新——即使代码里 Version 写错，
	// 一台设备也最多只会下载一次，绝不会无限循环烧流量。
	localVer := readLocalVer()
	if localVer != "" && localVer == remoteVer {
		fmt.Println("已是最新版本(本地标记), 无需更新")
		core.SetUpdateLogs("已是最新版本 v" + localVer)
		core.RandomSleep(3000, 3000)
		core.DestroyUpdateHUD()
		return UpdateStateUpToDate
	}

	// 首次运行（无标记）：用代码内 core.Version 判断
	if remoteVer == core.Version {
		fmt.Println("已是最新版本, 无需更新")
		core.SetUpdateLogs("已是最新版本 v" + core.Version)
		core.RandomSleep(3000, 3000)
		core.DestroyUpdateHUD()
		return UpdateStateUpToDate
	}

	// 2. 有新版本，分片下载（进度实时刷新 HUD 与控制台）
	const chunkSize = 1024 * 1024 // 每片 1MB

	fmt.Println("发现新版本: " + remoteVer + ", 开始下载...")
	core.SetUpdateLogs("发现新版本 v" + remoteVer + "，开始下载...")

	// 2.1 先取文件总大小（新服务器支持；旧服务器返回 404 时退回整包下载）
	total := 0
	code, sizeData := util.HttpRequest.Get(baseURL+"/size", 5000)
	if code == 200 {
		total, _ = strconv.Atoi(strings.TrimSpace(string(sizeData)))
	}

	if total > 0 {
		// 分片下载，每片完成后刷新 HUD 百分比
		data = make([]byte, 0, total)
		for start := 0; start < total; {
			end := start + chunkSize
			if end > total {
				end = total
			}
			code, chunk := util.HttpRequest.Get(
				baseURL+"/app?start="+strconv.Itoa(start)+"&end="+strconv.Itoa(end), 0)
			if code != 200 || len(chunk) != end-start {
				fmt.Println("下载分片失败, HTTP code:", code)
				core.SetUpdateLogs("下载失败: HTTP " + strconv.Itoa(code) + " 服务器未启动?")
				core.LogWithTimef("[热更] 下载分片失败 HTTP=%d start=%d end=%d 收到=%d", code, start, end, len(chunk))
				return UpdateStateFailed
			}
			data = append(data, chunk...)
			pct := len(data) * 100 / total
			fmt.Printf("下载中... %d%% (%s/%s)\n", pct, formatSize(len(data)), formatSize(total))
			core.SetUpdateLogs("下载中 " + fmt.Sprintf("%d%% (%s/%s)", pct, formatSize(len(data)), formatSize(total)))
			start = end
		}
	} else {
		// 兼容旧服务器：整包下载
		code, data = util.HttpRequest.Get(baseURL+"/app", 0)
		if code != 200 {
			fmt.Println("下载新版本失败, HTTP code:", code)
			core.SetUpdateLogs("下载失败: HTTP " + strconv.Itoa(code) + " 服务器未启动?")
			core.LogWithTimef("[热更] 整包下载失败 HTTP=%d url=%s", code, baseURL+"/app")
			return UpdateStateFailed
		}
	}

	fmt.Println("下载完成, 大小:", formatSize(len(data)))
	core.SetUpdateLogs("下载完成 " + formatSize(len(data)) + "，重启中...")

	// 3. 先写本地版本标记（重启后标记==服务器版本即跳过，防死循环），再覆盖自身并重启。
	// 覆盖目标用 executablePath() 的真实绝对路径：os.Args[0] 在安卓上常是相对路径，
	// RestartSelf 重启后工作目录会变，用相对路径会覆盖不到真正的可执行文件。
	writeLocalVer(remoteVer)
	exe := executablePath()
	fmt.Println("覆盖可执行文件:", exe)
	files.WriteBytes(exe, data)
	system.RestartSelf()
	return UpdateStateRestart
}

// executablePath 返回当前脚本的真实绝对路径（更新/标记文件都基于它）。
// 注意：不能用 os.Executable()——安卓上 /proc/self/exe 指向 app_process64
// （返回 /system/bin/app_process64，只读分区且不是真正脚本），用了会写失败、覆盖错文件。
// os.Args[0] 是用户实际执行脚本时给的路径（如 adb shell /data/local/tmp/app 时是绝对路径），
// 相对路径时基于当前工作目录解析成绝对路径。
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

// localVerFile 返回设备本地版本标记文件路径（可执行文件旁 .ver）
// 例如 /data/local/tmp/app → /data/local/tmp/app.ver
func localVerFile() string {
	return executablePath() + ".ver"
}

// readLocalVer 读取设备本地已生效的版本号；没有标记文件时返回空串
func readLocalVer() string {
	b, err := os.ReadFile(localVerFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writeLocalVer 把刚更新到的版本号写入本地标记文件。
// 必须用标准库 os.WriteFile：与 readLocalVer 的 os.ReadFile 读写同一份文件，
// 走引擎 files 包的话路径处理可能不一致，防循环会失效。
func writeLocalVer(v string) {
	if err := os.WriteFile(localVerFile(), []byte(v), 0644); err != nil {
		fmt.Println("写入本地版本标记失败:", err)
	}
}

// formatSize 把字节数格式化为可读大小，如 34.6MB
func formatSize(n int) string {
	const mb = 1024 * 1024
	if n >= mb {
		return fmt.Sprintf("%.1fMB", float64(n)/float64(mb))
	}
	return strconv.Itoa(n) + "B"
}
