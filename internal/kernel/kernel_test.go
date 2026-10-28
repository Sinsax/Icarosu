package kernel_test

import (
	"context"
	"errors"
	"testing"

	"icarosu/internal/kernel"
	"icarosu/internal/sdk"
)

type fakePlugin struct {
	manifest  sdk.Manifest
	order     *[]string
	name      string
	initErr   error
	startErr  error
	panicInit bool
	hostHas   *bool
}

func (p *fakePlugin) Manifest() sdk.Manifest { return p.manifest }

func (p *fakePlugin) Init(ctx context.Context, host sdk.Host) error {
	if p.hostHas != nil {
		*p.hostHas = host.Has("kernel.tools")
	}
	if p.panicInit {
		panic("init boom")
	}
	*p.order = append(*p.order, "init:"+p.name)
	return p.initErr
}

func (p *fakePlugin) Start(ctx context.Context) error {
	*p.order = append(*p.order, "start:"+p.name)
	return p.startErr
}

func (p *fakePlugin) Stop(ctx context.Context) error {
	*p.order = append(*p.order, "stop:"+p.name)
	return nil
}

func (p *fakePlugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func manifest(id string, provides, requires []string) sdk.Manifest {
	return sdk.Manifest{
		APIVersion: 1,
		ID:         id,
		Version:    "0.1.0",
		Kind:       sdk.KindTool,
		Name:       id,
		Provides:   provides,
		Requires:   requires,
	}
}

func TestKernelLifecycleOrder(t *testing.T) {
	var order []string
	hasKernelTools := false
	k := kernel.New(kernel.Options{})
	err := k.Register(
		sdk.PluginSpec{New: func() sdk.Plugin {
			return &fakePlugin{
				manifest: manifest("tool.a", []string{"a"}, []string{"b"}),
				order:    &order,
				name:     "a",
				hostHas:  &hasKernelTools,
			}
		}, Critical: true},
		sdk.PluginSpec{New: func() sdk.Plugin {
			return &fakePlugin{
				manifest: manifest("tool.b", []string{"b"}, nil),
				order:    &order,
				name:     "b",
			}
		}, Critical: true},
	)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !hasKernelTools {
		t.Fatal("host should expose kernel.tools")
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := k.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	want := []string{"init:b", "init:a", "start:b", "start:a", "stop:a", "stop:b"}
	if len(order) != len(want) {
		t.Fatalf("unexpected order: %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (all: %v)", i, order[i], want[i], order)
		}
	}
}

func TestKernelMissingDependency(t *testing.T) {
	k := kernel.New(kernel.Options{})
	if err := k.Register(sdk.PluginSpec{New: func() sdk.Plugin {
		return &fakePlugin{
			manifest: manifest("tool.a", []string{"a"}, []string{"missing"}),
			order:    new([]string),
			name:     "a",
		}
	}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err == nil {
		t.Fatal("expected unresolved dependency error")
	}
}

func TestKernelInitPanicIsError(t *testing.T) {
	k := kernel.New(kernel.Options{})
	if err := k.Register(sdk.PluginSpec{New: func() sdk.Plugin {
		return &fakePlugin{
			manifest:  manifest("tool.a", []string{"a"}, nil),
			order:     new([]string),
			name:      "a",
			panicInit: true,
		}
	}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	err := k.Init(context.Background())
	if err == nil {
		t.Fatal("expected panic to be converted to error")
	}
	var sdkErr *sdk.Error
	if !errors.As(err, &sdkErr) || sdkErr.Code != sdk.ErrPluginPanic {
		t.Fatalf("expected plugin panic error, got %v", err)
	}
}

func TestKernelCriticalStartFailureStopsStarted(t *testing.T) {
	var order []string
	k := kernel.New(kernel.Options{})
	_ = k.Register(
		sdk.PluginSpec{New: func() sdk.Plugin {
			return &fakePlugin{manifest: manifest("tool.a", []string{"a"}, nil), order: &order, name: "a"}
		}, Critical: true},
		sdk.PluginSpec{New: func() sdk.Plugin {
			return &fakePlugin{manifest: manifest("tool.b", []string{"b"}, nil), order: &order, name: "b", startErr: errors.New("boom")}
		}, Critical: true},
	)
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(context.Background()); err == nil {
		t.Fatal("expected critical start failure")
	}
	foundStop := false
	for _, o := range order {
		if o == "stop:a" {
			foundStop = true
		}
	}
	if !foundStop {
		t.Fatalf("started plugin should be stopped on critical failure: %v", order)
	}
}

func TestKernelDuplicateCapability(t *testing.T) {
	k := kernel.New(kernel.Options{})
	_ = k.Register(
		sdk.PluginSpec{New: func() sdk.Plugin {
			return &fakePlugin{manifest: manifest("tool.a", []string{"same"}, nil), order: new([]string), name: "a"}
		}},
		sdk.PluginSpec{New: func() sdk.Plugin {
			return &fakePlugin{manifest: manifest("tool.b", []string{"same"}, nil), order: new([]string), name: "b"}
		}},
	)
	if err := k.Init(context.Background()); err == nil {
		t.Fatal("expected duplicate capability error")
	}
}
