package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"icarosu/internal/qq"
)

func TestClientHandlesMessageAndSends(t *testing.T) {
	updates := make(chan qq.Event, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "getUpdates"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":{"message_id":10,"from":{"id":123,"first_name":"Tom"},"chat":{"id":-100,"type":"group"},"text":"hello"}}]}`))
		case strings.Contains(r.URL.Path, "sendMessage"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	c := NewClient("test-token", nil)
	c.baseURL = server.URL
	c.SetMessageHandler(func(_ context.Context, evt qq.Event) {
		updates <- evt
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	select {
	case evt := <-updates:
		if evt.MessageType != "group" || evt.GroupID != -100 || evt.UserID != 123 || evt.RawMessage != "hello" {
			t.Fatalf("unexpected event: %+v", evt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for telegram update")
	}

	if err := c.SendGroupMessage(ctx, -100, "hi"); err != nil {
		t.Fatalf("SendGroupMessage: %v", err)
	}
}
