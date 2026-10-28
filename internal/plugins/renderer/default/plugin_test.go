package renderer_test

import (
	"context"
	"testing"

	"icarosu/internal/plugins/renderer/default"
	"icarosu/internal/sdk"
)

func TestRenderText(t *testing.T) {
	p := renderer.New().(*renderer.Plugin)
	actions, err := p.Render(context.Background(), &sdk.Turn{
		Event: sdk.Event{
			Conversation: sdk.Conversation{ID: "qq:group:1", AdapterID: "onebot", Kind: "group"},
			Self:         "qq:user:1",
			Meta:         map[string]string{"native_message_id": "9"},
		},
	}, "  在的  ")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(actions) != 1 || actions[0].Parts[0].Text != "在的" || actions[0].ReplyTo != "9" {
		t.Fatalf("unexpected actions: %+v", actions)
	}
}
