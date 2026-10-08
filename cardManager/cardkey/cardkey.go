package cardkey

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Dasongzi1366/AutoGo/https"

	"app/core"
)

// ── 开发模式统一配置（全项目唯一开关，发布时只改 dev 一处）──
// dev = true  开发模式：跳过热更检查 + 测试卡 + 短心跳（本地联调用）
// dev = false 生产模式：走热更检查 + 正式卡密弹窗 + 生产心跳（发布用）
// dev 用 var（不用 const），避免编译期恒真/恒假提示；只在本包内读。
var dev = false

const (
	// TestCard 开发模式写死的测试卡密（dev=false 时忽略，正常走卡密输入弹窗）
	TestCard = "T6KV8L72"

	// HeartbeatInterval 开发模式卡密心跳间隔：设短方便观察心跳日志
	HeartbeatInterval = 10 * time.Second

	// ProdHeartbeatInterval 生产卡密心跳间隔
	ProdHeartbeatInterval = 5 * time.Minute
)

// IsDevMode 是否开发模式（开发调试统一判断入口，全项目都用它，不要再写别的判断）
func IsDevMode() bool { return dev }

// TestCardFor 当前生效的测试卡密：开发模式返回 TestCard（跳过卡密输入弹窗），生产返回空串（空串 = 正常弹窗输入）
func TestCardFor() string {
	if !dev {
		return ""
	}
	return TestCard
}

// CardHeartbeatInterval 卡密心跳间隔：开发模式用短间隔，生产固定 10 分钟
func CardHeartbeatInterval() time.Duration {
	if !dev {
		return ProdHeartbeatInterval
	}
	return HeartbeatInterval
}

// 内置 RSA 公钥 PEM（SubjectPublicKeyInfo，BEGIN PUBLIC KEY）。
// 由 main.go 从 assets/default.json 的 license_pubkey 注入。
var pubKeyPEM string

// windowID 窗口 ID：每次进程启动随机生成（服务器在线窗口表的键，多开/重启即新窗口）。
// 与 C# 端 WindowTable 契约一致：window_id 由设备端提供，服务器按卡号分组统计。
var windowID = newWindowID()

func newWindowID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 极不可能失败，回退时间戳保证唯一
		return fmt.Sprintf("w%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// WindowID 读取当前进程的窗口 ID（中控 WS 注册用，与卡密心跳同一标识）
func WindowID() string {
	return windowID
}

// SetPublicKey 注入服务器 RSA 公钥（来自配置 license_pubkey）
func SetPublicKey(pemStr string) {
	pubKeyPEM = pemStr
}

// deviceNo 设备号：启动界面（ui 包）输入，同一卡密内唯一。
// 用于中控识别设备；当前先随激活请求上报，服务器侧校验后续再做。
var deviceNo string

// SetDeviceNo 注入启动界面输入的设备号
func SetDeviceNo(no string) {
	deviceNo = strings.TrimSpace(no)
}

// DeviceNo 读取当前设备号
func DeviceNo() string {
	return deviceNo
}

// 服务器响应结构（与 C# 契约一致）
type cardResp struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Card   string `json:"card"`
	Expire int64  `json:"expire"`
	MaxWin int    `json:"maxwin"`
	Sig    string `json:"sig"`
}

// normURL 保证 base URL 以 / 结尾
func normURL(u string) string {
	if u == "" {
		return ""
	}
	if u[len(u)-1] != '/' {
		return u + "/"
	}
	return u
}

func fmtTime(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02")
}

// expireTip 到期提醒：剩余 <=3 天黄字提示续费；已到期红字提示
func expireTip(expire int64) string {
	days := (expire - time.Now().Unix()) / 86400
	if days <= 0 {
		return "卡密已到期，请联系购买方续费"
	}
	if days <= 3 {
		return fmt.Sprintf("卡密剩余 %d 天，请联系购买方续费", days)
	}
	return ""
}

