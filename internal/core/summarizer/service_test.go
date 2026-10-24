package summarizer

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/llm"
	"icarosu/internal/store"
)

type fakeLLM struct{}

func (fakeLLM) Chat(_ context.Context, _ []llm.Message) (string, error) {
	return "群聊总结内容", nil
}

func (fakeLLM) ChatWithTools(_ context.Context, _ []llm.Message, _ []llm.Tool, _ llm.ToolExecutor) (string, error) {
	return "", nil
}

func TestSummarizeGroupSavesMemory(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".summarizer-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	st, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	for i := 0; i < 3; i++ {
		if err := st.SaveMessage(100, 200, "小明", "今天聊点什么呢", "user"); err != nil {
			t.Fatalf("save message: %v", err)
		}
	}

	svc := New(fakeLLM{}, st, config.MemoryConfig{}, WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	svc.SummarizeGroup(context.Background(), 100)

	memories, err := st.GetMemories(100, 10)
	if err != nil {
		t.Fatalf("get memories: %v", err)
	}
	if len(memories) != 1 || memories[0].Content != "群聊总结内容" {
		t.Fatalf("unexpected memories: %+v", memories)
	}
}
