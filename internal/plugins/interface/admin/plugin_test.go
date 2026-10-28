package admin_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"icarosu/internal/plugins/interface/admin"
	"icarosu/internal/sdk"
)

type fakeView struct{}

func (fakeView) PluginStatuses() []sdk.PluginStatus {
	return []sdk.PluginStatus{{ID: "persona.default", Health: sdk.Healthy()}}
}
func (fakeView) PluginManual(id string) (string, bool) { return "manual for " + id, true }
func (fakeView) ToolManual(name string) (string, bool) { return "tool manual for " + name, true }
func (fakeView) ToolSchema(name string) (json.RawMessage, bool) {
	return json.RawMessage("{\"type\":\"object\"}"), true
}
func (fakeView) Metrics() map[string]int64             { return map[string]int64{"turns_total": 1} }
func (fakeView) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }
func (fakeView) RecentTraces(limit int) []sdk.Trace {
	return []sdk.Trace{{TurnID: "t1"}}
}

func TestAdminTools(t *testing.T) {
	p := admin.New(fakeView{}, nil).(*admin.Plugin)
	tools := p.Tools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	res, err := tools[1].Execute(context.Background(), sdk.ToolCall{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Text, "persona.default") {
		t.Fatalf("unexpected result: %q", res.Text)
	}
	res, err = tools[2].Execute(context.Background(), sdk.ToolCall{Arguments: []byte("{\"target\":\"tool:time_now\"}")})
	if err != nil || !strings.Contains(res.Text, "tool manual") {
		t.Fatalf("read_manual: %v %q", err, res.Text)
	}
}
