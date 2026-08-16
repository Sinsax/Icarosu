package recorder

import (
	"context"
	"log/slog"
	"time"

	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/store"
	"icarosu/internal/vision"
)

// RecorderPlugin 记录所有通过过滤器的消息到短期/长期记忆。
// 如果配置了 vision，还会识别图片内容并写入上下文。
// 同时收集引用回复、撤回、活跃时间段、互动关系。
type RecorderPlugin struct {
	plugin.BasePlugin
	mem    *memory.Memory
	store  *store.Store
	vision *vision.Service
	logger *slog.Logger
}

// Option 用于配置 RecorderPlugin。
type Option func(*RecorderPlugin)

// WithStore 设置数据库存储。
func WithStore(st *store.Store) Option {
	return func(p *RecorderPlugin) {
		p.store = st
	}
}

// WithVision 设置图片识别服务。
func WithVision(v *vision.Service) Option {
	return func(p *RecorderPlugin) {
		p.vision = v
	}
}

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *RecorderPlugin) {
		p.logger = logger
	}
}

func New(mem *memory.Memory, opts ...Option) plugin.Plugin {
	p := &RecorderPlugin{
		mem:    mem,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *RecorderPlugin) Name() string {
	return "recorder"
}

func (p *RecorderPlugin) Description() string {
	return "记录消息、图片、引用、撤回、活跃时间和互动关系到数据库"
}

func (p *RecorderPlugin) Priority() int {
	return 950
}

func (p *RecorderPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if e.Type == plugin.EventGroupRecall {
		if p.store != nil {
			_ = p.store.SaveRecall(e.GroupID, e.MessageID, e.UserID, e.OperatorID)
		}
		if p.logger != nil {
			p.logger.Debug("recorded recall", "group_id", e.GroupID, "message_id", e.MessageID)
		}
		return false
	}

	if e.Type != plugin.EventGroupMessage && e.Type != plugin.EventPrivateMessage {
		return false
	}
	if p.mem == nil {
		return false
	}

	p.mem.RecordIncoming(e)

	if p.store != nil && e.Type == plugin.EventGroupMessage {
		_ = p.store.RecordActivity(e.GroupID, e.UserID, time.Now())
		if e.ReplyToMessageID != 0 {
			_ = p.store.SaveReply(e.GroupID, e.UserID, e.MessageID, e.ReplyToMessageID)
		}
		if e.ReplyToUserID != 0 {
			_ = p.store.RecordInteraction(e.GroupID, e.UserID, e.ReplyToUserID)
		}
	}

	if p.vision != nil {
		for _, img := range e.Images {
			if img.URL == "" {
				continue
			}
			desc, err := p.vision.Understand(ctx, img.URL, "")
			if err != nil {
				if p.logger != nil {
					p.logger.Warn("image understand failed", "error", err)
				}
				continue
			}
			p.mem.RecordImageContext(e, desc)
			if p.logger != nil {
				p.logger.Debug("recorded image context", "group_id", e.GroupID, "user_id", e.UserID, "description", desc)
			}
		}
	}

	if p.logger != nil {
		p.logger.Debug("recorded message", "type", e.Type, "group_id", e.GroupID, "user_id", e.UserID)
	}
	return false
}
