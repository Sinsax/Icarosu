package system

import (
	"context"
	_ "embed"
	"encoding/json"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Plugin struct{}

func New() sdk.Plugin                                           { return &Plugin{} }
func (p *Plugin) Manifest() sdk.Manifest                        { return parsedManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error { return nil }
func (p *Plugin) Start(ctx context.Context) error               { return nil }
func (p *Plugin) Stop(ctx context.Context) error                { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }

func (p *Plugin) Tools() []sdk.Tool {
	return []sdk.Tool{{
		Name:        "system_ping",
		Summary:     "连通性测试，返回 pong",
		Schema:      json.RawMessage("{\"type\":\"object\",\"properties\":{}}"),
		Permissions: []string{},
		Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
			return sdk.Text("pong"), nil
		},
	}}
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.ToolProvider = (*Plugin)(nil)
