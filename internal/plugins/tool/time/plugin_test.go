package timeplugin_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"icarosu/internal/kernel"
	timeplugin "icarosu/internal/plugins/tool/time"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestManifestAndTool(t *testing.T) {
	p := timeplugin.New()
	m := p.Manifest()
	if m.ID != "tool.time" || m.Kind != sdk.KindTool {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if len(m.Tools) != 1 || m.Tools[0].Name != "time_now" {
		t.Fatalf("unexpected tools: %+v", m.Tools)
	}

	host := testkit.NewFakeHost()
	host.ClockValue = testkit.NewFakeClock(time.Date(2026, 10, 25, 22, 30, 0, 0, time.UTC))
	if err := p.Init(context.Background(), host); err != nil {
		t.Fatalf("Init: %v", err)
	}
	tools := p.(sdk.ToolProvider).Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	res, err := tools[0].Execute(context.Background(), sdk.ToolCall{Name: "time_now"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Text, "2026-10-25") {
		t.Fatalf("unexpected result: %q", res.Text)
	}
}

func TestKernelBootsTimePlugin(t *testing.T) {
	k := kernel.New(kernel.Options{})
	if err := k.Register(sdk.PluginSpec{New: timeplugin.New, Critical: false}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := k.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