// callCard POST JSON 到卡密接口；返回 (httpCode, respBody, netErr)
func callCard(base, path string, body map[string]string) (int, []byte, bool) {
	data, _ := json.Marshal(body)
	code, resp := https.Post(base+path, data, map[string]string{"Content-Type": "application/json"}, 10000)
	if code != 200 || len(resp) == 0 {
		return -1, nil, true
	}
	return code, resp, false
}

// activate 激活（已激活卡再次激活 = 续费）；返回 (ok, expire, maxwin, msg)
func activate(base, code, fp string) (bool, int64, int, string) {
	_, resp, netErr := callCard(base, "api/card/activate", map[string]string{
		"code": code, "fp": fp, "window_id": windowID, "device_no": deviceNo,
	})
	if netErr {
		return false, 0, 0, "网络错误，无法连接服务器"
	}
	var r cardResp
	if err := json.Unmarshal(resp, &r); err != nil {
		return false, 0, 0, "服务器响应异常"
	}
	if r.Code != 0 {
		return false, 0, 0, r.Msg
	}
	if !VerifyResp(r.Code, r.Card, r.Expire, r.MaxWin, r.Sig) {
		return false, 0, 0, "响应验签失败，疑似假服务器"
	}
	return true, r.Expire, r.MaxWin, "ok"
}

// heartbeat 心跳；返回 (ok, rejected)：
//
//	ok        心跳成功（code==0）
//	rejected  服务器明确拒绝（卡不存在/禁用/未激活/窗口已满），无需重试
//	!ok && !rejected 网络错误或响应异常
func heartbeat(base, code string) (bool, bool) {
	_, resp, netErr := callCard(base, "api/card/heartbeat", map[string]string{
		"code": code, "window_id": windowID,
	})
	if netErr {
		return false, false
	}
	var r cardResp
	if err := json.Unmarshal(resp, &r); err != nil {
		return false, false
	}
	return r.Code == 0, r.Code != 0
}

// activateWithCard 用指定卡密激活：激活成功返回 (expire, maxwin)。
// 卡密来自启动界面（ui 包）输入，不再弹窗。
// 激活失败（卡密无效/禁用/到期/断网）：HUD 红字常驻显示原因，阻塞等待用户重启脚本。
func activateWithCard(base, fp, card string) (int64, int) {
	card = strings.TrimSpace(card)
	if card == "" {
		fmt.Println("[cardkey] 卡密为空")
		core.SetUpdateLogsColor("#FF0000", "未输入卡密\n请重启脚本在启动界面输入")
		select {}
	}
	okAct, expire, maxwin, msg := activate(base, card, fp)
	if okAct {
		if err := SaveLicense(card, expire, maxwin, fp); err != nil {
			fmt.Println("[cardkey] 保存本地授权失败:", err)
		}
		fmt.Println("[cardkey] 激活成功，到期:", fmtTime(expire))
		return expire, maxwin
	}
	// 激活失败：红字常驻显示原因，阻塞等待用户重启（重启后可在启动界面换卡密）
	fmt.Println("[cardkey] 激活失败:", msg)
	core.SetUpdateLogsColor("#FF0000", "卡密激活失败 "+msg)
	select {}
	return 0, 0 // 不可达
}

// VerifyCard 供启动界面（ui 包）点击「运行脚本」时同步调用：
// 用卡密联网激活/校验，成功则保存本地授权并返回 (expire, "", true)；
// 失败返回 (0, 错误原因, false)，【不阻塞、不弹窗】，由 UI 决定如何提示（红字 + Toast）。
// 校验通过后主流程的 CheckAndActivate 会直接命中本地授权，不会重复请求。
func VerifyCard(serverURL, card string) (int64, string, bool) {
	base := normURL(serverURL)
	if base == "" {
		fmt.Println("[cardkey] license_server 未配置，跳过卡密校验")
		return 0, "", true
	}
	card = strings.TrimSpace(card)
	if card == "" {
		return 0, "请输入卡密", false
	}
	fp := DeviceID()
	okAct, expire, maxwin, msg := activate(base, card, fp)
	if !okAct {
		fmt.Println("[cardkey] 界面校验卡密失败:", msg)
		return 0, msg, false
	}
	if err := SaveLicense(card, expire, maxwin, fp); err != nil {
		fmt.Println("[cardkey] 保存本地授权失败:", err)
	}
	fmt.Println("[cardkey] 界面校验卡密成功，到期:", fmtTime(expire))
	return expire, "", true
}

