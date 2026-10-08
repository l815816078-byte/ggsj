package bot

import (
	"app/core"
	"app/task"
	"time"

	"github.com/Dasongzi1366/AutoGo/app"
	"github.com/Dasongzi1366/AutoGo/system"
)

// RecoveryContext 恢复上下文
// 管理游戏重启的全流程：kill→拉起→登录取 JSON 配置，通过引擎执行
type RecoveryContext struct {
	gamePackage string
	kitPackage  string
	loader      *task.Loader // 任务加载器（用于读取登录 JSON）

	loginMaxDuration time.Duration
}

// NewRecoveryContext 创建恢复上下文
func NewRecoveryContext(loader *task.Loader) *RecoveryContext {
	// 游戏包名统一从 app_packages.name 读取（default.json 配置），不存在硬编码兜底
	gamePkg := core.Config.GetString("app_packages.name")
	return &RecoveryContext{
		gamePackage:      gamePkg,
		kitPackage:       core.Config.GetString("app_packages.kit"),
		loader:           loader,
		loginMaxDuration: 5 * time.Minute,
	}
}

// RestartGame L2 重启游戏 —— 与 L3 一样也要起代理(代理不在游戏连不上服), 直接转调 FullRestart:
// kill 代理+游戏 → 启动代理(等起来→点开关) → 启动游戏 → 登录。
func (rc *RecoveryContext) RestartGame() bool {
	return rc.FullRestart()
}

// restartKit 重启代理(kit)统一入口: 杀掉 kit → 重新启动 → 等界面起来 → 点开关。
// 启动登录(StartupLogin) 和 全量重启(FullRestart) 都走这一套, 保证行为完全一致:
// 不再"检测到 kit 在跑就跳过"(那样 kit 进程活着但代理开关是关的时, 永远开不起来)。
// 返回是否成功启动; 没配 app_packages.kit 或启动失败 → 只记日志返回 false, 不阻断后面的登录流程。
func (rc *RecoveryContext) restartKit() bool {
	if rc.kitPackage == "" {
		core.LogWithTimef("[Recovery] 未配置代理包名(app_packages.kit) → 跳过启动代理")
		return false
	}
	// 1) 先杀干净, 避免旧实例/半开状态残留
	core.LogWithTimef("[Recovery] 停止代理: %s", rc.kitPackage)
	app.ForceStop(rc.kitPackage)
	core.RandomSleep(2000, 3000)

	// 2) 重新启动
	core.LogWithTimef("[Recovery] 启动代理: %s", rc.kitPackage)
	ok := app.Launch(rc.kitPackage, 0)
	if !ok {
		core.LogWithTimef("[Recovery] 代理启动失败(继续后面的流程)")
		return false
	}

	// 3) 等界面起来 → 点开关
	core.RandomSleep(3000, 5000)
	rc.clickKitToggle()
	return true
}

// FullRestart L3: kill代理+游戏→启动代理→启动游戏→登录
func (rc *RecoveryContext) FullRestart() bool {
	core.LogWithTimef("[Recovery] ===== L3 全量重启 =====")

	// 1. 杀掉游戏进程(代理由 restartKit 内部先杀再启)
	core.LogWithTimef("[Recovery] 停止游戏: %s", rc.gamePackage)
	app.ForceStop(rc.gamePackage)

	// 2. 重启代理(kit 那套: 杀 → 启动 → 等起来 → 点开关)
	rc.restartKit()

	// 3. 启动游戏
	core.LogWithTimef("[Recovery] 启动游戏: %s", rc.gamePackage)
	gameOk := app.Launch(rc.gamePackage, 0)
	if !gameOk {
		core.LogWithTimef("[Recovery] 游戏启动失败")
		return false
	}

	core.RandomSleep(10000, 15000)

	// 4. 登录流程 → 走 登录.json 配置
	return rc.doLogin()
}

// appRunning 应用进程是否在跑(不看是否在后台): system.GetPid(包名) 拿不到 pid 就是没在跑。
// 包名没配 → 直接算没在跑。
func appRunning(pkg string) bool {
	if pkg == "" {
		return false
	}
	return system.GetPid(pkg) > 0
}

