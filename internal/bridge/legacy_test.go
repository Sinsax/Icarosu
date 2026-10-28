package bridge_test

import (
	"testing"

	"icarosu/internal/bridge"
	"icarosu/internal/plugin"
	"icarosu/internal/sdk"
)

func TestToLegacyGroupMessage(t *testing.T) {
	ev := sdk.Event{
		Kind: "message.created",
		Self: "qq:user:1",
		Conversation: sdk.Conversation{
			ID:       "qq:group:100",
			Kind:     "group",
			Metadata: map[string]string{"native_id": "100"},
		},
		Actor: sdk.Actor{ID: "qq:user:200", Name: "小明"},
		Parts: []sdk.Part{
			{Type: sdk.PartText, Text: "早上好"},
			{Type: sdk.PartImage, URL: "https://img.example/a.png", Meta: map[string]string{"file": "a.png"}},
		},
		Mentions: []sdk.ActorID{"qq:user:1"},
		ReplyTo:  "456",
		Meta: map[string]string{
			"native_group_id":   "100",
			"native_user_id":    "200",
			"native_message_id": "9",
		},
	}
	legacy, ok := bridge.ToLegacyEvent(ev)
	if !ok {
		t.Fatal("expected conversion")
	}
	if legacy.Type != plugin.EventGroupMessage || legacy.GroupID != 100 || legacy.UserID != 200 {
		t.Fatalf("unexpected legacy event: %+v", legacy)
	}
	if !legacy.AtBot || legacy.RawMessage != "早上好" || len(legacy.Images) != 1 {
		t.Fatalf("unexpected message fields: %+v", legacy)
	}
	if legacy.ReplyToMessageID != 456 {
		t.Fatalf("unexpected reply id: %d", legacy.ReplyToMessageID)
	}
}

func TestToLegacyRecall(t *testing.T) {
	ev := sdk.Event{
		Kind: "notice.recalled",
		Conversation: sdk.Conversation{
			ID:   "qq:group:100",
			Kind: "group",
		},
		Actor: sdk.Actor{ID: "qq:user:200"},
		Meta: map[string]string{
			"native_group_id":    "100",
			"native_user_id":     "200",
			"native_message_id":  "9",
			"native_operator_id": "300",
		},
	}
	legacy, ok := bridge.ToLegacyEvent(ev)
	if !ok {
		t.Fatal("expected conversion")
	}
	if legacy.Type != plugin.EventGroupRecall || legacy.OperatorID != 300 || legacy.MessageID != 9 {
		t.Fatalf("unexpected recall event: %+v", legacy)
	}
}
