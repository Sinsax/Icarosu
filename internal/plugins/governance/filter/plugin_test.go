package filter_test

import (
	"context"
	"testing"

	"icarosu/internal/plugins/governance/filter"
	"icarosu/internal/sdk"
)

func TestFilterPolicy(t *testing.T) {
	p := filter.New().(*filter.Plugin)
	_ = p.Init(context.Background(), nil)
	// 默认 disabled：全部放行
	if err := p.Check(context.Background(), &sdk.Turn{Event: sdk.Event{
		Conversation: sdk.Conversation{ID: "qq:group:100", Kind: "group", Metadata: map[string]string{"native_id": "100"}},
	}}); err != nil {
		t.Fatalf("disabled filter should pass: %v", err)
	}
}
