package main

import (
	"app/bot"
	"fmt"
	"log"
)

// 版本号在 assets/default.json 的 version 字段（core.Version），发布新版时改 json 里的值即可。
// 开发模式开关统一在 cardManager/cardkey/cardkey.go 顶部（dev 常量）：
// 发布时只需把 dev 改成 false，即同时恢复生产心跳并关闭所有开发行为（跳过热更、注入测试卡）。

// 交易行是否开启不再由这里硬编码：改由【检测交易行开启】插件（背包物品使用任务的一个步骤）
// 进商店检测"购买完成"实际判定，命中后全局置位，之后不再检测（见 plugin/检测交易行开启.go）。

func main() {

	// ====================================================
	/*	beibao.RunBeibaoRecognize()
		select {}
	*/
	/*
		if lvPtr, ok := core.RefreshLevel(); ok && lvPtr != nil {
			core.LogWithTimef("[Plugin] %s 等级刷新成功: %d")
		}
		core.LogWithTimef("[Plugin] %s 等级识别不到(缓存未刷新) → 返回 no")
		for true {
			f := core.Exec("等级")
			fmt.Println(f)

			core.RandomSleep(1000, 2000)
		}*/

	/*for true {
		imageHandler.SetDict("ziku.txt", "library")
		f2 := core.Exec("等级")
		fmt.Println(f2)

		core.RandomSleep(1000, 1000)
	}
	select {}*/
	// 0) 必须先加载字库！找字/取文字库都要，不 SetDict 直接调 C++ 端访问空字库 → 内存/段错误

	/*// Memory 库（libmem.so）连通性测试：读自身进程内存并回环校验
	core.MemSelfTest()

	// 金币监控：搜当前金币值，每 3 秒刷新，观察哪个地址跟着变
	core.MemWatchGold(1164826, 0)

	select {}*/

	fmt.Println("========== AutoGo 游戏自动化框架 ==========")

	// 0) 最先注入 RSA 公钥！
	// 必须在启动界面（ui.StartUI → UI_运行脚本 → cardkey.VerifyCard）之前注入：
	// VerifyCard 会同步联网激活并验签，若此时 pubKeyPEM 还是零值（空串），
	// 验签必然失败，会误报"响应验签失败，疑似假服务器"。
	// 公钥来自 assets/default.json 的 license_pubkey（go:embed 静态值），无任何运行时依赖。
	//cardkey.SetPublicKey(core.Config.GetString("license_pubkey"))
	// 先做热更新检查（热更过程用热更专用 HUD，不依赖主 HUD）：
	//   UpdateStateRestart  → 已触发更新重启，直接退出
	//   UpdateStateFailed   → 服务器不可达/下载失败，跳过卡密激活直接退出（避免无意义弹窗）
	//   UpdateStateUpToDate → 已是最新，继续卡密激活流程
	// 更新服务器地址来自 assets/default.json 的 update_url 字段
	// dev=true 跳过热更检查（本地联调）；dev=false 走热更检查（发布）
	// ==================== UI 前置面板 ====================
	// 启动后先弹出 UI 控制面板：
	//   "run"   → 用户点击「运行脚本」，继续走后面的主流程
	//   "close" → 用户点击「退出脚本」，直接结束进程
	// 用户填写的卡密+设备号保存在 ui 包内，通过 ui.RunConfig 读取。
	/*var uiCard, uiDeviceNo string
	readUIRunConfig := func() {
		if ui.RunConfig != nil {
			uiCard = ui.RunConfig.KaMi
			uiDeviceNo = ui.RunConfig.SheBeiHao
		}
		fmt.Printf("UI：卡密=%s 设备号=%s\n", uiCard, uiDeviceNo)
		cardkey.SetDeviceNo(uiDeviceNo)
	}

	if cardkey.IsDevMode() {
		fmt.Println("开发模式：跳过热更新检查")
	} else {
		switch ui.StartUI() {
		case "run":
			fmt.Println("UI：用户点击运行脚本，进入主流程")
		case "close":
			fmt.Println("UI：用户点击退出脚本，程序结束")
			return
		default:
			fmt.Println("UI：无动作返回，程序结束")
			return
		}
		readUIRunConfig()
		// ====================================================

		switch updater.CheckAndUpdate(core.Config.GetString("update_url")) {
		case updater.UpdateStateRestart:
			return
		case updater.UpdateStateFailed:
			// 热更失败：保留热更 HUD 一直显示失败原因（不退出、不关 HUD、不弹卡密激活），
			// 进程阻塞等待用户处理（修好服务器后重新启动脚本即可）
			fmt.Println("热更新失败，HUD 保持提示，等待用户处理")
			select {}
		}
	}*/

	// 卡密激活/校验：公钥已在 main() 开头注入（见上方步骤 0），这里直接进入激活流程。
	// 到期提醒文本由 StartHeartbeat 在 HUD 初始化后显示。

	// 热更新流程结束（已是最新），此时才初始化主 HUD，真机先不走hud 报错
	/*core.InitHUD()
	core.SetStatus("已启动，等待任务下发") // 青色状态*/
	/*core.RandomSleep(3000, 10000)*/

	// 2. 创建 Bot
	cfg := bot.DefaultBotConfig()
	b := bot.NewBot(cfg)

	// 3. 初始化
	if err := b.Init(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 4. 卡密激活/校验：用启动界面输入的卡密（uiCard）做激活
	//licenseTip := cardkey.CheckAndActivate(core.Config.GetString("license_server"), uiCard)
	// 5. 启动卡密心跳：维持授权在线窗口；任何心跳错误（拒绝/断网）回调 b.Stop 停止脚本
	// 换卡/确认卡密由每次启动的启动界面负责（ui.StartUI + cardkey.CheckAndActivate）
	// 注：bot 本地看门狗心跳（b.Beat / monitor）已停用，bot.Run 里 monitor.Start 已注释
	//go cardkey.StartHeartbeat(core.Config.GetString("license_server"), licenseTip, cardkey.CardHeartbeatInterval(), func() { b.Stop() })

	// 6. 启动中控通道（WS 上报 + 接收下发配置/命令）
	// 在 main 里装配，避免 bot/client 互相 import：
	//   - client 需要 bot 的能力（停止/重启/重置）→ 在这里把 bot 的方法注入 client
	//   - bot 需要 client 的上报能力 → 把 client.ReportFieldChanged 注入 bot
	//core.SetReportHook(client.ReportFieldChanged)
	//bot.SetWSReportHook(client.ReportFieldChanged)
	//client.SetStopCallback(b.Stop)
	//client.SetRestartCallback(func() { b.Exception().TriggerFullRestart("中控重启") })
	//client.SetResetScriptCallback(b.ResetScript)
	//go client.Start(core.Config.GetString("license_server"), uiCard, cardkey.WindowID(), cardkey.DeviceNo())

	// 7. 运行（阻塞）
	b.Run()
	fmt.Println("========== AutoGo 已退出 ==========")
}
