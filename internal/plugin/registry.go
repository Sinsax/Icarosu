package plugin

import (
	"fmt"
	"sync"
)

// Registry 是全局插件注册表。
// 插件包可以在 init() 中调用 RegisterGlobal 自注册，
// 主程序只需匿名导入对应包即可自动收集插件。
type Registry struct {
	mu      sync.RWMutex
	plugins []Plugin
	byName  map[string]Plugin
}

var global = &Registry{
	byName: make(map[string]Plugin),
}

// RegisterGlobal 将插件注册到全局注册表。
// 插件名必须唯一，重复注册会 panic。
func RegisterGlobal(p Plugin) {
	global.Register(p)
}

// GlobalPlugins 返回全局注册表中的所有插件。
func GlobalPlugins() []Plugin {
	return global.Plugins()
}

// Register 注册一个插件到该注册表。
func (r *Registry) Register(p Plugin) {
	if p == nil {
		panic("plugin: Register called with nil plugin")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byName == nil {
		r.byName = make(map[string]Plugin)
	}

	if _, ok := r.byName[p.Name()]; ok {
		panic(fmt.Sprintf("plugin: duplicate plugin name %q", p.Name()))
	}

	r.byName[p.Name()] = p
	r.plugins = append(r.plugins, p)
}

// Plugins 返回注册表中的所有插件副本。
func (r *Registry) Plugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]Plugin, len(r.plugins))
	copy(plugins, r.plugins)
	return plugins
}
