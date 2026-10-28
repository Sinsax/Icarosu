package onebot

import (
	"context"
	_ "embed"
	"log/slog"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed plugin.yaml
var pluginManifestYAML []byte

var parsedPluginManifest = manifest.MustParse(pluginManifestYAML)

// Plugin 把 Adapter 包装成 sdk.Plugin + sdk.AdapterProvider。
type Plugin struct {
	adapter *Adapter
}

func NewPlugin(client Client, logger *slog.Logger) sdk.Plugin {
	return NewPluginWithAdmins(client, nil, logger)
}

func NewPluginWithAdmins(client Client, adminIDs []int64, logger *slog.Logger) sdk.Plugin {
	return &Plugin{adapter: NewWithAdmins(client, adminIDs, logger)}
}

func (p *Plugin) Manifest() sdk.Manifest                        { return parsedPluginManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error { return nil }
func (p *Plugin) Start(ctx context.Context) error               { return nil }
func (p *Plugin) Stop(ctx context.Context) error                { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }
func (p *Plugin) Adapter() sdk.Adapter                          { return p.adapter }

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.AdapterProvider = (*Plugin)(nil)
