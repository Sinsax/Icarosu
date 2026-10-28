package summarizer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"icarosu/internal/plugins/reflection/summarizer"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestSummarizeAfterThreshold(t *testing.T) {
	host := testkit.NewFakeHost()
	host.LLMValue = &testkit.FakeLLM{Reply: "大家在聊看番"}
	p := summarizer.New().(*summarizer.Plugin)
	if err := p.Init(context.Background(), host); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	for i := 0; i < 50; i++ {
		p.Observe(context.Background(), sdk.Event{
			Kind:         "message.created",
			Conversation: sdk.Conversation{ID: "qq:group:1"},
			Parts:        []sdk.Part{{Type: sdk.PartText, Text: "看番"}},
		})
	}
	deadline := time.Now().Add(time.Second)
	for {
		blocks, _ := p.Provide(context.Background(), &sdk.Turn{Event: sdk.Event{Conversation: sdk.Conversation{ID: "qq:group:1"}}})
		if len(blocks) > 0 && strings.Contains(blocks[0].Content, "看番") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("summarizer did not produce context")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
