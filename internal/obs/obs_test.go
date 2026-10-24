package obs

import (
	"testing"
	"time"
)

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics()
	m.IncMessagesReceived()
	m.IncRepliesSent()
	m.IncLLMRequests()
	m.IncLLMErrors()

	snap := m.Snapshot()
	if snap["messages_received"] != 1 || snap["replies_sent"] != 1 || snap["llm_requests"] != 1 || snap["llm_errors"] != 1 {
		t.Fatalf("unexpected snapshot: %v", snap)
	}
}

func TestSpanLifecycle(t *testing.T) {
	tr := NewTracer()
	s := tr.StartSpan("test")
	time.Sleep(time.Millisecond)
	s.End()
	if s.Duration() <= 0 {
		t.Fatalf("expected positive duration, got %v", s.Duration())
	}
}
