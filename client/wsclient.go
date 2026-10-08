// Package wsclient 手机端中控通道客户端。
//
// 职责：
//  1. 主动连接服务端 /ws/script（反向连接，手机在 NAT 后无需公网端口）
//  2. 上线报全量、30 秒心跳保活、数据变化时立即上报
//  3. 接收服务端下发的配置（逐项合并写入 assets/脚本配置.json）与命令（停止/重启/重置脚本）
//
// 设计要点：
//   - 上报挂钩在 core.GetLevel / core.GetCachedTextInt 的成功分支（见 core/report.go）
//   - 配置下发后由本包写入文件，命令通过回调注入（避免 import 环）
package wsclient

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"app/core"
)

// ── 对外状态（供 core 上报挂钩读取）──

var (
	mu      sync.RWMutex
	conn    *websocket.Conn
	started bool
)

// IsConnected 当前中控连接是否在线
func IsConnected() bool {
	mu.RLock()
	defer mu.RUnlock()
	return conn != nil
}

// ── 命令回调（由 bot 注入，避免 client → bot 的 import 环）──

var (
	stopCallback        func()
	restartCallback     func()
	resetScriptCallback func()
)

// SetStopCallback 注入「停止脚本」实现（bot.Stop）
func SetStopCallback(f func()) { stopCallback = f }

// SetRestartCallback 注入「重启脚本」实现（bot 全量重启）
func SetRestartCallback(f func()) { restartCallback = f }

// SetResetScriptCallback 注入「重置脚本」实现（重置任务状态）
func SetResetScriptCallback(f func()) { resetScriptCallback = f }

// ── 协议结构 ──

// wsMsg 与 C# 端契约一致：{"type": "...", "params": {...}}
type wsMsg struct {
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// reportParams 全量上报字段（识别缺失的字段留空/0，见方案"识别缺失"说明）
type reportParams struct {
	Server  string `json:"server"`
	Role    string `json:"role"`
	Job     string `json:"job"`
	Level   int    `json:"level"`
	Power   int    `json:"power"`
	Gold    int64  `json:"gold"`
	Diamond int64  `json:"diamond"`
	BagFree int    `json:"bagFree"`
	State   string `json:"state"`
}

// 最近一次已上报值，用于「变化才报」判断
var lastReport reportParams
var lastReportMu sync.Mutex

// ── 连接主流程 ──

// Start 连接中控服务端并进入收发循环（阻塞，自动重连）。
//   baseURL 形如 http://111.231.167.150:8080/
//   card/window/device 为身份参数
func Start(baseURL, card, windowID, deviceNo string) {
	wsURL, err := buildWSURL(baseURL, card, windowID, deviceNo)
	if err != nil {
		core.LogWithTimef("[中控] WS 地址无效: %v", err)
		return
	}

	backoff := time.Second
	for {
		core.LogWithTimef("[中控] 正在连接 %s", wsURL)
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			core.LogWithTimef("[中控] 连接失败: %v，%v 后重试", err, backoff)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second

		mu.Lock()
		conn = c
		started = true
		mu.Unlock()

		core.LogWithTimef("[中控] 已连接，开始上报")

		// 上线立即全量上报一次
		go func() {
			// 首次上线：等级/战力等可能读不到，读到啥报啥
			ReportNow(true)
		}()

		// 30 秒心跳 + 数据变化检测
		stopHb := make(chan struct{})
		go heartbeatLoop(c, stopHb)

		// 接收循环
		readLoop(c)

		close(stopHb)
		mu.Lock()
		conn = nil
		mu.Unlock()
		core.LogWithTimef("[中控] 连接断开，准备重连")

		time.Sleep(backoff)
	}
}

// buildWSURL 把 http(s) base 转成 ws(s) 并带上身份参数
func buildWSURL(baseURL, card, windowID, deviceNo string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/ws/script"
	q := u.Query()
	q.Set("card", card)
	q.Set("window", windowID)
	q.Set("device", deviceNo)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// heartbeatLoop 30 秒心跳保活
func heartbeatLoop(c *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := writeMsg(c, wsMsg{Type: "heartbeat"}); err != nil {
				return
			}
		case <-stop:
			return
		}
	}
}

