package main

import (
	"fmt"
	"time"

	"app/core"
	"app/imageHandler"
)

// ============================================================================
// OCR 区域耗时对比（go run ocr耗时对比.go）
//
// 用"自动攻击开启中"预设，分别传 3 个区域识别，对比耗时：
//   区域1: 1007,637,1057,682
//   区域2: 582,436,618,470
//   区域3: 578,428,1126,705（合并大区域）
//
// 每个区域跑 N 次，打印 平均耗时 / 总耗时 / 命中数。
// 注意：命中=0 时耗时极低（没找到提前退出），不能当"快"的依据。
// ============================================================================

// 测试配置：名称 + 区域 + 次数
var 测试区域 = []struct {
	名 string
	区 [4]int
	次 int
}{
	{"区域1(小)", [4]int{1007, 637, 1057, 682}, 20},
	{"区域2(小)", [4]int{582, 436, 618, 470}, 20},
	{"区域3(大)", [4]int{578, 428, 1126, 705}, 20},
}

func main() {
	if !imageHandler.SetDict("ziku.txt", "library") {
		fmt.Println("字库加载失败")
		return
	}
	for _, item := range 测试区域 {
		var 总 time.Duration
		命中 := 0
		for i := 0; i < item.次; i++ {
			t0 := time.Now()
			r := core.Exec("自动攻击开启中", item.区[0], item.区[1], item.区[2], item.区[3])
			总 += time.Since(t0)
			if r.Found {
				命中++
			}
		}
		fmt.Printf("[耗时对比] %s 区域=%d,%d,%d,%d  次数=%d  命中=%d  平均=%.2fms  总=%.2fms\n",
			item.名, item.区[0], item.区[1], item.区[2], item.区[3], item.次, 命中,
			float64(总.Microseconds())/float64(item.次)/1000.0, float64(总.Microseconds())/1000.0)
	}
	fmt.Println("=== 完成 ===")
}
