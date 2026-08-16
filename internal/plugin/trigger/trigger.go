package trigger

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"icarosu/internal/attention"
	"icarosu/internal/config"
	"icarosu/internal/plugin"
)

// TriggerPlugin 决定群消息是否需要继续交给后续插件处理。
// 私聊默认放行；群聊根据 @、关键词、兴趣、概率和冷却决定。
type TriggerPlugin struct {
	plugin.BasePlugin
	cfg       config.BotConfig
	attention *attention.Tracker
	botNames  []string
	logger    *slog.Logger

	mu          sync.Mutex
	cfgMu       sync.RWMutex
	lastReplied map[int64]time.Time
}

// Option 用于配置 TriggerPlugin。
type Option func(*TriggerPlugin)

// WithAttention 设置注意力系统。
func WithAttention(t *attention.Tracker) Option {
	return func(p *TriggerPlugin) {
		p.attention = t
	}
}

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *TriggerPlugin) {
		p.logger = logger
	}
}

func New(cfg config.BotConfig, opts ...Option) *TriggerPlugin {
	p := &TriggerPlugin{
		cfg:         cfg,
		logger:      slog.Default(),
		lastReplied: make(map[int64]time.Time),
	}
	if cfg.Name != "" {
		p.botNames = append(p.botNames, cfg.Name)
	}
	p.botNames = append(p.botNames, cfg.Nicknames...)
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Update 热更新触发/人设相关配置。
func (p *TriggerPlugin) Update(cfg config.BotConfig) {
	p.cfgMu.Lock()
	defer p.cfgMu.Unlock()
	p.cfg = cfg
	p.botNames = p.botNames[:0]
	if cfg.Name != "" {
		p.botNames = append(p.botNames, cfg.Name)
	}
	p.botNames = append(p.botNames, cfg.Nicknames...)
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

func (p *TriggerPlugin) Handle(_ context.Context, _ plugin.Bot, e *plugin.Event) bool {
	p.cfgMu.RLock()
	defer p.cfgMu.RUnlock()

	if e.Type == plugin.EventPrivateMessage {
		return false
	}
	if e.Type != plugin.EventGroupMessage {
		return false
	}

	msg := strings.ToLower(e.RawMessage)

	// 注意力系统：先根据当前消息更新注意力
	if p.attention != nil {
		p.attention.Observe(
			e.GroupID,
			e.AtBot,
			p.mentioned(msg),
			p.matchAny(msg, p.cfg.Interests),
		)
	}

	// 被 @ 时必回
	if e.AtBot && p.cfg.MentionReply {
		return false
	}

	// 冷却时间受注意力影响：注意力越高冷却越短
	cooldown := p.cfg.CooldownSeconds
	if p.attention != nil {
		cooldown = p.attention.CooldownSeconds(e.GroupID, cooldown)
	}
	if p.inCooldown(e.GroupID, cooldown) {
		if p.logger != nil {
			p.logger.Debug("trigger blocked by cooldown", "group_id", e.GroupID)
		}
		return true
	}

	if p.matchAny(msg, p.cfg.Keywords) {
		p.markReplied(e.GroupID)
		return false
	}
	if p.cfg.InterestReply && p.matchAny(msg, p.cfg.Interests) {
		p.markReplied(e.GroupID)
		if p.logger != nil {
			p.logger.Info("trigger by interest", "group_id", e.GroupID, "message", e.RawMessage)
		}
		return false
	}

	prob := p.cfg.RandomProbability
	if p.attention != nil {
		prob = p.attention.EffectiveProbability(e.GroupID, prob)
	}
	if prob > 0 && rand.Float64() < prob {
		p.markReplied(e.GroupID)
		if p.logger != nil {
			p.logger.Info("trigger by random", "group_id", e.GroupID, "message", e.RawMessage, "probability", prob)
		}
		return false
	}

	return true
}

func (p *TriggerPlugin) matchAny(msg string, keys []string) bool {
	for _, k := range keys {
		if k != "" && strings.Contains(msg, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func (p *TriggerPlugin) mentioned(msg string) bool {
	return p.matchAny(msg, p.botNames)
}

func (p *TriggerPlugin) inCooldown(groupID int64, cooldownSeconds int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	last, ok := p.lastReplied[groupID]
	if !ok {
		return false
	}
	cd := time.Duration(cooldownSeconds) * time.Second
	return time.Since(last) < cd
}

func (p *TriggerPlugin) markReplied(groupID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastReplied[groupID] = time.Now()
}
