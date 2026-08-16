package config

import (
	"fmt"
	"sync"
)

// Manager 持有当前配置，并支持从磁盘热重载。
// 组件可以通过 OnReload 注册回调，在配置更新后自动应用新值。
type Manager struct {
	mu        sync.RWMutex
	path      string
	cfg       *Config
	listeners []func(*Config)
}

// NewManager 加载初始配置并创建管理器。
func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Manager{path: path, cfg: cfg}, nil
}

// Get 返回当前配置。
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Path 返回配置文件路径。
func (m *Manager) Path() string {
	return m.path
}

// Reload 重新从磁盘加载配置，并通知所有监听器。
func (m *Manager) Reload() error {
	cfg, err := Load(m.path)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	m.mu.Lock()
	m.cfg = cfg
	listeners := make([]func(*Config), len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.Unlock()

	for _, fn := range listeners {
		fn(cfg)
	}
	return nil
}

// OnReload 注册一个配置更新回调。回调在 Reload 成功后被调用。
func (m *Manager) OnReload(fn func(*Config)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	m.listeners = append(m.listeners, fn)
	m.mu.Unlock()
}
