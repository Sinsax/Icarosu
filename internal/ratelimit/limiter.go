package ratelimit

import (
	"sync"
	"time"
)

// Limiter 是一个简单的按 key 限流器（令牌桶）。
type Limiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens map[string]float64
	last   map[string]time.Time
}

func New(rate, burst float64) *Limiter {
	if rate <= 0 {
		rate = 5
	}
	if burst <= 0 {
		burst = rate
	}
	return &Limiter{
		rate:   rate,
		burst:  burst,
		tokens: make(map[string]float64),
		last:   make(map[string]time.Time),
	}
}

// Update 热更新限流参数。
func (l *Limiter) Update(rate, burst float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if rate > 0 {
		l.rate = rate
	}
	if burst > 0 {
		l.burst = burst
	}
}

// Allow 判断 key 是否允许通过。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	last, ok := l.last[key]
	if !ok {
		l.tokens[key] = l.burst
		l.last[key] = now
	} else {
		elapsed := now.Sub(last).Seconds()
		l.tokens[key] += elapsed * l.rate
		if l.tokens[key] > l.burst {
			l.tokens[key] = l.burst
		}
		l.last[key] = now
	}

	if l.tokens[key] < 1 {
		return false
	}
	l.tokens[key]--
	return true
}
