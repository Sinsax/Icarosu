package summarizer

import (
	"context"
	"log/slog"

	"icarosu/internal/config"
	coresummarizer "icarosu/internal/core/summarizer"
	"icarosu/internal/llm"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/semantic"
	"icarosu/internal/store"
)

// SummarizerPlugin 定期或在消息量达到阈值时总结群聊，写入长期记忆。
// 核心逻辑已下沉到 internal/core/summarizer。
type SummarizerPlugin struct {
	plugin.BasePlugin
	svc *coresummarizer.Service
}

// Option 用于配置 SummarizerPlugin。
type Option func(*SummarizerPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *SummarizerPlugin) {
		if p.svc != nil {
			coresummarizer.WithLogger(logger)(p.svc)
		}
	}
}

// WithSemantic 设置语义记忆服务。
func WithSemantic(svc *semantic.Service) Option {
	return func(p *SummarizerPlugin) {
		if p.svc != nil {
			coresummarizer.WithSemantic(svc)(p.svc)
		}
	}
}

func New(client llm.Provider, mem *memory.Memory, st store.SummaryStore, cfg config.MemoryConfig, opts ...Option) *SummarizerPlugin {
	p := &SummarizerPlugin{}
	p.svc = coresummarizer.New(client, st, cfg)
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *SummarizerPlugin) Name() string {
	return "summarizer"
}

func (p *SummarizerPlugin) Description() string {
	return "按消息量或时间间隔总结群聊并写入长期记忆"
}

func (p *SummarizerPlugin) Priority() int {
	return 920
}

func (p *SummarizerPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *SummarizerPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if p.svc != nil {
		p.svc.Observe(ctx, plugin.ToDomainMessage(e))
	}
	return false
}

// SummarizeGroup 手动触发一次群总结。
func (p *SummarizerPlugin) SummarizeGroup(ctx context.Context, groupID int64) {
	if p.svc != nil {
		p.svc.SummarizeGroup(ctx, groupID)
	}
}
