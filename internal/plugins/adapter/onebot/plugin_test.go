package onebot_test

import (
	"context"
	"testing"

	"icarosu/bundles/standard"
	"icarosu/internal/kernel"
	"icarosu/internal/plugins/adapter/onebot"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestPluginKernelEndToEnd(t *testing.T) {
	client := &fakeClient{}
	k := kernel.New(kernel.Options{LLM: &testkit.FakeLLM{Reply: "在的"}})
	if err := k.Register(standard.Plugins()...); err != nil {
		t.Fatalf("register standard: %v", err)
	}
	if err := k.Register(sdk.PluginSpec{New: func() sdk.Plugin {
		return onebot.NewPlugin(client, nil)
	}, Critical: true}); err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = k.Stop(context.Background()) }()

	if !client.connected {
		t.Fatal("adapter should be started by kernel")
	}
	if _, err := k.HandleEvent(context.Background(), sdk.Event{
		ID:   "e1",
		Kind: "message.created",
		Conversation: sdk.Conversation{
			ID:        "qq:user:200",
			AdapterID: "onebot",
			Kind:      "private",
			Metadata:  map[string]string{"native_id": "200"},
		},
		Actor: sdk.Actor{ID: "qq:user:200", Name: "小明"},
		Self:  "qq:user:1",
		Parts: []sdk.Part{{Type: sdk.PartText, Text: "在吗"}},
		Meta:  map[string]string{"native_message_id": "9"},
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(client.private) != 1 {
		t.Fatalf("expected private send, got %#v", client.private)
	}
}
