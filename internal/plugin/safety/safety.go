package safety

import (
	"context"
	"log/slog"
	"strings"

	"icarosu/internal/config"
	"icarosu/internal/plugin"
)

// SafetyPlugin 是一个简单的安全过滤插件。
// 它会拦截包含 bot.forbidden 中定义内容的群聊/私聊消息，避免进入 LLM 或触发回复。
type SafetyPlugin struct {
	plugin.BasePlugin
	forbidden []string
	logger    *slog.Logger
}

// Option 用于配置 SafetyPlugin。
type Option func(*SafetyPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *SafetyPlugin) {
		p.logger = logger
	}
}

// New 创建安全过滤插件。
func New(cfg config.BotConfig, opts ...Option) plugin.Plugin {
	p := &SafetyPlugin{
		forbidden: append([]string(nil), cfg.Forbidden...),
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *SafetyPlugin) Name() string {
	return "safety"
}

func (p *SafetyPlugin) Description() string {
	return "按 bot.forbidden 内容过滤敏感消息"
}

func (p *SafetyPlugin) Priority() int {
	return 990
}

func (p *SafetyPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *SafetyPlugin) Handle(_ context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage && e.Type != plugin.EventPrivateMessage {
		return false
	}
	msg := strings.ToLower(e.RawMessage)
	for _, f := range p.forbidden {
		if f != "" && strings.Contains(msg, strings.ToLower(f)) {
			if p.logger != nil {
				p.logger.Warn("message blocked by safety filter",
					"type", e.Type,
					"group_id", e.GroupID,
					"user_id", e.UserID,
					"keyword", f,
				)
			}
			return true
		}
	}
	return false
}
