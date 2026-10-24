package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Manager 负责插件的注册、生命周期管理和事件分发。
type Manager struct {
	mu       sync.RWMutex
	plugins  []Plugin
	disabled map[string]bool
}

// PluginState 是插件运行状态，用于管理界面展示和热开关。
type PluginState struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stage       Stage  `json:"stage"`
	Priority    int    `json:"priority"`
	Enabled     bool   `json:"enabled"`
}

func NewManager() *Manager {
	return &Manager{disabled: make(map[string]bool)}
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

// SetEnabled 动态启用或禁用插件。
func (m *Manager) SetEnabled(name string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if enabled {
		delete(m.disabled, name)
	} else {
		m.disabled[name] = true
	}
}

// States 返回所有插件状态（按优先级排序）。
func (m *Manager) States() []PluginState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plugins := make([]Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	sortPlugins(plugins)

	states := make([]PluginState, 0, len(plugins))
	for _, p := range plugins {
		states = append(states, PluginState{
			Name:        p.Name(),
			Description: p.Description(),
			Stage:       p.Stage(),
			Priority:    p.Priority(),
			Enabled:     !m.disabled[p.Name()],
		})
	}
	return states
}

func (m *Manager) isEnabled(name string) bool {
	return !m.disabled[name]
}

// Start 按优先级顺序初始化所有启用的插件。
func (m *Manager) Start(ctx context.Context, bot Bot) error {
	m.mu.RLock()
	plugins := m.enabledPluginsLocked()
	m.mu.RUnlock()

	sortPlugins(plugins)

	for _, p := range plugins {
		if err := p.Init(ctx, bot); err != nil {
			return fmt.Errorf("init plugin %q: %w", p.Name(), err)
		}
	}
	return nil
}

// Stop 逆序停止所有启用的插件。
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.RLock()
	plugins := m.enabledPluginsLocked()
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

// Dispatch 按流水线阶段分发事件：
//  1. 先执行所有 Preprocessor，某个插件返回 true 时整体停止；
//  2. 再执行所有 Responder，某个插件返回 true 时停止后续 Responder，
//     但仍继续执行 Postprocessor；
//  3. 最后执行所有 Postprocessor。
//
// 每个阶段内部仍按 Priority 从高到低执行。
// 插件可实现 ContextualPlugin 来读写流水线中间产物 Context.Data。
func (m *Manager) Dispatch(ctx context.Context, bot Bot, e *Event) {
	m.mu.RLock()
	plugins := m.enabledPluginsLocked()
	m.mu.RUnlock()

	sortPlugins(plugins)
	pc := NewContext(e)

	for _, stage := range []Stage{StagePreprocessor, StageResponder, StagePostprocessor} {
		if stage == StageResponder {
			for _, p := range plugins {
				if p.Stage() != stage {
					continue
				}
				if m.handlePlugin(ctx, bot, p, e, pc) {
					break
				}
			}
			continue
		}

		for _, p := range plugins {
			if p.Stage() != stage {
				continue
			}
			if m.handlePlugin(ctx, bot, p, e, pc) {
				return
			}
		}
	}
}

func (m *Manager) handlePlugin(ctx context.Context, bot Bot, p Plugin, e *Event, pc *Context) bool {
	if cp, ok := p.(ContextualPlugin); ok {
		return cp.HandleContext(ctx, bot, pc)
	}
	return p.Handle(ctx, bot, e)
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

func (m *Manager) enabledPluginsLocked() []Plugin {
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		if !m.disabled[p.Name()] {
			plugins = append(plugins, p)
		}
	}
	return plugins
}

func sortPlugins(plugins []Plugin) {
	sort.SliceStable(plugins, func(i, j int) bool {
		return plugins[i].Priority() > plugins[j].Priority()
	})
}
