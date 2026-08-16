package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Manager 负责插件的注册、生命周期管理和事件分发。
type Manager struct {
	mu      sync.RWMutex
	plugins []Plugin
}

func NewManager() *Manager {
	return &Manager{}
}

// Register 注册一个插件。插件按 Priority 从高到低执行。
func (m *Manager) Register(p Plugin) {
	if p == nil {
		panic("plugin: Register called with nil plugin")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, p)
}

// Start 按优先级顺序初始化所有插件。
func (m *Manager) Start(ctx context.Context, bot Bot) error {
	m.mu.RLock()
	plugins := make([]Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	sortPlugins(plugins)

	for _, p := range plugins {
		if err := p.Init(ctx, bot); err != nil {
			return fmt.Errorf("init plugin %q: %w", p.Name(), err)
		}
	}
	return nil
}

// Stop 逆序停止所有插件。
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.RLock()
	plugins := make([]Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	sortPlugins(plugins)
	for i := len(plugins) - 1; i >= 0; i-- {
		p := plugins[i]
		if err := p.Stop(ctx); err != nil {
			return fmt.Errorf("stop plugin %q: %w", p.Name(), err)
		}
	}
	return nil
}

// Dispatch 将事件按优先级依次交给插件处理。
// 某个插件返回 true 时停止分发。
func (m *Manager) Dispatch(ctx context.Context, bot Bot, e *Event) {
	m.mu.RLock()
	plugins := make([]Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	sortPlugins(plugins)

	for _, p := range plugins {
		if p.Handle(ctx, bot, e) {
			return
		}
	}
}

// Plugins 返回当前已注册的插件列表（按优先级排序）。
func (m *Manager) Plugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	sortPlugins(plugins)
	return plugins
}

func sortPlugins(plugins []Plugin) {
	sort.SliceStable(plugins, func(i, j int) bool {
		return plugins[i].Priority() > plugins[j].Priority()
	})
}
