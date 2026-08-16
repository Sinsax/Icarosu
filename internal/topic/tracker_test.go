package topic_test

import (
	"context"
	"strings"
	"testing"

	"icarosu/internal/semantic"
	"icarosu/internal/topic"
)

func TestTrackerStableTopicHint(t *testing.T) {
	tr := topic.NewTracker(semantic.NewLocalEmbedder(128), 0.55)
	ctx := context.Background()

	tr.Observe(ctx, 1, "看什么番")
	tr.Observe(ctx, 1, "看什么番")
	tr.RecordReply(ctx, 1, "看番呢，别催")

	hint := tr.Hint(ctx, 1, "看什么番")
	if hint == "" {
		t.Fatal("expected topic consistency hint for stable topic")
	}
	if !strings.Contains(hint, "看番呢，别催") {
		t.Fatalf("hint should contain last reply, got %q", hint)
	}
}

func TestTrackerTopicSwitchResetsStability(t *testing.T) {
	tr := topic.NewTracker(semantic.NewLocalEmbedder(128), 0.55)
	ctx := context.Background()

	tr.Observe(ctx, 1, "看什么番")
	tr.Observe(ctx, 1, "看什么番")
	tr.RecordReply(ctx, 1, "看番呢，别催")

	// 切换到完全无关的话题后，不应再输出旧话题的一致性提示
	tr.Observe(ctx, 1, "今天天气不错")
	if hint := tr.Hint(ctx, 1, "今天天气不错"); hint != "" {
		t.Fatalf("expected no hint after topic switch, got %q", hint)
	}
}

func TestTrackerNoHintBeforeStable(t *testing.T) {
	tr := topic.NewTracker(semantic.NewLocalEmbedder(128), 0.55)
	ctx := context.Background()

	tr.Observe(ctx, 1, "看什么番")
	tr.RecordReply(ctx, 1, "看番呢")

	if hint := tr.Hint(ctx, 1, "看什么番"); hint != "" {
		t.Fatalf("expected no hint before topic is stable, got %q", hint)
	}
}
