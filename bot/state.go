package bot

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// State 机器人运行状态
type State int32

const (
	// StateIdle 空闲/刚启动，等待调度
	StateIdle State = iota
	// StateRunning 正常运行中
	StateRunning
	// StatePaused 手动暂停
	StatePaused
	// StateRecovering 自动恢复中（全量重启）
	StateRecovering
	// StateStopped 已停止
	StateStopped
)

// String 状态名称
func (s State) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateRunning:
		return "RUNNING"
	case StatePaused:
		return "PAUSED"
	case StateRecovering:
		return "RECOVERING"
	case StateStopped:
		return "STOPPED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// StateMachine goroutine 安全的状态机
type StateMachine struct {
	current atomic.Int32 // 存 State 值

	mu          sync.RWMutex
	subscribers []chan StateChange
	pausedBy    string // 谁暂停的
}

// StateChange 状态变更事件
type StateChange struct {
	From State
	To   State
	Msg  string
}

// NewStateMachine 创建状态机
func NewStateMachine() *StateMachine {
	sm := &StateMachine{}
	sm.current.Store(int32(StateIdle))
	return sm
}

// Get 获取当前状态（原子读）
func (sm *StateMachine) Get() State {
	return State(sm.current.Load())
}

// Is 判断是否为指定状态
func (sm *StateMachine) Is(s State) bool {
	return sm.Get() == s
}

// transition 执行状态迁移（内部方法，不加锁，调用方负责通知）
func (sm *StateMachine) transition(from, to State) bool {
	return sm.current.CompareAndSwap(int32(from), int32(to))
}

// GoRunning Idle/Paused → Running
func (sm *StateMachine) GoRunning() bool {
	cur := sm.Get()
	if cur != StateIdle && cur != StatePaused {
		return false
	}
	if sm.transition(cur, StateRunning) {
		sm.notify(StateChange{From: cur, To: StateRunning, Msg: "启动运行"})
		return true
	}
	return false
}

// Pause Running → Paused（手动暂停）
func (sm *StateMachine) Pause(reason string) {
	cur := sm.Get()
	if cur != StateRunning {
		return
	}
	if sm.transition(cur, StatePaused) {
		sm.mu.Lock()
		sm.pausedBy = reason
		sm.mu.Unlock()
		sm.notify(StateChange{From: cur, To: StatePaused, Msg: reason})
	}
}

// Resume Paused → Running
func (sm *StateMachine) Resume() bool {
	cur := sm.Get()
	if cur != StatePaused {
		return false
	}
	if sm.transition(cur, StateRunning) {
		sm.mu.Lock()
		sm.pausedBy = ""
		sm.mu.Unlock()
		sm.notify(StateChange{From: cur, To: StateRunning, Msg: "恢复运行"})
		return true
	}
	return false
}

// StartRecover Running → Recovering
func (sm *StateMachine) StartRecover(reason string) {
	cur := sm.Get()
	if cur != StateRunning {
		return
	}
	if sm.transition(cur, StateRecovering) {
		sm.notify(StateChange{From: cur, To: StateRecovering, Msg: reason})
	}
}

// RecoverOK Recovering → Running
func (sm *StateMachine) RecoverOK() bool {
	cur := sm.Get()
	if cur != StateRecovering {
		return false
	}
	if sm.transition(cur, StateRunning) {
		sm.notify(StateChange{From: cur, To: StateRunning, Msg: "恢复成功"})
		return true
	}
	return false
}

// Stop 任意状态 → Stopped（正常关机）
func (sm *StateMachine) Stop() {
	cur := sm.Get()
	if cur == StateStopped {
		return
	}
	if sm.transition(cur, StateStopped) {
		sm.notify(StateChange{From: cur, To: StateStopped, Msg: "停止"})
	}
}

// Subscribe 订阅状态变更事件
func (sm *StateMachine) Subscribe() <-chan StateChange {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	ch := make(chan StateChange, 32)
	sm.subscribers = append(sm.subscribers, ch)
	return ch
}

// notify 通知所有订阅者
func (sm *StateMachine) notify(change StateChange) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, ch := range sm.subscribers {
		select {
		case ch <- change:
		default:
			// 订阅者处理不过来，丢弃（不阻塞状态机）
		}
	}
}

// CanRun 当前状态是否可以执行任务
func (sm *StateMachine) CanRun() bool {
	s := sm.Get()
	return s == StateRunning
}

// IsAlive 是否还在运行（没停）
func (sm *StateMachine) IsAlive() bool {
	s := sm.Get()
	return s != StateStopped
}
