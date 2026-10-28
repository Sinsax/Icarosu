package attention_test

import (
	"context"
	"testing"

	"icarosu/internal/plugins/drive/attention"
	"icarosu/internal/sdk"
)

func TestDecideAtBotAndPrivate(t *testing.T) {
	p := attention.NewWithConfig(attention.Config{}).(*attention.Plugin)
	_ = p.Init(context.Background(), nil)

	at, _ := p.Decide(context.Background(), &sdk.Turn{Event: sdk.Event{
		Kind:         "message.created",
		Conversation: sdk.Conversation{Kind: "group"},
		Self:         "qq:user:1",
		Mentions:     []sdk.ActorID{"qq:user:1"},
	}})
	if at.Kind != sdk.DecisionReply {
		t.Fatalf("expected at_bot reply, got %+v", at)
	}

	private, _ := p.Decide(context.Background(), &sdk.Turn{Event: sdk.Event{
		Kind:         "message.created",
		Conversation: sdk.Conversation{Kind: "private"},
	}})
	if private.Kind != sdk.DecisionReply {
		t.Fatalf("expected private reply, got %+v", private)
	}

	observe, _ := p.Decide(context.Background(), &sdk.Turn{Event: sdk.Event{
		Kind:         "message.created",
		Conversation: sdk.Conversation{Kind: "group"},
		Self:         "qq:user:1",
	}})
	if observe.Kind != sdk.DecisionObserve {
		t.Fatalf("expected observe, got %+v", observe)
	}
}
