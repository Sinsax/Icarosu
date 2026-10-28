package qq

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCloseWithoutConnectionIsSafe(t *testing.T) {
	c := NewClient("ws://127.0.0.1:1", "", slog.Default())
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.SendGroupRaw(context.Background(), 1, "x"); err == nil {
		t.Fatal("send without connection should fail")
	}
}

func apiServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req map[string]any
			_ = json.Unmarshal(data, &req)
			echo, _ := req["echo"].(string)
			status, retcode := "ok", 0
			if fail {
				status, retcode = "failed", 100
			}
			_ = conn.WriteJSON(map[string]any{
				"status": status, "retcode": retcode, "echo": echo,
				"data": map[string]any{"message_id": 1},
			})
		}
	}))
}

func TestConnectAfterClose(t *testing.T) {
	srv := apiServer(t, false)
	defer srv.Close()
	c := NewClient("ws"+strings.TrimPrefix(srv.URL, "http"), "", slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	if err := c.SendGroupRaw(ctx, 1, "hi"); err != nil {
		t.Fatalf("send after reconnect: %v", err)
	}
	_ = c.Close()
}

func TestSendWaitsForAPISuccess(t *testing.T) {
	srv := apiServer(t, false)
	defer srv.Close()
	c := NewClient("ws"+strings.TrimPrefix(srv.URL, "http"), "", slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()
	if err := c.SendGroupRaw(ctx, 1, "hi"); err != nil {
		t.Fatalf("SendGroupRaw: %v", err)
	}
}

func TestSendReportsAPIFailure(t *testing.T) {
	srv := apiServer(t, true)
	defer srv.Close()
	c := NewClient("ws"+strings.TrimPrefix(srv.URL, "http"), "", slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()
	if err := c.SendGroupRaw(ctx, 1, "hi"); err == nil {
		t.Fatal("expected API failure")
	}
}
