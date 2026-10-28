package steal_test

import (
	"context"
	"testing"
	"time"

	"icarosu/internal/plugins/reflection/steal"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestStealSavesMeme(t *testing.T) {
	host := testkit.NewFakeHost()
	host.VisionValue = &testkit.FakeVision{Analysis: sdk.ImageAnalysis{IsMeme: true, FitsPersona: true}}
	host.MemeValue = &testkit.FakeMemeStore{}
	p := steal.NewWithConfig(steal.Config{Probability: 1}).(*steal.Plugin)
	if err := p.Init(context.Background(), host); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	p.Observe(context.Background(), sdk.Event{
		Kind:         "message.created",
		Conversation: sdk.Conversation{ID: "qq:group:1", Kind: "group"},
		Parts:        []sdk.Part{{Type: sdk.PartImage, URL: "https://img.example/a.png"}},
	})
	deadline := time.Now().Add(time.Second)
	for {
		if len(host.MemeValue.SavedURLs()) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("meme was not saved")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
