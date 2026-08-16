package filter

import (
	"context"
	"log/slog"

	"icarosu/internal/config"
	"icarosu/internal/plugin"
)

// FilterPlugin 是消息过滤器。
// 它处理群消息和私聊消息：不在允许/黑名单范围内的消息会被直接消费，避免后续插件响应。
type FilterPlugin struct {
	plugin.BasePlugin
	cfg    config.FilterConfig
	logger *slog.Logger
}

// Option 用于配置 FilterPlugin。
type Option func(*FilterPlugin)

// WithLogger 设置过滤器日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *FilterPlugin) {
		p.logger = logger
	}
}

func New(cfg config.FilterConfig, opts ...Option) plugin.Plugin {
	p := &FilterPlugin{
		cfg:    cfg,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *FilterPlugin) Name() string {
	return "filter"
}

func (p *FilterPlugin) Description() string {
	return "按群号/QQ号白名单或黑名单过滤群聊和私聊消息"
}

func (p *FilterPlugin) Priority() int {
	// 最高优先级，最先执行，确保过滤先于其他业务插件。
	return 1000
}

func (p *FilterPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *FilterPlugin) Handle(_ context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if !p.cfg.Enabled {
		return false
	}

	var allowed bool
	switch e.Type {
	case plugin.EventGroupMessage:
		allowed = p.allowedByMode(e.GroupID, p.cfg.GroupIDs)
	case plugin.EventPrivateMessage:
		allowed = p.allowedByMode(e.UserID, p.cfg.UserIDs)
	default:
		return false
	}

	if !allowed {
		if p.logger != nil {
			p.logger.Debug("message blocked by filter",
				"type", e.Type,
				"group_id", e.GroupID,
				"user_id", e.UserID,
				"message", e.RawMessage,
			)
		}
		// 消费掉该事件，后续插件不再处理。
		return true
	}

	if p.logger != nil {
		p.logger.Info("message passed filter",
			"type", e.Type,
			"group_id", e.GroupID,
			"user_id", e.UserID,
			"at_bot", e.AtBot,
			"message", e.RawMessage,
		)
	}
	return false
}

func (p *FilterPlugin) allowedByMode(id int64, ids []int64) bool {
	switch p.cfg.Mode {
	case "deny":
		return !contains(ids, id)
	default:
		// 默认 allow 白名单模式
		return contains(ids, id)
	}
}

func contains(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
