package detector

import (
	"app/core"
	"sync/atomic"
	"time"
)

// IdleDetector 脚本空闲/卡死检测器
// 判定依据：任务执行心跳(Beat) 与 识别配置命中(core.LastConfigHit) 两个判据，
// 两者同时超过阈值无动静才认为脚本卡死/空闲
type IdleDetector struct {
	interval          time.Duration
	idleThreshold     time.Duration
	lastExecutionTime atomic.Value // time.Time
	lastTaskName      atomic.Value // string
	enabled           bool
}

// NewIdleDetector 创建空闲检测器
func NewIdleDetector(interval, threshold time.Duration) *IdleDetector {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if threshold <= 0 {
		threshold = 15 * time.Minute
	}
	d := &IdleDetector{
		interval:      interval,
		idleThreshold: threshold,
		enabled:       true,
	}
	d.lastExecutionTime.Store(time.Now())
	d.lastTaskName.Store("")
	return d
}

func (d *IdleDetector) Name() string            { return "空闲检测" }
func (d *IdleDetector) Interval() time.Duration { return d.interval }
func (d *IdleDetector) SetEnabled(v bool)       { d.enabled = v }

// Beat 记录一次任务执行（调度器每次执行完任务调用）
func (d *IdleDetector) Beat(taskName string) {
	d.lastExecutionTime.Store(time.Now())
	d.lastTaskName.Store(taskName)
}

func (d *IdleDetector) Check() bool {
	if !d.enabled {
		return false
	}

	beatElapsed := time.Since(d.lastExecutionTime.Load().(time.Time))
	hitElapsed := time.Since(core.LastConfigHit())

	core.LogWithTimef("[Detector] 空闲检测: 距上次任务=%v, 距上次识别命中=%v, 阈值=%v",
		beatElapsed.Round(time.Second), hitElapsed.Round(time.Second), d.idleThreshold)

	// 两个判据需同时超时才判空闲（and 关系）：
	// 1) 任务执行心跳超时（任务卡死/不推进）
	// 2) 识别命中心跳超时（recogs.json 全部预设持续不命中，画面卡死）
	if beatElapsed > d.idleThreshold && hitElapsed > d.idleThreshold {
		core.LogWithTimef("[Detector] 空闲检测异常: 超过%v既无任务执行又无任何识别命中", d.idleThreshold)
		return true
	}
	return false
}
