package jargonlearn_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"icarosu/internal/plugins/reflection/jargonlearn"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestLearnJargonAndProvide(t *testing.T) {
	host := testkit.NewFakeHost()
	host.LLMValue = &testkit.FakeLLM{Reply: `{"term":"摸鱼","meaning":"偷懒"}`}
	p := jargonlearn.New().(*jargonlearn.Plugin)
	if err := p.Init(context.Background(), host); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	for i := 0; i < 20; i++ {
		p.Observe(context.Background(), sdk.Event{
			Kind:         "message.created",
			Conversation: sdk.Conversation{ID: "qq:group:1"},
			Parts:        []sdk.Part{{Type: sdk.PartText, Text: "摸鱼"}},
		})
	}
	deadline := time.Now().Add(time.Second)
	for {
		blocks, _ := p.Provide(context.Background(), &sdk.Turn{Event: sdk.Event{Conversation: sdk.Conversation{ID: "qq:group:1"}}})
		if len(blocks) > 0 && strings.Contains(blocks[0].Content, "摸鱼") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("jargonlearn did not produce context")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
