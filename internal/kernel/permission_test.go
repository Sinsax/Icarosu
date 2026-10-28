package kernel_test

import (
	"context"
	"testing"

	"icarosu/internal/kernel"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

type permissionPlugin struct {
	llmVisible    bool
	memoryVisible bool
	visionVisible bool
}

func (p *permissionPlugin) Manifest() sdk.Manifest {
	return sdk.Manifest{APIVersion: 1, ID: "tool.perm", Version: "0.1.0", Kind: sdk.KindTool, Name: "perm", Provides: []string{"tool.perm"}}
}
func (p *permissionPlugin) Init(ctx context.Context, host sdk.Host) error {
	p.llmVisible = host.LLM() != nil
	if _, err := host.Memory().Recall(ctx, "x", 1); err == nil {
		p.memoryVisible = true
	}
	if vh, ok := host.(sdk.VisionHost); ok && vh.Vision() != nil {
		p.visionVisible = true
	}
	return nil
}
func (p *permissionPlugin) Start(ctx context.Context) error       { return nil }
func (p *permissionPlugin) Stop(ctx context.Context) error        { return nil }
func (p *permissionPlugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func TestHostCapabilitiesRequireManifestPermission(t *testing.T) {
	p := &permissionPlugin{}
	k := kernel.New(kernel.Options{
		LLM:    &testkit.FakeLLM{Reply: "ok"},
		Memory: &testkit.FakeMemory{},
		Vision: &testkit.FakeVision{},
	})
	if err := k.Register(sdk.PluginSpec{New: func() sdk.Plugin { return p }}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if p.llmVisible || p.memoryVisible || p.visionVisible {
		t.Fatalf("unpermitted capabilities leaked: %+v", p)
	}
}
