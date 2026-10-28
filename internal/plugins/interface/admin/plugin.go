package admin

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

//go:embed MANUAL.md
var manual string

var parsedManifest = manifest.MustParse(manifestYAML)

type Plugin struct {
	view   sdk.AdminView
	logger *slog.Logger
}

func New(view sdk.AdminView, logger *slog.Logger) sdk.Plugin {
	if logger == nil {
		logger = slog.Default()
	}
	return &Plugin{view: view, logger: logger}
}

func (p *Plugin) Manifest() sdk.Manifest                        { return parsedManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error { return nil }
func (p *Plugin) Start(ctx context.Context) error               { return nil }
func (p *Plugin) Stop(ctx context.Context) error                { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }

func (p *Plugin) Tools() []sdk.Tool {
	schema := json.RawMessage("{\"type\":\"object\",\"properties\":{}}")
	return []sdk.Tool{
		{
			Name:        "admin_status",
			Summary:     "查看 Icarosu 运行时健康、插件与 Trace 概况",
			Schema:      schema,
			Permissions: []string{"admin"},
			Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
				if p.view == nil {
					return sdk.ToolResult{}, fmt.Errorf("admin view is not configured")
				}
				return jsonResult(map[string]any{
					"health":  p.view.Health(ctx),
					"plugins": p.view.PluginStatuses(),
					"traces":  len(p.view.RecentTraces(200)),
				})
			},
		},
		{
			Name:        "admin_plugins",
			Summary:     "列出插件状态",
			Schema:      schema,
			Permissions: []string{"admin"},
			Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
				if p.view == nil {
					return sdk.ToolResult{}, fmt.Errorf("admin view is not configured")
				}
				return jsonResult(p.view.PluginStatuses())
			},
		},
		{
			Name:        "read_manual",
			Summary:     "读取插件或工具手册，target 形如 plugin:persona.default / tool:time_now",
			Schema:      json.RawMessage("{\"type\":\"object\",\"properties\":{\"target\":{\"type\":\"string\"}},\"required\":[\"target\"]}"),
			Permissions: []string{"admin"},
			Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
				if p.view == nil {
					return sdk.ToolResult{}, fmt.Errorf("admin view is not configured")
				}
				var args struct {
					Target string `json:"target"`
				}
				if err := json.Unmarshal(call.Arguments, &args); err != nil || args.Target == "" {
					return sdk.ToolResult{}, fmt.Errorf("target is required")
				}
				switch {
				case strings.HasPrefix(args.Target, "plugin:"):
					manual, ok := p.view.PluginManual(strings.TrimPrefix(args.Target, "plugin:"))
					if !ok {
						return sdk.ToolResult{}, fmt.Errorf("plugin manual not found")
					}
					return sdk.Text(manual), nil
				case strings.HasPrefix(args.Target, "tool:"):
					manual, ok := p.view.ToolManual(strings.TrimPrefix(args.Target, "tool:"))
					if !ok {
						return sdk.ToolResult{}, fmt.Errorf("tool manual not found")
					}
					return sdk.Text(manual), nil
				default:
					return sdk.ToolResult{}, fmt.Errorf("target must start with plugin: or tool:")
				}
			},
		},
		{
			Name:        "admin_traces",
			Summary:     "查看最近回合 Trace",
			Schema:      json.RawMessage("{\"type\":\"object\",\"properties\":{\"limit\":{\"type\":\"integer\"}}}"),
			Permissions: []string{"admin"},
			Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
				if p.view == nil {
					return sdk.ToolResult{}, fmt.Errorf("admin view is not configured")
				}
				limit := 5
				if len(call.Arguments) > 0 {
					var args struct {
						Limit int `json:"limit"`
					}
					if err := json.Unmarshal(call.Arguments, &args); err == nil && args.Limit > 0 {
						limit = args.Limit
					}
				}
				return jsonResult(p.view.RecentTraces(limit))
			},
		},
	}
}

func (p *Plugin) Manual() string { return manual }

func jsonResult(v any) (sdk.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return sdk.ToolResult{}, err
	}
	return sdk.Text(string(data)), nil
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.ToolProvider = (*Plugin)(nil)
