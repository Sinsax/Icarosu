package attention

import (
	"math"
	"sync"
	"time"

	"icarosu/internal/config"
)

// Tracker 是注意力系统。
// 它维护每个群/会话的注意力分数，分数越高越倾向于回复。
type Tracker struct {
	cfg config.AttentionConfig

	mu        sync.Mutex
	score     map[int64]float64
	last      map[int64]time.Time
	peakSince map[int64]time.Time
}

func New(cfg config.AttentionConfig) *Tracker {
	if cfg.MaxScore <= 0 {
		cfg.MaxScore = 100
	}
	return &Tracker{
		cfg:       cfg,
		score:     make(map[int64]float64),
		last:      make(map[int64]time.Time),
		peakSince: make(map[int64]time.Time),
	}
}

// Update 热更新注意力配置。
func (t *Tracker) Update(cfg config.AttentionConfig) {
	if cfg.MaxScore <= 0 {
		cfg.MaxScore = 100
	}
	t.mu.Lock()
	t.cfg = cfg
	t.mu.Unlock()
}

// Observe 根据一条消息更新注意力。
// atBot: 是否 @ 了机器人；mentioned: 是否提到了 bot 名字；interested: 是否命中兴趣。
func (t *Tracker) Observe(key int64, atBot, mentioned, interested bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cfg := t.cfg
	if !cfg.Enabled {
		return
	}
	var boost float64
	if atBot {
		boost += cfg.BoostAtBot
	}
	if mentioned {
		boost += cfg.BoostMention
	}
	if interested {
		boost += cfg.BoostInterest
	}
	if boost == 0 {
		return
	}

	now := time.Now()
	t.applyDecayLocked(key, now)
	t.score[key] = clamp(t.score[key]+boost, 0, cfg.MaxScore)
	t.last[key] = now
	t.checkTripLocked(key, now)
}

// OnBotReplied 在机器人回复后提升注意力。
func (t *Tracker) OnBotReplied(key int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cfg := t.cfg
	if !cfg.Enabled {
		return
	}
	now := time.Now()
	t.applyDecayLocked(key, now)
	t.score[key] = clamp(t.score[key]+cfg.BoostBotReplied, 0, cfg.MaxScore)
	t.last[key] = now
	t.checkTripLocked(key, now)
}

// Scores 返回所有已记录会话的当前注意力分数。
func (t *Tracker) Scores() map[int64]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[int64]float64, len(t.score))
	now := time.Now()
	for key := range t.score {
		t.applyDecayLocked(key, now)
		out[key] = t.score[key]
	}
	return out
}

// Score 返回当前注意力分数（会先衰减）。
func (t *Tracker) Score(key int64) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.applyDecayLocked(key, time.Now())
	return t.score[key]
}

// EffectiveProbability 根据注意力调整基础随机回复概率。
func (t *Tracker) EffectiveProbability(key int64, base float64) float64 {
	t.mu.Lock()
	cfg := t.cfg
	t.applyDecayLocked(key, time.Now())
	score := t.score[key]
	t.mu.Unlock()

	if !cfg.Enabled {
		return base
	}
	if cfg.MaxScore <= 0 {
		return base
	}
	// 注意力满时最多额外增加 0.2 概率
	boost := (score / cfg.MaxScore) * 0.2
	prob := base + boost
	if prob > 0.5 {
		prob = 0.5
	}
	return prob
}

// CooldownSeconds 根据注意力缩短冷却时间。
func (t *Tracker) CooldownSeconds(key int64, base int) int {
	if base <= 0 {
		return base
	}

	t.mu.Lock()
	cfg := t.cfg
	t.applyDecayLocked(key, time.Now())
	score := t.score[key]
	t.mu.Unlock()

	if !cfg.Enabled || cfg.MaxScore <= 0 {
		return base
	}
	// 注意力越高，冷却越短，最高缩短一半
	reduced := float64(base) * (1 - 0.5*score/cfg.MaxScore)
	if reduced < 1 {
		reduced = 1
	}
	return int(math.Round(reduced))
}

func (t *Tracker) applyDecayLocked(key int64, now time.Time) {
	last, ok := t.last[key]
	if !ok {
		t.last[key] = now
		return
	}
	minutes := now.Sub(last).Minutes()
	if minutes <= 0 {
		t.checkTripLocked(key, now)
		return
	}
	decay := minutes * t.cfg.DecayPerMinute
	t.score[key] = clamp(t.score[key]-decay, 0, t.cfg.MaxScore)
	t.last[key] = now
	t.checkTripLocked(key, now)
}

// checkTripLocked 熔断检查：注意力持续处于最高值一段时间后，直接归零。
func (t *Tracker) checkTripLocked(key int64, now time.Time) {
	if t.cfg.TripHoldSeconds <= 0 {
		return
	}
	if t.score[key] >= t.cfg.MaxScore {
		if t.peakSince[key].IsZero() {
			t.peakSince[key] = now
			return
		}
		if now.Sub(t.peakSince[key]) >= time.Duration(t.cfg.TripHoldSeconds)*time.Second {
			t.score[key] = 0
			delete(t.peakSince, key)
			t.last[key] = now
		}
		return
	}
	delete(t.peakSince, key)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
