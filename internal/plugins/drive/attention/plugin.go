package attention

import (
	"context"
	_ "embed"
	"math/rand"
	"strings"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	Keywords          []string `json:"keywords"`
	RandomProbability float64  `json:"random_probability"`
}

type Plugin struct {
	cfg Config
}

func New() sdk.Plugin {
	return &Plugin{cfg: Config{RandomProbability: 0.02}}
}

// NewWithConfig 便于测试/显式配置。
func NewWithConfig(cfg Config) sdk.Plugin { return &Plugin{cfg: cfg} }

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil {
		_ = host.Config("drive.attention", &p.cfg)
	}
	return nil
}

func (p *Plugin) Start(ctx context.Context) error       { return nil }
func (p *Plugin) Stop(ctx context.Context) error        { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Decide(ctx context.Context, turn *sdk.Turn) (sdk.Decision, error) {
	ev := turn.Event
	if ev.Kind != "message.created" {
		return sdk.Decision{Kind: sdk.DecisionObserve, Reason: "non_message"}, nil
	}
	if ev.Conversation.Kind == "private" {
		return sdk.Decision{Kind: sdk.DecisionReply, Reason: "private"}, nil
	}
	for _, m := range ev.Mentions {
		if m == ev.Self {
			return sdk.Decision{Kind: sdk.DecisionReply, Reason: "at_bot"}, nil
		}
	}
	text := eventText(ev)
	for _, k := range p.cfg.Keywords {
		if k != "" && strings.Contains(text, k) {
			return sdk.Decision{Kind: sdk.DecisionReply, Reason: "keyword"}, nil
		}
	}
	if p.cfg.RandomProbability > 0 && rand.Float64() < p.cfg.RandomProbability {
		return sdk.Decision{Kind: sdk.DecisionReply, Reason: "random"}, nil
	}
	return sdk.Decision{Kind: sdk.DecisionObserve, Reason: "not_addressed"}, nil
}

func eventText(ev sdk.Event) string {
	var b strings.Builder
	for _, part := range ev.Parts {
		if part.Type == sdk.PartText {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Decider = (*Plugin)(nil)
