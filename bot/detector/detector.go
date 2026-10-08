package detector

import "time"

// Detector 异常检测器接口
type Detector interface {
	// Name 检测器名称
	Name() string
	// Check 执行检测，返回是否触发异常
	Check() bool
	// Interval 检测间隔
	Interval() time.Duration
}

// Detected 检测结果
type Detected struct {
	DetectorName string
	At           time.Time
	Msg          string
}

// Manager 检测器管理器
type Manager struct {
	detectors []Detector
	onDetect  func(Detected)
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewManager 创建管理器
func NewManager(onDetect func(Detected)) *Manager {
	return &Manager{
		onDetect: onDetect,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Register 注册检测器
func (m *Manager) Register(d Detector) {
	m.detectors = append(m.detectors, d)
}

// Start 启动所有检测器（各自 goroutine）
func (m *Manager) Start() {
	for _, d := range m.detectors {
		go m.runDetector(d)
	}
}

// Stop 停止所有检测器
func (m *Manager) Stop() {
	close(m.stopCh)
}

// AllDetectors 返回所有已注册的检测器
func (m *Manager) AllDetectors() []Detector {
	return m.detectors
}

// runDetector 以指定间隔循环运行单个检测器
func (m *Manager) runDetector(d Detector) {
	ticker := time.NewTicker(d.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if d.Check() && m.onDetect != nil {
				m.onDetect(Detected{
					DetectorName: d.Name(),
					At:           time.Now(),
					Msg:          d.Name() + " 触发异常",
				})
			}
		}
	}
}
