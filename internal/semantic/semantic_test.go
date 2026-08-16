package semantic_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/semantic"
	"icarosu/internal/store"
)

func TestLocalEmbedderDeterministic(t *testing.T) {
	e := semantic.NewLocalEmbedder(64)
	ctx := context.Background()

	v1, err := e.Embed(ctx, "小明喜欢看番")
	if err != nil {
		t.Fatalf("embed 1: %v", err)
	}
	v2, err := e.Embed(ctx, "小明喜欢看番")
	if err != nil {
		t.Fatalf("embed 2: %v", err)
	}
	if len(v1) != 64 || len(v2) != 64 {
		t.Fatalf("unexpected dims: %d %d", len(v1), len(v2))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("embedding not deterministic at %d: %v != %v", i, v1[i], v2[i])
		}
	}
}

func TestSemanticServiceSearch(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".semantic-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	st, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := semantic.NewService(st, semantic.NewLocalEmbedder(128), nil)
	ctx := context.Background()

	if err := svc.SaveMemory(ctx, 100, 200, "小明喜欢看番"); err != nil {
		t.Fatalf("save memory: %v", err)
	}
	if err := svc.SaveMemory(ctx, 100, 200, "今天天气不错"); err != nil {
		t.Fatalf("save memory 2: %v", err)
	}

	records, err := svc.Search(ctx, 100, "最近在看什么番", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one memory")
	}
	if records[0].Content != "小明喜欢看番" {
		t.Fatalf("expected semantic top to be 小明喜欢看番, got %q", records[0].Content)
	}
}