// StartupLogin 程序启动时的"启动登录":
//
//	① kit(代理): 游戏进程存在 且 kit 进程也存在 → 不杀(避免打断正在运行的连接);
//	             否则 → 杀 → 重新启动 → 点开关(restartKit)
//	② 游戏没在跑 → 启动游戏(等 10~15 秒); 已经在跑 → 只 Launch 切回前台(等 3~5 秒), 不重启
//	③ 走 登录.json 登录
func (rc *RecoveryContext) StartupLogin() bool {
	// ① 代理: 游戏和 kit 都在跑 → 不杀(直接复用); 否则走 restartKit(杀 → 启动 → 点开关)
	if appRunning(rc.gamePackage) && appRunning(rc.kitPackage) {
		core.LogWithTimef("[Recovery] 游戏与代理均已在运行 → 不杀代理, 直接复用")
	} else {
		rc.restartKit()
	}

	// ② 游戏
	if appRunning(rc.gamePackage) {
		core.LogWithTimef("[Recovery] 游戏 %s 已在运行 → 切回前台(不重启)", rc.gamePackage)
		app.Launch(rc.gamePackage, 0)
		core.RandomSleep(3000, 5000)
	} else {
		core.LogWithTimef("[Recovery] 游戏没在跑 → 启动游戏: %s", rc.gamePackage)
		app.Launch(rc.gamePackage, 0)
		core.RandomSleep(10000, 15000)
	}

	// ③ 登录 → 走 登录.json 配置
	return rc.doLogin()
}

// doLogin 使用 登录.json 任务定义，通过引擎执行登录
func (rc *RecoveryContext) doLogin() bool {
	core.LogWithTimef("[Recovery] 开始登录流程 (最长 %v)", rc.loginMaxDuration)

	// 获取登录任务
	rt := rc.loader.Get("登录")
	if rt == nil {
		core.LogWithTimef("[Recovery] 找不到 登录 任务配置，登录失败")
		return false
	}

	// 重置状态 + 创建引擎 → MAA 统一 Execute()
	rt.Reset()
	engine := task.NewPipelineEngine(rt.Config())

	core.LogWithTimef("[Recovery] 开始场景检测...")
	result := engine.Execute(rt.RunState())

	if result.Success {
		core.LogWithTimef("[Recovery] 登录成功 (steps=%d)", result.ExecutedSteps)
	} else {
		core.LogWithTimef("[Recovery] 登录失败: %v (lastStep=%s, steps=%d)",
			result.Error, result.LastStep, result.ExecutedSteps)
	}
	return result.Success
}

// ───── 辅助函数 ─────

// clickKitToggle 循环点击代理开关(超时6秒、间隔2秒): 每轮先看是否已"代理开启中",
// 已开启 → 直接结束; 否则找"代理启动按钮"就点它, 点完等2秒进入下一轮, 直到看到"代理开启中"或超时。
func (rc *RecoveryContext) clickKitToggle() {
	deadline := time.Now().Add(6 * time.Second)
	for {
		if core.Exec("代理开启中").Found {
			core.LogWithTimef("[Recovery] 代理已'代理开启中' → 结束")
			return
		}
		r2 := core.Exec("代理启动按钮")
		if r2.Found {
			core.LogWithTimef("[Recovery] 点击代理启动按钮 (%d,%d) sim=%.2f", r2.X, r2.Y, r2.Sim)
			core.RandomClickInAreaWithDelay(r2.X, r2.Y, r2.X+5, r2.Y+5)
		} else {
			core.LogWithTimef("[Recovery] 代理界面里没找到'代理启动按钮'(recogs 预设'代理启动按钮') → 不点开关")
		}
		if time.Now().After(deadline) {
			core.LogWithTimef("[Recovery] 等待'代理开启中'超时(6秒) → 结束")
			return
		}
		core.RandomSleep(2000, 2000)
	}
}
