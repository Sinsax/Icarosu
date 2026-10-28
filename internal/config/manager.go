package config

import (
	"fmt"
	"log/slog"
	"sync"
)

// Manager 持有当前配置，并支持从磁盘热重载。
// 组件可以通过 OnReload 注册回调，在配置更新后自动应用新值。
type Manager struct {
	mu        sync.RWMutex
	reloadMu  sync.Mutex
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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
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

// Reload 重新从磁盘加载配置，校验通过后原子替换，并通知所有监听器。
// 监听器 panic 不会拖垮进程；但监听器失败无法回滚已替换的配置，调用方应保证
// 监听器自身是幂等/可重入的。
func (m *Manager) Reload() error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	cfg, err := Load(m.path)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate reloaded config: %w", err)
	}

	m.mu.Lock()
	listeners := make([]func(*Config), len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.Unlock()

	// 先在旧配置仍生效时执行监听器；任一监听器 panic 都不提交新配置，
	// 让 WebUI/调用方可以恢复配置文件。
	for _, fn := range listeners {
		var listenerErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					listenerErr = fmt.Errorf("config reload listener panic: %v", r)
				}
			}()
			fn(cfg)
		}()
		if listenerErr != nil {
			slog.Error("config reload listener failed", "error", listenerErr)
			return listenerErr
		}
	}

	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
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
