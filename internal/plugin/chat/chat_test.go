package chat_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"icarosu/internal/llm"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/chat"
	"icarosu/internal/semantic"
	"icarosu/internal/topic"
)

type mockBot struct {
	mu       sync.Mutex
	groupMsg []string
	private  []string
}

func (m *mockBot) SendGroupMessage(_ context.Context, _ int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupMsg = append(m.groupMsg, message)
	return nil
}

func (m *mockBot) SendPrivateMessage(_ context.Context, _ int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.private = append(m.private, message)
	return nil
}

func (m *mockBot) SendGroupImage(_ context.Context, _ int64, file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupMsg = append(m.groupMsg, "image:"+file)
	return nil
}

func (m *mockBot) SendPrivateImage(_ context.Context, _ int64, file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.private = append(m.private, "image:"+file)
	return nil
}

func newTestChatPlugin(t *testing.T) (plugin.Plugin, *mockBot) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你好呀"}}]}`))
	}))
	t.Cleanup(server.Close)

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	bot := &mockBot{}
	return chat.New(client, "你是群友"), bot
}

func TestChatHandlesGroupMessageWithoutAt(t *testing.T) {
	p, bot := newTestChatPlugin(t)

	handled := p.Handle(context.Background(), bot, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    123,
		RawMessage: "你好",
		AtBot:      false,
	})
	if !handled {
		t.Fatal("chat plugin should handle group message that passed trigger")
	}
	if len(bot.groupMsg) != 1 || bot.groupMsg[0] != "你好呀" {
		t.Fatalf("unexpected group messages: %v", bot.groupMsg)
	}
}

func TestChatHandlesGroupMessageWithAt(t *testing.T) {
	p, bot := newTestChatPlugin(t)

	handled := p.Handle(context.Background(), bot, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    123,
		RawMessage: "你好",
		AtBot:      true,
	})
	if !handled {
		t.Fatal("should handle group message with @")
	}
	if len(bot.groupMsg) != 1 || bot.groupMsg[0] != "你好呀" {
		t.Fatalf("unexpected group messages: %v", bot.groupMsg)
	}
}

func TestChatHandlesPrivateMessage(t *testing.T) {
	p, bot := newTestChatPlugin(t)

	handled := p.Handle(context.Background(), bot, &plugin.Event{
		Type:       plugin.EventPrivateMessage,
		UserID:     456,
		RawMessage: "在吗",
	})
	if !handled {
		t.Fatal("should handle private message")
	}
	if len(bot.private) != 1 || bot.private[0] != "你好呀" {
		t.Fatalf("unexpected private messages: %v", bot.private)
	}
}

func TestChatIncludesImageContext(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"嗯"}}]}`))
	}))
	t.Cleanup(server.Close)

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	p := chat.New(client, "你是群友")
	bot := &mockBot{}
	if !p.Handle(context.Background(), bot, &plugin.Event{
		Type:         plugin.EventGroupMessage,
		GroupID:      1,
		RawMessage:   "",
		ImageContext: "一只猫在睡觉",
		AtBot:        true,
	}) {
		t.Fatal("should handle image message")
	}

	if !strings.Contains(got, "一只猫在睡觉") {
		t.Fatalf("expected image context in LLM request, got: %s", got)
	}
}

func TestChatIncludesTopicStabilityHint(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"看番呢"}}]}`))
	}))
	t.Cleanup(server.Close)

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	tr := topic.NewTracker(semantic.NewLocalEmbedder(128), 0.55)
	ctx := context.Background()
	tr.Observe(ctx, 123, "看什么番")
	tr.Observe(ctx, 123, "看什么番")
	tr.RecordReply(ctx, 123, "看番呢，别催")

	p := chat.New(client, "你是群友", chat.WithTopicTracker(tr))
	bot := &mockBot{}
	if !p.Handle(ctx, bot, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    123,
		RawMessage: "看什么番",
		AtBot:      true,
	}) {
		t.Fatal("should handle message")
	}

	if !strings.Contains(got, "当前稳定话题") || !strings.Contains(got, "看番呢，别催") {
		t.Fatalf("expected topic stability hint in LLM request, got: %s", got)
	}
}
