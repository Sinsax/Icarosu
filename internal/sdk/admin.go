package sdk

import (
	"context"
	"encoding/json"
)

// ManualProvider 是可选接口：插件提供 Markdown 手册。
type ManualProvider interface {
	Manual() string
}

// PluginController 是管理界面的写操作能力。
type PluginController interface {
	SetPluginEnabled(ctx context.Context, id string, enabled bool) error
	ReloadPlugin(ctx context.Context, id string) error
}

// PluginStatus 是内核暴露给管理界面的插件状态。
type PluginStatus struct {
	ID          string
	Version     string
	Name        string
	Description string
	Kind        Kind
	Critical    bool
	HasManual   bool
	Enabled     bool
	Health      Health
}

// AdminView 是 interface 插件可读取的内核只读视图。
type AdminView interface {
	PluginStatuses() []PluginStatus
	PluginManual(id string) (string, bool)
	ToolManual(name string) (string, bool)
	ToolSchema(name string) (json.RawMessage, bool)
	Health(ctx context.Context) Health
	RecentTraces(limit int) []Trace
	Metrics() map[string]int64
}
