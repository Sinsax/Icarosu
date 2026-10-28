package timeplugin

import (
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

//go:embed MANUAL.md
var manual string

//go:embed manuals/time_now.md
var timeNowManual string

//go:embed schemas/time_now.json
var timeNowSchema []byte

var parsedManifest = manifest.MustParse(manifestYAML)

// Plugin 是最小示例插件：只暴露一个纯函数工具。
type Plugin struct {
	host sdk.Host
}

// New 创建时间插件。
func New() sdk.Plugin { return &Plugin{} }

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	p.host = host
	return nil
}

func (p *Plugin) Start(ctx context.Context) error { return nil }
func (p *Plugin) Stop(ctx context.Context) error  { return nil }

func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

// Tools 实现 sdk.ToolProvider。
func (p *Plugin) Tools() []sdk.Tool {
	return []sdk.Tool{{
		Name:    "time_now",
		Summary: "获取当前日期和时间",
		Schema:  json.RawMessage(timeNowSchema),
		Manual:  timeNowManual,
		Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
			now := time.Now()
			if p.host != nil && p.host.Clock() != nil {
				now = p.host.Clock().Now()
			}
			return sdk.Text(now.Format("2006-01-02 15:04:05 Monday")), nil
		},
	}}
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.ToolProvider = (*Plugin)(nil)

// Manual 返回插件手册，供测试/文档使用。
func Manual() string { return manual }

func (p *Plugin) Manual() string { return manual }