// readLoop 接收服务端下发
func readLoop(c *websocket.Conn) {
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		var m wsMsg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		switch m.Type {
		case "config":
			handleConfig(c, m.Params)
		case "cmd":
			handleCmd(c, m.Params)
		}
	}
}

// handleConfig 收到下发配置：逐项合并写入手机端脚本配置，然后回执。
// 按方案约定：配置改动「重启后生效」，这里只落盘不热加载。
func handleConfig(c *websocket.Conn, params map[string]interface{}) {
	schemeName, _ := params["schemeName"].(string)
	configJson, _ := params["configJson"].(string)

	core.LogWithTimef("[中控] 收到配置方案 %q，长度=%d", schemeName, len(configJson))

	if err := MergeAndSaveLocalConfig(configJson); err != nil {
		core.LogWithTimef("[中控] 配置写入失败: %v", err)
		return
	}

	_ = writeMsg(c, wsMsg{Type: "configApplied", Params: map[string]interface{}{
		"schemeName": schemeName,
	}})
	core.LogWithTimef("[中控] 配置已写入，重启脚本后生效")
}

// handleCmd 收到命令
func handleCmd(c *websocket.Conn, params map[string]interface{}) {
	cmd, _ := params["cmd"].(string)
	core.LogWithTimef("[中控] 收到命令: %s", cmd)

	switch cmd {
	case "stop":
		if stopCallback != nil {
			stopCallback()
		}
	case "restart":
		if restartCallback != nil {
			restartCallback()
		}
	case "resetScript":
		if resetScriptCallback != nil {
			resetScriptCallback()
		}
	default:
		core.LogWithTimef("[中控] 未知命令: %s", cmd)
		return
	}

	_ = writeMsg(c, wsMsg{Type: "cmdAck", Params: map[string]interface{}{"cmd": cmd}})
}

// ── 上报 ──

// ReportNow 立即上报一次当前数据。
//   force=true 时忽略「是否变化」判断，强制上报（上线时用）
func ReportNow(force bool) {
	mu.RLock()
	c := conn
	mu.RUnlock()
	if c == nil {
		return
	}

	cur := collectReport()

	lastReportMu.Lock()
	changed := force || cur != lastReport
	if changed {
		lastReport = cur
	}
	lastReportMu.Unlock()
	if !changed {
		return
	}

	params := map[string]interface{}{
		"server":  cur.Server,
		"role":    cur.Role,
		"job":     cur.Job,
		"level":   cur.Level,
		"power":   cur.Power,
		"gold":    cur.Gold,
		"diamond": cur.Diamond,
		"bagFree": cur.BagFree,
		"state":   cur.State,
	}
	if err := writeMsg(c, wsMsg{Type: "report", Params: params}); err != nil {
		core.LogWithTimef("[中控] 上报失败: %v", err)
	}
}

// ReportFieldChanged 在识别到某个数值字段时调用（由 core 上报挂钩触发）：
// 只做轻量的「标记有变化 + 触发一次上报」，实际值仍从全局读取。
func ReportFieldChanged(field string) {
	ReportNow(false)
}

// collectReport 从全局读取当前可上报数据（识别不到的字段为空/0）
func collectReport() reportParams {
	r := reportParams{State: currentState()}

	// 等级：读缓存（不触发截图识别，避免上报本身产生识别开销）
	if v, ok := core.PeekCachedInt("等级"); ok {
		r.Level = v
	}
	// 战力：同样读缓存
	if v, ok := core.PeekCachedInt("战力"); ok {
		r.Power = v
	}
	// 区服/角色/职业/金币/钻石/背包剩余：recogs 暂无预设，先占位（读到啥报啥）
	// TODO 待用户提供识别区域后补齐 collectReport 的读取
	return r
}

// currentState 当前脚本状态（供上报）
func currentState() string {
	if core.IsStopped() {
		return "stopped"
	}
	return "running"
}

// writeMsg 带锁发送一条 JSON（Conn 写操作不并发安全）
var writeMu sync.Mutex

func writeMsg(c *websocket.Conn, m wsMsg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	return c.WriteMessage(websocket.TextMessage, b)
}

// ── 调试辅助 ──

// Logf 统一日志前缀（便于和中控日志区分）
func Logf(format string, args ...interface{}) {
	core.LogWithTimef("[中控] "+fmt.Sprintf(format, args...))
}
