package plugin

import (
	"time"

	"app/core"
)

func init() {
	Register(&画册{})
}

// 画册 插件（入口步骤"进入画册"已点开画册界面）：
//
//	先找一次红点(13,71,840,706)
//	外层 for 红点(83,121,122,578) 数量6：
//	    内层 for（最多滑动5次结束）：
//	        内层 for 红点(139,170,1168,633) 数量10：点开后点【确认】→ 点【返回】
//	        滑动 612,347,650,401 从下往上
//
// 预设："红点" 走字库找字 + "寻找个数"（外层6/内层10），靠 core.ExecAll 返回全部红点。
type 画册 struct{}

func (p *画册) Name() string { return "画册" }

func (p *画册) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}

	core.LogWithTimef("[画册] 已进入画册界面 → 开始处理")

	// 先循环找红点(13,71,840,706)
	CheckLoopClick("红点", 6*time.Second, 2*time.Second, 13, 71, 840, 706)

	// 外层 for 红点(83,121,122,578) 数量6
	外层点 := core.ExecAll("红点", 83, 121, 122, 578)
	for _, op := range 外层点 {
		if IsStopped() {
			break
		}
		core.LogWithTimef("[画册] 外层红点(%d,%d) → 点击", op.X, op.Y)
		core.RandomClickInAreaWithDelay(op.X, op.Y, op.X+10, op.Y+10)

		// 内层 for：最多滑动 5 次结束
		for i := 1; i <= 5; i++ {
			if IsStopped() {
				break
			}
			// 内层 for 红点(139,170,1168,633) 数量10：点开后点【确认】→ 点【返回】
			内层点 := core.ExecAll("红点", 139, 170, 1168, 633)
			for _, ip := range 内层点 {
				if IsStopped() {
					break
				}
				core.LogWithTimef("[画册] 内层红点(%d,%d) → 点击", ip.X, ip.Y)
				core.RandomClickInAreaWithDelay(ip.X, ip.Y, ip.X+10, ip.Y+10)
				ClickAdaptive(2000, 1000, nil,
					btn("确认"),
					btn("绿色按钮", 978, 583, 1054, 649),
				)
				CheckLoopClick("返回", 6*time.Second, 2*time.Second)
			}
			// 滑动 604,315,664,453 从下往上
			cx := (604 + 664) / 2
			core.LogWithTimef("[画册] 从下往上滑动 (%d,%d)→(%d,%d) 第%d次", cx, 453, cx, 315, i)
			core.RandomSwipe(cx, 453, cx, 315, 1500)
		}
	}

	core.LogWithTimef("[画册] 处理完成")
	return "ok", nil
}
