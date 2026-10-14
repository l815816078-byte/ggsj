package main

import (
	"app/core"
	"app/zt"
	"fmt"

	"github.com/Dasongzi1366/AutoGo/https"
)

// 版本号在 assets/default.json 的 version 字段（core.Version），发布新版时改 json 里的值即可。
// 开发模式开关统一在 cardManager/cardkey/cardkey.go 顶部（dev 常量）：
// 发布时只需把 dev 改成 false，即同时恢复生产心跳并关闭所有开发行为（跳过热更、注入测试卡）。

// 交易行是否开启不再由这里硬编码：改由【检测交易行开启】插件（背包物品使用任务的一个步骤）
// 进商店检测"购买完成"实际判定，命中后全局置位，之后不再检测（见 plugin/检测交易行开启.go）。

func main() {
	// ==================== 加护口袋 多点找色测试 ====================
	/*	{
		imageHandler.SetDict("ziku.txt", "library")
		r := core.Exec("加护口袋")
		fmt.Printf("[加护口袋测试] Found=%v 命中=(%d,%d)\n", r.Found, r.X, r.Y)
		select {}
	}*/
	// ==================== 每日任务 ExecAll 多结果测试 ====================
	/*	imageHandler.SetDict("ziku.txt", "library")
		pts := core.ExecAll("每日任务", 929, 99, 981, 371)
		fmt.Printf("[每日任务测试] 命中 %d 个\n", len(pts))
		for i, pt := range pts {
			fmt.Printf("  #%d 坐标=(%d,%d) X2,Y2=(%d,%d) Sim=%.2f Word=%s Type=%s\n",
				i+1, pt.X, pt.Y, pt.X2, pt.Y2, pt.Sim, pt.Word, pt.Type)
		}
		select {}*/
	/*{
		imageHandler.SetDict("ziku.txt", "library")
		for i := 0; i < 10; i++ {
			t0 := time.Now()
			r := core.Exec("自动攻击开启中")
			fmt.Printf("[多区域验证] Found=%v 命中坐标=(%d,%d) 耗时=%.2fms\n",
				r.Found, r.X, r.Y, float64(time.Since(t0).Microseconds())/1000.0)
			core.RandomSleep(500, 500)
		}
		select {}
	}*/

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
	code, data := https.Get("http://127.0.0.1:8989/float?x=0&y=623", 500)
	fmt.Printf("[HTTP测试] code=%d data=%s\n", code, string(data))
	// 热更新流程结束（已是最新），此时才初始化主 HUD，真机先不走hud 报错
	core.InitHUD()
	core.SetStatus("已启动，等待任务下发") // 青色状态
	/*core.RandomSleep(3000, 10000)*/

	// ══════════════════════════════════════════════════════════════════
	// 【zt 独立框架】——脱离老引擎（bot/scheduler/task/JSON 全部不走）
	//
	// 老引擎的启动代码整体停用（保留在下面注释里，随时可回退）：
	//   cfg := bot.DefaultBotConfig()
	//   b := bot.NewBot(cfg)
	//   if err := b.Init(); err != nil { log.Fatalf("初始化失败: %v", err) }
	//   b.Run()
	// ══════════════════════════════════════════════════════════════════

	// 跑 zt 主循环（= 老脚本 main.lua 的 _GameFlow，阻塞直到流程结束）
	// 字库加载在 zt.Run() 内部做（imageHandler 是 cgo 包，避免 main.go 直接依赖）
	zt.Run()

	fmt.Println("========== AutoGo 已退出 ==========")
}
