// Package trigger 提供群消息触发决策核心逻辑。
package trigger

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"icarosu/internal/config"
	"icarosu/internal/core/domain"
)

// AttentionTracker 是触发决策依赖的注意力能力。
type AttentionTracker interface {
	Observe(key int64, atBot, mentioned, interested bool)
	CooldownSeconds(key int64, base int) int
	EffectiveProbability(key int64, base float64) float64
}

// Service 决定群消息是否需要继续交给后续插件处理。
// 私聊默认放行；群聊根据 @、关键词、兴趣、概率和冷却决定。
type Service struct {
	cfg       config.BotConfig
	attention AttentionTracker
	botNames  []string
	logger    *slog.Logger

	mu          sync.Mutex
	cfgMu       sync.RWMutex
	lastReplied map[int64]time.Time
}

// Option 用于配置 Service。
type Option func(*Service)

// WithAttention 设置注意力系统。
func WithAttention(t AttentionTracker) Option {
	return func(s *Service) {
		s.attention = t
	}
}

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// New 创建触发决策服务。
func New(cfg config.BotConfig, opts ...Option) *Service {
	s := &Service{
		cfg:         cfg,
		logger:      slog.Default(),
		lastReplied: make(map[int64]time.Time),
	}
	if cfg.Name != "" {
		s.botNames = append(s.botNames, cfg.Name)
	}
	s.botNames = append(s.botNames, cfg.Nicknames...)
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Update 热更新触发/人设相关配置。
func (s *Service) Update(cfg config.BotConfig) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	s.cfg = cfg
	s.botNames = s.botNames[:0]
	if cfg.Name != "" {
		s.botNames = append(s.botNames, cfg.Name)
	}
	s.botNames = append(s.botNames, cfg.Nicknames...)
}

// ShouldReply 返回 true 表示当前消息应继续交给后续插件处理。
func (s *Service) ShouldReply(_ context.Context, m *domain.ChatMessage) bool {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()

	if m.Type == domain.MessagePrivate {
		return true
	}
	if m.Type != domain.MessageGroup {
		return false
	}

	msg := strings.ToLower(m.RawMessage)
	matchText := msg
	if m.ImageContext != "" {
		matchText += " " + strings.ToLower(m.ImageContext)
	}

	// 注意力系统：先根据当前消息更新注意力
	if s.attention != nil {
		s.attention.Observe(
			m.GroupID,
			m.AtBot,
			s.mentioned(matchText),
			s.matchAny(matchText, s.cfg.Interests),
		)
	}

	// 被 @ 时必回
	if m.AtBot && s.cfg.MentionReply {
		return true
	}

	// 冷却时间受注意力影响：注意力越高冷却越短
	cooldown := s.cfg.CooldownSeconds
	if s.attention != nil {
		cooldown = s.attention.CooldownSeconds(m.GroupID, cooldown)
	}
	if s.inCooldown(m.GroupID, cooldown) {
		if s.logger != nil {
			s.logger.Debug("trigger blocked by cooldown", "group_id", m.GroupID)
		}
		return false
	}

	if s.matchAny(matchText, s.cfg.Keywords) {
		s.markReplied(m.GroupID)
		return true
	}
	if s.cfg.InterestReply && s.matchAny(matchText, s.cfg.Interests) {
		s.markReplied(m.GroupID)
		if s.logger != nil {
			s.logger.Info("trigger by interest", "group_id", m.GroupID, "message", m.RawMessage)
		}
		return true
	}

	prob := s.cfg.RandomProbability
	if s.attention != nil {
		prob = s.attention.EffectiveProbability(m.GroupID, prob)
	}
	if prob > 0 && rand.Float64() < prob {
		s.markReplied(m.GroupID)
		if s.logger != nil {
			s.logger.Info("trigger by random", "group_id", m.GroupID, "message", m.RawMessage, "probability", prob)
		}
		return true
	}

	return false
}

func (s *Service) matchAny(msg string, keys []string) bool {
	for _, k := range keys {
		if k != "" && strings.Contains(msg, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func (s *Service) mentioned(msg string) bool {
	return s.matchAny(msg, s.botNames)
}

func (s *Service) inCooldown(groupID int64, cooldownSeconds int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	last, ok := s.lastReplied[groupID]
	if !ok {
		return false
	}
	cd := time.Duration(cooldownSeconds) * time.Second
	return time.Since(last) < cd
}

func (s *Service) markReplied(groupID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastReplied[groupID] = time.Now()
}