package session

import (
	"context"
	"sync"
	"time"
)

// Session 表示一个会话（群/私聊）。
type Session struct {
	ID         int64
	LastActive time.Time
	mu         sync.Mutex
}

// Lock 锁定该会话，保证同一会话内的消息按顺序处理。
func (s *Session) Lock() {
	s.mu.Lock()
}

// Unlock 解锁该会话。
func (s *Session) Unlock() {
	s.mu.Unlock()
}

// Manager 管理所有会话，支持 TTL 清理。
type Manager struct {
	mu       sync.Mutex
	sessions map[int64]*Session
	ttl      time.Duration
}

func New(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Manager{
		sessions: make(map[int64]*Session),
		ttl:      ttl,
	}
}

// Get 获取会话，不存在则创建。
func (m *Manager) Get(id int64) *Session {
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

// Cleanup 清理超过 TTL 未活跃的会话。
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, s := range m.sessions {
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
