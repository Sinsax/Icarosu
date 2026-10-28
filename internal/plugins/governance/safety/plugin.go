package safety

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	Forbidden []string `json:"forbidden"`
}

type Plugin struct {
	cfg Config
}

func New() sdk.Plugin { return &Plugin{} }

// NewWithConfig 便于测试/显式配置。
func NewWithConfig(cfg Config) sdk.Plugin { return &Plugin{cfg: cfg} }

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil {
		_ = host.Config("governance.safety", &p.cfg)
	}
	return nil
}
func (p *Plugin) Start(ctx context.Context) error       { return nil }
func (p *Plugin) Stop(ctx context.Context) error        { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Check(ctx context.Context, turn *sdk.Turn) error {
	return p.checkText(eventText(turn.Event))
}

// CheckOutput 阻止模型把违禁词发出去。
func (p *Plugin) CheckOutput(ctx context.Context, turn *sdk.Turn, reply string) error {
	return p.checkText(reply)
}

func (p *Plugin) checkText(text string) error {
	text = strings.ToLower(text)
	for _, word := range p.cfg.Forbidden {
		if word != "" && strings.Contains(text, strings.ToLower(word)) {
			return fmt.Errorf("blocked by safety filter: %s", word)
		}
	}
	return nil
}

func eventText(ev sdk.Event) string {
	var b strings.Builder
	for _, part := range ev.Parts {
		if part.Type == sdk.PartText {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Policy = (*Plugin)(nil)
var _ sdk.OutputPolicy = (*Plugin)(nil)
