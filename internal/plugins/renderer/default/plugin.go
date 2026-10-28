package renderer

import (
	"context"
	_ "embed"
	"strings"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Plugin struct{}

func New() sdk.Plugin { return &Plugin{} }

func (p *Plugin) Manifest() sdk.Manifest                        { return parsedManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error { return nil }
func (p *Plugin) Start(ctx context.Context) error               { return nil }
func (p *Plugin) Stop(ctx context.Context) error                { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }

func (p *Plugin) Render(ctx context.Context, turn *sdk.Turn, reply string) ([]sdk.Action, error) {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil, nil
	}
	return []sdk.Action{{
		Conversation: turn.Event.Conversation,
		Actor:        turn.Event.Self,
		Parts:        []sdk.Part{{Type: sdk.PartText, Text: reply}},
		ReplyTo:      turn.Event.Meta["native_message_id"],
	}}, nil
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Renderer = (*Plugin)(nil)
