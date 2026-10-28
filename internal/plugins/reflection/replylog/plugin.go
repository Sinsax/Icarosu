package replylog

import (
	"context"
	_ "embed"
	"log/slog"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Plugin struct {
	logger *slog.Logger
}

func New() sdk.Plugin { return &Plugin{logger: slog.Default()} }

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil && host.Logger() != nil {
		p.logger = host.Logger()
	}
	return nil
}
func (p *Plugin) Start(ctx context.Context) error       { return nil }
func (p *Plugin) Stop(ctx context.Context) error        { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Observe(ctx context.Context, ev sdk.Event) {
	p.logger.Debug("observed event", "kind", ev.Kind, "conversation", ev.Conversation.ID, "actor", ev.Actor.ID)
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Observer = (*Plugin)(nil)
