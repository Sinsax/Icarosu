package recent_test

import (
	"context"
	"strings"
	"testing"

	"icarosu/internal/plugins/memory/recent"
	"icarosu/internal/sdk"
)

func TestRecentMemory(t *testing.T) {
	p := recent.New().(*recent.Plugin)
	p.Observe(context.Background(), sdk.Event{
		Kind:         "message.created",
		Conversation: sdk.Conversation{ID: "qq:group:1"},
		Actor:        sdk.Actor{Name: "小明"},
		Parts:        []sdk.Part{{Type: sdk.PartText, Text: "早上好"}},
	})
	blocks, err := p.Provide(context.Background(), &sdk.Turn{Event: sdk.Event{Conversation: sdk.Conversation{ID: "qq:group:1"}}})
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	if len(blocks) != 1 || !strings.Contains(blocks[0].Content, "小明: 早上好") {
		t.Fatalf("unexpected blocks: %+v", blocks)
	}
}
