package topic_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/semantic"
	"icarosu/internal/store"
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

func TestTrackerPersistsAcrossInstances(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".topic-store-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	st, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	tr1 := topic.NewTracker(semantic.NewLocalEmbedder(128), 0.55, topic.WithStore(st))
	tr1.Observe(ctx, 1, "看什么番")
	tr1.Observe(ctx, 1, "看什么番")
	tr1.RecordReply(ctx, 1, "看番呢，别催")

	tr2 := topic.NewTracker(semantic.NewLocalEmbedder(128), 0.55, topic.WithStore(st))
	hint := tr2.Hint(ctx, 1, "看什么番")
	if !strings.Contains(hint, "看番呢，别催") {
		t.Fatalf("expected persisted hint after reload, got %q", hint)
	}
}
