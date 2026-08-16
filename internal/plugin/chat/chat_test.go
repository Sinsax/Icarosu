package chat_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"icarosu/internal/llm"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/chat"
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
