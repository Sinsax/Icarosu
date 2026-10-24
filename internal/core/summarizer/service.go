// Package summarizer 提供群聊自动总结核心业务。
package summarizer

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"icarosu/internal/config"
	"icarosu/internal/core/domain"
	"icarosu/internal/llm"
	"icarosu/internal/store"
)

// MemorySaver 是总结结果写入长期记忆的能力。
type MemorySaver interface {
	SaveMemory(ctx context.Context, groupID, userID int64, content string) error
}

// Service 定期或在消息量达到阈值时总结群聊，写入长期记忆。
type Service struct {
	client   llm.Provider
	store    store.SummaryStore
	semantic MemorySaver
	cfg      config.MemoryConfig
	logger   *slog.Logger

	mu          sync.Mutex
	counts      map[int64]int
	lastSummary map[int64]time.Time
}

// Option 用于配置 Service。
type Option func(*Service)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithSemantic 设置语义记忆服务。
func WithSemantic(svc MemorySaver) Option {
	return func(s *Service) {
		s.semantic = svc
	}
}

// New 创建群聊总结服务。
func New(client llm.Provider, st store.SummaryStore, cfg config.MemoryConfig, opts ...Option) *Service {
	s := &Service{
		client:      client,
		store:       st,
		cfg:         cfg,
		logger:      slog.Default(),
		counts:      make(map[int64]int),
		lastSummary: make(map[int64]time.Time),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Observe 在群消息到来时更新计数，并在达到阈值或间隔时触发总结。
func (s *Service) Observe(ctx context.Context, m *domain.ChatMessage) {
	if m.Type != domain.MessageGroup {
		return
	}
	if s.client == nil || s.store == nil {
		return
	}

	s.mu.Lock()
	s.counts[m.GroupID]++
	count := s.counts[m.GroupID]
	last := s.lastSummary[m.GroupID]
	threshold := s.cfg.SummaryThreshold
	if threshold <= 0 {
		threshold = 50
	}
	interval := time.Duration(s.cfg.SummaryIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 60 * time.Minute
	}
	should := count >= threshold || (!last.IsZero() && time.Since(last) >= interval)
	s.mu.Unlock()

	if should {
		s.SummarizeGroup(ctx, m.GroupID)
	}
}

// SummarizeGroup 手动触发一次群总结。
func (s *Service) SummarizeGroup(ctx context.Context, groupID int64) {
	if s.client == nil || s.store == nil {
		return
	}
	records, err := s.store.RecentMessages(groupID, 30)
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
	summary, err := s.client.Chat(ctx, messages)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("summarize failed", "group_id", groupID, "error", err)
		}
		return
	}

	var saveErr error
	if s.semantic != nil {
		saveErr = s.semantic.SaveMemory(ctx, groupID, 0, summary)
	} else {
		saveErr = s.store.SaveMemory(groupID, 0, summary)
	}
	if saveErr != nil {
		if s.logger != nil {
			s.logger.Warn("save summary failed", "group_id", groupID, "error", saveErr)
		}
		return
	}

	s.mu.Lock()
	s.counts[groupID] = 0
	s.lastSummary[groupID] = time.Now()
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Info("group summarized", "group_id", groupID, "summary", summary)
	}
}