package vlm_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"icarosu/internal/vlm"
)

func TestAnalyzeParsesResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"description\":\"一张摸鱼表情包\",\"is_meme\":true,\"fits_persona\":true,\"reason\":\"很符合摸鱼人设\",\"tags\":[\"摸鱼\",\"表情包\"],\"name\":\"摸鱼图\"}"}}]}`)
	}))
	defer server.Close()

	client := vlm.NewClient(vlm.Config{
		BaseURL: server.URL,
		Model:   "test-vlm",
		Prompt:  "test",
		Timeout: 5 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	result, err := client.Analyze(context.Background(), "http://example.com/a.png", "摸鱼人设")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if result.Description != "一张摸鱼表情包" || !result.IsMeme || !result.FitsPersona || result.Name != "摸鱼图" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
