package topic

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"icarosu/internal/semantic"
)

// State 表示某个群当前的话题状态。
type State struct {
	Topic       string
	Vec         []float32
	LastUserMsg string
	LastReply   string
	Stability   int
	UpdatedAt   time.Time
}

// Tracker 跟踪每个群的话题稳定性，并在稳定话题上提供一致性提示。
type Tracker struct {
	mu        sync.Mutex
	embedder  semantic.Embedder
	threshold float64
	states    map[int64]*State
}

func NewTracker(embedder semantic.Embedder, threshold float64) *Tracker {
	if embedder == nil {
		embedder = semantic.NewLocalEmbedder(256)
	}
	if threshold <= 0 {
		threshold = 0.55
	}
	return &Tracker{
		embedder:  embedder,
		threshold: threshold,
		states:    make(map[int64]*State),
	}
}

// Observe 观察一条用户消息，更新话题状态。
func (t *Tracker) Observe(ctx context.Context, groupID int64, message string) {
	message = trimMessage(message)
	if message == "" {
		return
	}
	vec, err := t.embedder.Embed(ctx, message)
	if err != nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	st := t.states[groupID]
	if st == nil || len(st.Vec) == 0 || cosineSimilarity(st.Vec, vec) < t.threshold {
		t.states[groupID] = &State{
			Topic:       message,
			Vec:         vec,
			LastUserMsg: message,
			Stability:   1,
			UpdatedAt:   time.Now(),
		}
		return
	}

	st.Stability++
	st.LastUserMsg = message
	st.UpdatedAt = time.Now()
}

// RecordReply 记录机器人当前话题下的回复，用于后续一致性约束。
func (t *Tracker) RecordReply(ctx context.Context, groupID int64, reply string) {
	reply = trimMessage(reply)
	if reply == "" {
		return
	}
	_ = ctx

	t.mu.Lock()
	defer t.mu.Unlock()
	if st := t.states[groupID]; st != nil {
		st.LastReply = reply
		st.UpdatedAt = time.Now()
	}
}

// Hint 返回话题一致性提示；如果当前不是稳定话题则返回空字符串。
func (t *Tracker) Hint(ctx context.Context, groupID int64, currentMessage string) string {
	currentMessage = trimMessage(currentMessage)
	if currentMessage == "" {
		return ""
	}
	vec, err := t.embedder.Embed(ctx, currentMessage)
	if err != nil {
		return ""
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	st := t.states[groupID]
	if st == nil || st.Stability < 2 || st.LastReply == "" || len(st.Vec) == 0 {
		return ""
	}
	if cosineSimilarity(st.Vec, vec) < t.threshold {
		return ""
	}
	return fmt.Sprintf("当前稳定话题：%s\n你之前在这个话题下说过：%s\n对方还在问同一件事，请延续这个说法，不要改口。", st.Topic, st.LastReply)
}

func trimMessage(s string) string {
	s = trimSpace(s)
	if len(s) > 200 {
		return string([]rune(s)[:200])
	}
	return s
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpaceByte(s[start]) {
		start++
	}
	for end > start && isSpaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
