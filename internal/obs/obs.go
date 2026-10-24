// Package obs 提供轻量可观测性能力：指标计数和简单 Span 追踪。
package obs

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 是运行时指标集合。
type Metrics struct {
	MessagesReceived atomic.Int64
	RepliesSent      atomic.Int64
	LLMRequests      atomic.Int64
	LLMErrors        atomic.Int64
}

// NewMetrics 创建指标集合。
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncMessagesReceived 记录收到一条平台消息。
func (m *Metrics) IncMessagesReceived() {
	if m != nil {
		m.MessagesReceived.Add(1)
	}
}

// IncRepliesSent 记录发送一条回复。
func (m *Metrics) IncRepliesSent() {
	if m != nil {
		m.RepliesSent.Add(1)
	}
}

// IncLLMRequests 记录一次 LLM 请求。
func (m *Metrics) IncLLMRequests() {
	if m != nil {
		m.LLMRequests.Add(1)
	}
}

// IncLLMErrors 记录一次 LLM 请求错误。
func (m *Metrics) IncLLMErrors() {
	if m != nil {
		m.LLMErrors.Add(1)
	}
}

// Snapshot 返回当前指标快照。
func (m *Metrics) Snapshot() map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return map[string]int64{
		"messages_received": m.MessagesReceived.Load(),
		"replies_sent":      m.RepliesSent.Load(),
		"llm_requests":      m.LLMRequests.Load(),
		"llm_errors":        m.LLMErrors.Load(),
	}
}

// Span 是一个简单的时间段追踪单元。
type Span struct {
	mu       sync.Mutex
	name     string
	start    time.Time
	fields   map[string]string
	ended    bool
	duration time.Duration
}

// Tracer 创建 Span。
type Tracer struct{}

// NewTracer 创建 Tracer。
func NewTracer() *Tracer {
	return &Tracer{}
}

// StartSpan 开始一个 Span。
func (t *Tracer) StartSpan(name string) *Span {
	return &Span{
		name:   name,
		start:  time.Now(),
		fields: make(map[string]string),
	}
}

// SetField 设置 Span 字段。
func (s *Span) SetField(key, value string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fields[key] = value
}

// End 结束 Span 并记录耗时。
func (s *Span) End() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.duration = time.Since(s.start)
	s.ended = true
}

// Duration 返回 Span 耗时；未结束时为 0。
func (s *Span) Duration() time.Duration {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.duration
}
