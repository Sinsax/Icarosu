package trigger

import (
	"context"
	"log/slog"

	"icarosu/internal/attention"
	"icarosu/internal/config"
	coretrigger "icarosu/internal/core/trigger"
	"icarosu/internal/plugin"
)

// TriggerPlugin 决定群消息是否需要继续交给后续插件处理。
// 核心决策逻辑已下沉到 internal/core/trigger。
type TriggerPlugin struct {
	plugin.BasePlugin
	svc *coretrigger.Service
}

// Option 用于配置 TriggerPlugin。
type Option func(*TriggerPlugin)

// WithAttention 设置注意力系统。
func WithAttention(t *attention.Tracker) Option {
	return func(p *TriggerPlugin) {
		if p.svc != nil {
			coretrigger.WithAttention(t)(p.svc)
		}
	}
}

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *TriggerPlugin) {
		if p.svc != nil {
			coretrigger.WithLogger(logger)(p.svc)
		}
	}
}

func New(cfg config.BotConfig, opts ...Option) *TriggerPlugin {
	p := &TriggerPlugin{}
	p.svc = coretrigger.New(cfg)
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Update 热更新触发/人设相关配置。
func (p *TriggerPlugin) Update(cfg config.BotConfig) {
	if p.svc != nil {
		p.svc.Update(cfg)
	}
}

func (p *TriggerPlugin) Name() string {
	return "trigger"
}

func (p *TriggerPlugin) Description() string {
	return "根据 @、关键词、兴趣、概率和冷却决定是否回复群消息"
}

func (p *TriggerPlugin) Priority() int {
	return 900
}

func (p *TriggerPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *TriggerPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if p.svc == nil {
		return false
	}
	return !p.svc.ShouldReply(ctx, plugin.ToDomainMessage(e))
}

// HandleContext 实现 ContextualPlugin，从流水线中间产物中补充图片上下文。
func (p *TriggerPlugin) HandleContext(ctx context.Context, bot plugin.Bot, pc *plugin.Context) bool {
	e := pc.Event
	if v, ok := pc.Get("image_context").(string); ok && e.ImageContext == "" {
		e.ImageContext = v
	}
	return p.Handle(ctx, bot, e)
}
