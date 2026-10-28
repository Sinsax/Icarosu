package steal

import (
	"context"
	_ "embed"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"icarosu/internal/manifest"
	"icarosu/internal/plugins/reflection/internal/async"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	Probability float64 `json:"probability"`
}

type Plugin struct {
	mu     sync.Mutex
	host   sdk.Host
	logger *slog.Logger
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc
	wg     async.Group
}

func New() sdk.Plugin {
	return &Plugin{logger: slog.Default(), cfg: Config{Probability: 0.8}}
}

// NewWithConfig 便于测试/显式配置。
func NewWithConfig(cfg Config) sdk.Plugin {
	p := New().(*Plugin)
	p.cfg = cfg
	return p
}

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	p.host = host
	if host != nil {
		if host.Logger() != nil {
			p.logger = host.Logger()
		}
		_ = host.Config("reflection.steal", &p.cfg)
	}
	if p.cfg.Probability < 0 {
		p.cfg.Probability = 0
	}
	if p.cfg.Probability > 1 {
		p.cfg.Probability = 1
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.wg = async.Group{}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	return nil
}

func (p *Plugin) Start(ctx context.Context) error { return nil }
func (p *Plugin) Stop(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	return p.wg.Close(ctx)
}
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Observe(ctx context.Context, ev sdk.Event) {
	if ev.Kind != "message.created" || ev.Conversation.Kind != "group" {
		return
	}
	var imageURL string
	for _, part := range ev.Parts {
		if part.Type == sdk.PartImage && part.URL != "" {
			imageURL = part.URL
			break
		}
	}
	if imageURL == "" {
		return
	}
	visionHost, ok := p.host.(sdk.VisionHost)
	if !ok || visionHost.Vision() == nil {
		return
	}
	memeHost, ok := p.host.(sdk.MemeHost)
	if !ok || memeHost.Memes() == nil {
		return
	}
	p.wg.Go(func() { p.check(imageURL) })
}

func (p *Plugin) check(url string) {
	visionHost := p.host.(sdk.VisionHost)
	memeHost := p.host.(sdk.MemeHost)
	ctx, cancel := context.WithTimeout(p.ctx, 20*time.Second)
	defer cancel()
	result, err := visionHost.Vision().AnalyzeImage(ctx, url, "")
	if err != nil {
		p.logger.Debug("steal analyze failed", "error", err)
		return
	}
	if !result.IsMeme || !result.FitsPersona {
		return
	}
	if rand.Float64() > p.cfg.Probability {
		return
	}
	if _, err := memeHost.Memes().SaveFromURL(ctx, url); err != nil {
		p.logger.Debug("save meme failed", "error", err)
	}
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Observer = (*Plugin)(nil)
