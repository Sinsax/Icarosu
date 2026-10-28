package handler

import (
	"context"
	"log/slog"
	"testing"

	"icarosu/internal/plugin"
	"icarosu/internal/qq"
)

func TestSessionKeysDiffer(t *testing.T) {
	if groupKey(1) == userKey(1) {
		t.Fatal("group and user keys must differ")
	}
}

func TestHandleMessageAndNotice(t *testing.T) {
	h := New(nil, plugin.NewManager(), 1, slog.Default())
	ctx := context.Background()
	h.Handle(ctx, qq.Event{PostType: "message", MessageType: "group", GroupID: 100, UserID: 200, RawMessage: "hi"})
	h.Handle(ctx, qq.Event{PostType: "message", MessageType: "private", UserID: 200, RawMessage: "hi"})
	h.Handle(ctx, qq.Event{PostType: "notice", NoticeType: "group_recall", GroupID: 100, UserID: 200, MessageID: 9})
	h.Handle(ctx, qq.Event{PostType: "meta_event", SubType: "heartbeat"})
}
