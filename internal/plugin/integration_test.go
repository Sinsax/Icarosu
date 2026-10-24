package plugin_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "icarosu/internal/plugin/builtin"

	"icarosu/internal/config"
	"icarosu/internal/llm"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/chat"
	"icarosu/internal/plugin/recorder"
	"icarosu/internal/plugin/trigger"
)

func TestFullPipelineChatSenderIntegration(t *testing.T) {
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

	mem := memory.New(20, nil)
	bot := &mockBot{}

	m := plugin.NewManager()
	for _, p := range plugin.GlobalPlugins() {
		m.Register(p)
	}
	m.Register(recorder.New(mem))
	m.Register(trigger.New(config.BotConfig{
		MentionReply:    true,
		CooldownSeconds: 60,
	}))
	m.Register(chat.New(client, "你是群友", chat.WithMemory(mem)))

	m.Dispatch(context.Background(), bot, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    123,
		UserID:     456,
		RawMessage: "你好",
		AtBot:      true,
	})

	if len(bot.messages) != 1 || bot.messages[0] != "group:你好呀" {
		t.Fatalf("expected sender to send one reply, got %v", bot.messages)
	}
}
