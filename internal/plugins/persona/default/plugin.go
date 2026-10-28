package persona

import (
	"context"
	_ "embed"
	"strings"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

//go:embed MANUAL.md
var manual string

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	Name          string   `json:"name"`
	Nicknames     []string `json:"nicknames"`
	Personality   string   `json:"personality"`
	SpeakingStyle string   `json:"speaking_style"`
	Background    string   `json:"background"`
}

type Plugin struct {
	cfg     Config
	persona sdk.Persona
}

func New() sdk.Plugin { return &Plugin{} }

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	p.cfg = Config{
		Name:          "小智",
		Personality:   "性格开朗，有点懒，喜欢吐槽，偶尔嘴贫，但不会冒犯别人。",
		SpeakingStyle: "短句、口语化，像QQ群友一样自然；少用列表和总结。",
		Background:    "你是一个常在群里水群的普通网友，不是客服，也不是助手。",
	}
	if host != nil {
		_ = host.Config("persona.default", &p.cfg)
	}
	p.persona = sdk.Persona{
		ID:           "persona.default",
		Name:         p.cfg.Name,
		SystemPrompt: buildPrompt(p.cfg),
	}
	return nil
}

func (p *Plugin) Start(ctx context.Context) error       { return nil }
func (p *Plugin) Stop(ctx context.Context) error        { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Persona(ctx context.Context) (sdk.Persona, error) {
	return p.persona, nil
}

func (p *Plugin) Manual() string { return manual }

func buildPrompt(cfg Config) string {
	parts := []string{
		"你是QQ群里的普通群友，不是助手，也不是客服。",
		"你的名字是：" + cfg.Name + "。",
	}
	if len(cfg.Nicknames) > 0 {
		parts = append(parts, "别人也可能叫你："+strings.Join(cfg.Nicknames, "、")+"。")
	}
	if cfg.Background != "" {
		parts = append(parts, "背景设定："+cfg.Background)
	}
	if cfg.Personality != "" {
		parts = append(parts, "性格："+cfg.Personality)
	}
	if cfg.SpeakingStyle != "" {
		parts = append(parts, "说话风格："+cfg.SpeakingStyle)
	}
	parts = append(parts,
		"回复通常很短，1~7 个字，像真人随手打字。",
		"不要自称 AI，不要列点，不要总结，不要客服腔。",
	)
	return strings.Join(parts, "\n")
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.PersonaProvider = (*Plugin)(nil)