// CheckAndActivate 卡密激活/校验主入口（在热更检查之后、InitHUD 之前调用）。
// 卡密由启动界面（ui 包）输入并通过参数传入，不再弹 InputAlert 输入框。
// 通过校验则保存授权继续启动；失败/断网/空输入 → 每5秒 Toast 显示原因，阻塞等用户重启脚本。
// 返回到期提醒文本（空串=无提醒），由 StartHeartbeat 在 HUD 就绪后显示。
func CheckAndActivate(serverURL, card string) string {
	base := normURL(serverURL)
	if base == "" {
		fmt.Println("[cardkey] license_server 未配置，跳过卡密校验")
		return ""
	}
	fp := DeviceID()

	// ── 开发调试：dev=true 时先用写死测试卡激活 ──
	// 卡密一律从 UI 传入（开发模式若 UI 未填则回退到写死测试卡）
	if code := TestCardFor(); code != "" {
		fmt.Println("[cardkey] 开发模式：使用写死卡密激活")
		expire, _ := activateWithCard(base, fp, code)
		return expireTip(expire)
	}

	// ── 正式模式：用启动界面输入的卡密做激活/校验 ──
	fmt.Println("[cardkey] 使用启动界面输入的卡密激活")
	expire, _ := activateWithCard(base, fp, card)
	return expireTip(expire)
}

// StartHeartbeat 启动心跳 goroutine（在 bot 运行前调用，非阻塞）。
// interval 为卡密心跳间隔，由调用方传入（生产 10 分钟，开发模式用 CardHeartbeatInterval()）。
// 每轮最多试 2 次，连续 2 次失败判定断网。
// 任何卡密心跳错误（服务器拒绝/断网）一律红字提示后回调 onStop 停止脚本；
// 换卡/确认卡密走启动流程（每次启动必弹窗，见 CheckAndActivate）。
// tip 为 CheckAndActivate 返回的到期提醒，HUD 初始化完成后显示。
func StartHeartbeat(serverURL string, tip string, interval time.Duration, onStop func()) {
	base := normURL(serverURL)
	if base == "" {
		return
	}
	go func() {
		// 等主 HUD 初始化完成再显示到期提醒
		if tip != "" {
			time.Sleep(3 * time.Second)
			core.SetStatusColor("#F59E0B", tip)
		}
		fmt.Println("[cardkey] 心跳间隔", interval)
		for {
			time.Sleep(interval)
			lic, err := LoadLicense()
			if err != nil {
				continue // 未激活不心跳
			}
			// 每轮最多试 2 次（第 2 次间隔 5 秒），连续 2 次失败判定断网
			failCount := 0
			for attempt := 0; attempt < 2; attempt++ {
				if attempt > 0 {
					time.Sleep(5 * time.Second)
				}
				ok, rejected := heartbeat(base, lic.Card)
				if rejected {
					// 服务器明确拒绝（禁用/解绑/窗口满）：任何卡密心跳错误一律红字提示后停止脚本。
					// 不在心跳 goroutine 弹窗（InputAlert 只能主线程弹），用户重启脚本后启动界面重新输入。
					fmt.Println("[cardkey] 服务器拒绝心跳，停止脚本")
					core.SetUpdateLogsColor("#FF0000", "服务器拒绝心跳\n请检查卡密状态")
					time.Sleep(3 * time.Second)
					if onStop != nil {
						onStop()
					}
					return
				}
				if ok {
					failCount = 0
					break
				}
				failCount++
			}
			if failCount >= 2 {
				// 连续 2 次失败 = 断网：红字常驻显示原因，稍候停止脚本
				fmt.Println("[cardkey] 心跳连不上服务器，停止脚本")
				core.SetUpdateLogsColor("#FF0000", "心跳连不上服务器\n请检查网络")
				time.Sleep(3 * time.Second)
				if onStop != nil {
					onStop()
				}
				return
			}
		}
	}()
}
