package bot

import (
	"app/core"
	"sync/atomic"
	"time"
)

// Monitor 健康监控 + 心跳看门狗
// 防止脚本在"逻辑死锁"状态下悄悄卡死
type Monitor struct {
	lastHeartbeat atomic.Int64 // Unix 时间戳

	// 配置
	beatInterval time.Duration // 心跳间隔（Bot 主动 Beat）
	watchTimeout time.Duration // 超时阈值（超时触发恢复）

	// 回调：超时后动作
	onTimeout func()

	stopCh chan struct{}
	doneCh chan struct{}

	// 统计
	totalTimeouts atomic.Int32
	lastTimeoutAt atomic.Int64
	beatCount     atomic.Int64
}

// NewMonitor 创建监控器
//   - beatInterval: 心跳间隔，默认 5s
//   - timeout: 超时阈值，默认 90s
//   - onTimeout: 超时回调，一般为触发全量重启
func NewMonitor(beatInterval, timeout time.Duration, onTimeout func()) *Monitor {
	if beatInterval <= 0 {
		beatInterval = 5 * time.Second
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &Monitor{
		beatInterval: beatInterval,
		watchTimeout: timeout,
		onTimeout:    onTimeout,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Beat 记录一次心跳（Bot 每次主循环调用）
func (m *Monitor) Beat() {
	m.lastHeartbeat.Store(time.Now().Unix())
	m.beatCount.Add(1)
}

// Start 启动看门狗 goroutine
func (m *Monitor) Start() {
	// 先初始化心跳
	m.lastHeartbeat.Store(time.Now().Unix())

	go m.watchLoop()
	core.LogWithTimef("[Monitor] 看门狗已启动 (间隔=%v, 超时=%v)", m.beatInterval, m.watchTimeout)
}

// Stop 停止看门狗
func (m *Monitor) Stop() {
	close(m.stopCh)
	<-m.doneCh
	core.LogWithTimef("[Monitor] 看门狗已停止, 共心跳 %d 次, 超时 %d 次",
		m.BeatCount(), m.TimeoutCount())
}

// watchLoop 检查循环
func (m *Monitor) watchLoop() {
	defer close(m.doneCh)
	ticker := time.NewTicker(m.beatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			elapsed := time.Since(time.Unix(m.lastHeartbeat.Load(), 0))
			if elapsed > m.watchTimeout {
				m.totalTimeouts.Add(1)
				m.lastTimeoutAt.Store(time.Now().Unix())
				core.LogWithTimef("[Monitor] ⚠ 心跳超时! 上次心跳距今 %v, 阈值 %v",
					elapsed.Round(time.Second), m.watchTimeout)

				if m.onTimeout != nil {
					m.onTimeout()
				}
			}
		}
	}
}

// BeatCount 心跳次数
func (m *Monitor) BeatCount() int64 {
	return m.beatCount.Load()
}

// TimeoutCount 超时次数
func (m *Monitor) TimeoutCount() int32 {
	return m.totalTimeouts.Load()
}

// LastBeatAgo 最后一次心跳距今
func (m *Monitor) LastBeatAgo() time.Duration {
	return time.Since(time.Unix(m.lastHeartbeat.Load(), 0))
}

// Healthy 当前是否健康（心跳未超时）
func (m *Monitor) Healthy() bool {
	return m.LastBeatAgo() < m.watchTimeout
}
