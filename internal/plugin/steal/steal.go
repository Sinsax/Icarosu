package steal

import (
	"context"
	"log/slog"

	"icarosu/internal/config"
	"icarosu/internal/plugin"
	"icarosu/internal/vision"
)

// StealPlugin 在 VLM 通用图片理解基础上，把符合人设的表情包收藏进本地表情库。
type StealPlugin struct {
	plugin.BasePlugin
	cfg     config.MemeConfig
	vision  *vision.Service
	persona string
	logger  *slog.Logger
}

// Option 用于配置 StealPlugin。
type Option func(*StealPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *StealPlugin) {
		p.logger = logger
	}
}

func New(cfg config.MemeConfig, visionSvc *vision.Service, persona string, opts ...Option) plugin.Plugin {
	p := &StealPlugin{
		cfg:     cfg,
		vision:  visionSvc,
		persona: persona,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *StealPlugin) Name() string {
	return "steal"
}

func (p *StealPlugin) Description() string {
	return "在图片识别基础上，把符合人设的表情包收藏进表情库"
}

func (p *StealPlugin) Priority() int {
	return 940
}

func (p *StealPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *StealPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage {
		return false
	}
	if p.vision == nil || len(e.Images) == 0 {
		return false
	}

	for _, img := range e.Images {
		if img.URL == "" {
			continue
		}
		if err := p.vision.MaybeSteal(ctx, img.URL, p.persona); err != nil {
			if p.logger != nil {
				p.logger.Warn("steal meme failed", "error", err)
			}
			continue
		}
		if p.logger != nil {
			p.logger.Info("steal checked", "group_id", e.GroupID, "user_id", e.UserID, "image", img.URL)
		}
		// 一条消息最多处理一张，避免刷库。
		break
	}
	return false
}
