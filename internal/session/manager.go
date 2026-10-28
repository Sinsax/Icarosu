package session

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Session 表示一个会话（群/私聊）。
// ID 使用带命名空间的 key，例如 "group:123" / "user:123"，避免群号和用户号撞锁。
type Session struct {
	ID         string
	LastActive time.Time
	mu         sync.Mutex
	inUse      atomic.Int32
	refs       atomic.Int32
}

// Lock 锁定该会话，保证同一会话内的消息按顺序处理。
func (s *Session) Lock() {
	s.inUse.Add(1)
	s.mu.Lock()
}

// Unlock 解锁该会话。
func (s *Session) Unlock() {
	s.mu.Unlock()
	s.inUse.Add(-1)
}

// InUse 返回当前是否有人在持有或等待该会话锁。
func (s *Session) InUse() bool {
	return s.inUse.Load() > 0
}

// Release 归还 Acquire 的引用。Handler 必须在处理完一条消息后调用。
func (s *Session) Release() {
	if s.refs.Add(-1) < 0 {
		panic("session: Release called more than Acquire")
	}
}

// Manager 管理所有会话，支持 TTL 清理。
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

func New(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Manager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

// Acquire 获取会话并占一个引用；调用方必须在使用完后 Release。
// 引用用于防止 Cleanup 在“拿到 Session”和“Lock”之间删除会话，
// 从而避免同一 key 出现两把不同的锁。
func (m *Manager) Acquire(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		s = &Session{ID: id, LastActive: time.Now()}
		m.sessions[id] = s
	}
	s.refs.Add(1)
	s.LastActive = time.Now()
	return s
}

// Get 获取会话，不存在则创建。
func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		s = &Session{ID: id, LastActive: time.Now()}
		m.sessions[id] = s
	}
	s.LastActive = time.Now()
	return s
}

// Cleanup 清理超过 TTL 未活跃且当前未被使用的会话。
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, s := range m.sessions {
		if s.InUse() || s.refs.Load() > 0 {
			continue
		}
		if now.Sub(s.LastActive) > m.ttl {
			delete(m.sessions, id)
		}
	}
}

// CleanupLoop 定期清理过期会话，直到 ctx 取消。
func (m *Manager) CleanupLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Cleanup()
		}
	}
}
