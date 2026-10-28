package kernel_test

import (
	"context"
	"sync/atomic"
	"testing"

	"icarosu/internal/kernel"
	"icarosu/internal/sdk"
)

type racePlugin struct {
	state atomic.Int32
}

func (p *racePlugin) Manifest() sdk.Manifest {
	return sdk.Manifest{
		APIVersion: 1,
		ID:         "tool.race",
		Version:    "0.1.0",
		Kind:       sdk.KindTool,
		Name:       "race",
		Provides:   []string{"tool.race"},
	}
}
func (p *racePlugin) Init(ctx context.Context, host sdk.Host) error { p.state.Add(1); return nil }
func (p *racePlugin) Start(ctx context.Context) error               { p.state.Add(1); return nil }
func (p *racePlugin) Stop(ctx context.Context) error                { p.state.Add(1); return nil }
func (p *racePlugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }

func TestKernelAdminViewConcurrentWithToggle(t *testing.T) {
	k := kernel.New(kernel.Options{})
	if err := k.Register(sdk.PluginSpec{New: func() sdk.Plugin { return &racePlugin{} }}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = k.Stop(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_ = k.Health(ctx)
			_ = k.PluginStatuses()
			_ = k.Plugins()
			_ = k.RecentTraces(10)
			_ = k.Metrics()
		}
	}()
	for i := 0; i < 50; i++ {
		if err := k.SetPluginEnabled(ctx, "tool.race", i%2 == 0); err != nil {
			t.Fatalf("toggle: %v", err)
		}
	}
	<-done
}
