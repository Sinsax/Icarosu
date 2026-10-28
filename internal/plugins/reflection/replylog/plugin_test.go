package replylog_test

import (
	"context"
	"testing"

	"icarosu/internal/plugins/reflection/replylog"
	"icarosu/internal/sdk"
)

func TestObserve(t *testing.T) {
	p := replylog.New().(*replylog.Plugin)
	p.Observe(context.Background(), sdk.Event{Kind: "message.created", Conversation: sdk.Conversation{ID: "qq:group:1"}})
}
