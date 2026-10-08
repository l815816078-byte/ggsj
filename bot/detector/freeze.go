package detector

import (
	"app/core"
	"time"
)

// FreezeDetector 画面冻结检测器
// 配置位于 assets/config/detectors.json → freeze 节点
//
// check_images: 用于判断画面是否变化的模板图列表，放在 assets/ 目录下
// threshold:    匹配阈值，默认 0.85
// timeout_sec:  冻结超时，持续冻结超过此时间才触发异常
type FreezeDetector struct {
	interval    time.Duration
	timeout     time.Duration
	enabled     bool
	threshold   float64
	checkImages []string
	freezeStart time.Time
}

// NewFreezeDetector 从 JSON 配置创建
func NewFreezeDetector(cfg *FreezeConfig) *FreezeDetector {
	if cfg == nil {
		cfg = &FreezeConfig{IntervalSec: 15, TimeoutSec: 300, Sim: 0.85}
	}
	interval := time.Duration(cfg.IntervalSec) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	threshold := cfg.Sim
	if threshold <= 0 {
		threshold = 0.85
	}
	return &FreezeDetector{
		interval:    interval,
		timeout:     timeout,
		enabled:     cfg.Enabled,
		threshold:   threshold,
		checkImages: cfg.CheckImages,
	}
}

func (d *FreezeDetector) Name() string            { return "画面冻结检测" }
func (d *FreezeDetector) Interval() time.Duration { return d.interval }
func (d *FreezeDetector) SetEnabled(v bool)       { d.enabled = v }

func (d *FreezeDetector) Check() bool {
	if !d.enabled || len(d.checkImages) == 0 {
		return false
	}

	sameMatches := 0
	totalChecks := 0

	for _, img := range d.checkImages {
		x, y, s := d.findImage(img)
		if s >= d.threshold {
			totalChecks++
			prevX, prevY, prevSim := getPrevMatch(img)
			if prevSim >= d.threshold && abs(x-prevX) <= 2 && abs(y-prevY) <= 2 {
				sameMatches++
			}
			setPrevMatch(img, x, y, s)
		}
	}

	if totalChecks > 0 && sameMatches == totalChecks {
		if d.freezeStart.IsZero() {
			d.freezeStart = time.Now()
			core.LogWithTimef("[Detector] 画面冻结检测: 开始跟踪")
		} else if time.Since(d.freezeStart) > d.timeout {
			core.LogWithTimef("[Detector] 画面冻结异常: 持续%v", time.Since(d.freezeStart).Round(time.Second))
			d.freezeStart = time.Time{}
			return true
		}
	} else {
		d.freezeStart = time.Time{}
	}

	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// findImage 走识别预设，跟任务引擎一样
func (d *FreezeDetector) findImage(img string) (int, int, float64) {
	r := core.Exec(img)
	return r.X, r.Y, r.Sim
}

type prevMatch struct {
	x, y int
	sim  float64
}

var prevMatches = make(map[string]prevMatch)

func getPrevMatch(key string) (int, int, float64) {
	m := prevMatches[key]
	return m.x, m.y, m.sim
}
func setPrevMatch(key string, x, y int, sim float64) {
	prevMatches[key] = prevMatch{x, y, sim}
}
