package standard_test

import (
	"context"
	"testing"

	"icarosu/bundles/standard"
	"icarosu/internal/kernel"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestStandardBundleBootsAndReplies(t *testing.T) {
	k := kernel.New(kernel.Options{LLM: &testkit.FakeLLM{Reply: "在的"}})
	if err := k.Register(standard.Plugins()...); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = k.Stop(context.Background()) }()

	res, err := k.HandleEvent(context.Background(), sdk.Event{
		ID:   "e1",
		Kind: "message.created",
		Conversation: sdk.Conversation{
			ID:        "qq:private:200",
			AdapterID: "onebot",
			Kind:      "private",
			Metadata:  map[string]string{"native_id": "200"},
		},
		Actor: sdk.Actor{ID: "qq:user:200", Name: "小明"},
		Self:  "qq:user:1",
		Parts: []sdk.Part{{Type: sdk.PartText, Text: "在吗"}},
		Meta:  map[string]string{"native_message_id": "9"},
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if res.Decision.Kind != sdk.DecisionReply {
		t.Fatalf("expected reply decision, got %+v", res.Decision)
	}
	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %+v", res.Actions)
	}
	if traces := k.RecentTraces(1); len(traces) != 1 || traces[0].TurnID != res.TurnID {
		t.Fatalf("expected trace for turn, got %+v", traces)
	}
	if plugins := k.PluginStatuses(); len(plugins) < 5 {
		t.Fatalf("expected plugin statuses, got %+v", plugins)
	}
}
