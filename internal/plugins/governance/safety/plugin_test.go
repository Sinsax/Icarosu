package safety_test

import (
	"context"
	"testing"

	"icarosu/internal/plugins/governance/safety"
	"icarosu/internal/sdk"
)

func TestIncomingPolicy(t *testing.T) {
	p := safety.NewWithConfig(safety.Config{Forbidden: []string{"广告"}})
	checker := p.(sdk.Policy)
	if err := checker.Check(context.Background(), &sdk.Turn{Event: sdk.Event{Parts: []sdk.Part{{Type: sdk.PartText, Text: "这里有广告"}}}}); err == nil {
		t.Fatal("expected incoming policy to block")
	}
	if err := checker.Check(context.Background(), &sdk.Turn{Event: sdk.Event{Parts: []sdk.Part{{Type: sdk.PartText, Text: "正常聊天"}}}}); err != nil {
		t.Fatalf("clean message rejected: %v", err)
	}
}

func TestOutputPolicyBlocksReply(t *testing.T) {
	p := safety.NewWithConfig(safety.Config{Forbidden: []string{"广告"}})
	checker := p.(sdk.OutputPolicy)
	if err := checker.CheckOutput(context.Background(), &sdk.Turn{}, "这里有广告"); err == nil {
		t.Fatal("expected output policy to block reply")
	}
	if err := checker.CheckOutput(context.Background(), &sdk.Turn{}, "正常回复"); err != nil {
		t.Fatalf("clean reply rejected: %v", err)
	}
}
