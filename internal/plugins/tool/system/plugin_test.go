package system_test

import (
	"context"
	"testing"

	"icarosu/internal/plugins/tool/system"
	"icarosu/internal/sdk"
)

func TestPingTool(t *testing.T) {
	p := system.New().(*system.Plugin)
	tools := p.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	res, err := tools[0].Execute(context.Background(), sdk.ToolCall{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Text != "pong" {
		t.Fatalf("expected pong, got %q", res.Text)
	}
}
