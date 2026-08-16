package llm_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"icarosu/internal/llm"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestChatReturnsReply(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你好呀"}}]}`))
	}))
	defer server.Close()

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, discardLogger())

	reply, err := client.Chat(context.Background(), []llm.Message{
		{Role: "user", Content: "你好"},
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if reply != "你好呀" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotBody["model"] != "test-model" {
		t.Fatalf("unexpected model: %v", gotBody["model"])
	}
	if gotBody["stream"] != false {
		t.Fatalf("stream should be false, got %v", gotBody["stream"])
	}
}

func TestChatReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, discardLogger())

	_, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("error should contain status code, got: %v", err)
	}
}

func TestChatWithToolsExecutesToolCall(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{}"}}]}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"现在是 12:00"}}]}`)
	}))
	defer server.Close()

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, discardLogger())

	var executed bool
	reply, err := client.ChatWithTools(
		context.Background(),
		[]llm.Message{{Role: "user", Content: "几点了"}},
		[]llm.Tool{{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_time",
				Description: "获取时间",
				Parameters:  map[string]any{},
			},
		}},
		func(ctx context.Context, name, arguments string) (string, error) {
			executed = true
			if name != "get_time" {
				t.Fatalf("unexpected tool name: %s", name)
			}
			return "12:00", nil
		},
	)
	if err != nil {
		t.Fatalf("ChatWithTools returned error: %v", err)
	}
	if !executed {
		t.Fatal("expected tool to be executed")
	}
	if reply != "现在是 12:00" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if calls != 2 {
		t.Fatalf("expected 2 api calls, got %d", calls)
	}
}

func TestChatNoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, discardLogger())

	_, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}
