package summarizer

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"icarosu/internal/config"
	"icarosu/internal/llm"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/store"
)

// SummarizerPlugin 定期或在消息量达到阈值时总结群聊，写入长期记忆。
type SummarizerPlugin struct {
	plugin.BasePlugin
	client *llm.Client
	mem    *memory.Memory
	store  *store.Store
	cfg    config.MemoryConfig
	logger *slog.Logger

	mu          sync.Mutex
	counts      map[int64]int
	lastSummary map[int64]time.Time
}

// Option 用于配置 SummarizerPlugin。
type Option func(*SummarizerPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *SummarizerPlugin) {
		p.logger = logger
	}
}

func New(client *llm.Client, mem *memory.Memory, st *store.Store, cfg config.MemoryConfig, opts ...Option) plugin.Plugin {
	p := &SummarizerPlugin{
		client:      client,
		mem:         mem,
		store:       st,
		cfg:         cfg,
		logger:      slog.Default(),
		counts:      make(map[int64]int),
		lastSummary: make(map[int64]time.Time),
	}
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
	if e.Type != plugin.EventGroupMessage {
		return false
	}
	if p.client == nil || p.store == nil {
		return false
	}

	p.mu.Lock()
	p.counts[e.GroupID]++
	count := p.counts[e.GroupID]
	last := p.lastSummary[e.GroupID]
	threshold := p.cfg.SummaryThreshold
	if threshold <= 0 {
		threshold = 50
	}
	interval := time.Duration(p.cfg.SummaryIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 60 * time.Minute
	}
	should := count >= threshold || (!last.IsZero() && time.Since(last) >= interval)
	p.mu.Unlock()

	if should {
		p.summarize(ctx, e.GroupID)
	}
	return false
}

func (p *SummarizerPlugin) summarize(ctx context.Context, groupID int64) {
	records, err := p.store.RecentMessages(groupID, 30)
	if err != nil || len(records) == 0 {
		return
	}

	var b strings.Builder
	for _, r := range records {
		name := r.SenderName
		if name == "" {
			name = "用户"
		}
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(r.Content)
		b.WriteString("\n")
	}

	messages := []llm.Message{
		{Role: "system", Content: "你是群聊总结助手。请用简洁的中文总结这段群聊的核心话题、关键信息和人物关系，不超过200字。"},
		{Role: "user", Content: b.String()},
	}
	summary, err := p.client.Chat(ctx, messages)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("summarize failed", "group_id", groupID, "error", err)
		}
		return
	}

	if err := p.store.SaveMemory(groupID, 0, summary); err != nil {
		if p.logger != nil {
			p.logger.Warn("save summary failed", "group_id", groupID, "error", err)
		}
		return
	}

	p.mu.Lock()
	p.counts[groupID] = 0
	p.lastSummary[groupID] = time.Now()
	p.mu.Unlock()

	if p.logger != nil {
		p.logger.Info("group summarized", "group_id", groupID, "summary", summary)
	}
}
