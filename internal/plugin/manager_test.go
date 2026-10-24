package plugin_test

import (
	"context"
	"sync"
	"testing"

	"icarosu/internal/plugin"
)

type mockBot struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockBot) SendGroupMessage(_ context.Context, _ int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "group:"+message)
	return nil
}

func (m *mockBot) SendPrivateMessage(_ context.Context, _ int64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "private:"+message)
	return nil
}

func (m *mockBot) SendGroupImage(_ context.Context, _ int64, file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "group_image:"+file)
	return nil
}

func (m *mockBot) SendPrivateImage(_ context.Context, _ int64, file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "private_image:"+file)
	return nil
}

type testPlugin struct {
	plugin.BasePlugin
	name     string
	priority int
	stage    *plugin.Stage
	handle   func(e *plugin.Event) bool
}

func (p *testPlugin) Name() string  { return p.name }
func (p *testPlugin) Priority() int { return p.priority }
func (p *testPlugin) Stage() plugin.Stage {
	if p.stage != nil {
		return *p.stage
	}
	return plugin.StageResponder
}
func (p *testPlugin) Handle(_ context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if p.handle != nil {
		return p.handle(e)
	}
	return false
}

func TestManagerDispatchStopsAfterHandled(t *testing.T) {
	m := plugin.NewManager()
	var order []string
	m.Register(&testPlugin{
		name:     "low",
		priority: 0,
		handle: func(e *plugin.Event) bool {
			order = append(order, "low")
			return false
		},
	})
	m.Register(&testPlugin{
		name:     "high",
		priority: 10,
		handle: func(e *plugin.Event) bool {
			order = append(order, "high")
			return true
		},
	})

	m.Dispatch(context.Background(), &mockBot{}, &plugin.Event{Type: plugin.EventGroupMessage})

	if len(order) != 1 || order[0] != "high" {
		t.Fatalf("expected only high to run, got %v", order)
	}
}

func TestManagerDispatchRunsObserversUntilHandled(t *testing.T) {
	m := plugin.NewManager()
	var order []string
	m.Register(&testPlugin{
		name:     "handler",
		priority: 5,
		handle: func(e *plugin.Event) bool {
			order = append(order, "handler")
			return true
		},
	})
	m.Register(&testPlugin{
		name:     "observer",
		priority: 10,
		handle: func(e *plugin.Event) bool {
			order = append(order, "observer")
			return false
		},
	})

	m.Dispatch(context.Background(), &mockBot{}, &plugin.Event{})

	if len(order) != 2 || order[0] != "observer" || order[1] != "handler" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestManagerDispatchRunsStagesInOrder(t *testing.T) {
	m := plugin.NewManager()
	var order []string
	pre := plugin.StagePreprocessor
	post := plugin.StagePostprocessor

	m.Register(&testPlugin{
		name:     "post",
		priority: 100,
		stage:    &post,
		handle: func(e *plugin.Event) bool {
			order = append(order, "post")
			return false
		},
	})
	m.Register(&testPlugin{
		name:     "responder",
		priority: 10,
		handle: func(e *plugin.Event) bool {
			order = append(order, "responder")
			return false
		},
	})
	m.Register(&testPlugin{
		name:     "pre",
		priority: 1000,
		stage:    &pre,
		handle: func(e *plugin.Event) bool {
			order = append(order, "pre")
			return false
		},
	})

	m.Dispatch(context.Background(), &mockBot{}, &plugin.Event{Type: plugin.EventGroupMessage})

	if len(order) != 3 || order[0] != "pre" || order[1] != "responder" || order[2] != "post" {
		t.Fatalf("unexpected stage order: %v", order)
	}
}

func TestManagerSetEnabledSkipsDisabled(t *testing.T) {
	m := plugin.NewManager()
	var order []string
	m.Register(&testPlugin{
		name:     "high",
		priority: 10,
		handle: func(e *plugin.Event) bool {
			order = append(order, "high")
			return true
		},
	})
	m.Register(&testPlugin{
		name:     "low",
		priority: 0,
		handle: func(e *plugin.Event) bool {
			order = append(order, "low")
			return true
		},
	})

	m.SetEnabled("high", false)
	m.Dispatch(context.Background(), &mockBot{}, &plugin.Event{Type: plugin.EventGroupMessage})

	if len(order) != 1 || order[0] != "low" {
		t.Fatalf("expected only low to run, got %v", order)
	}
}

func TestManagerDispatchPreprocessorCanStopPipeline(t *testing.T) {
	m := plugin.NewManager()
	var order []string
	pre := plugin.StagePreprocessor

	m.Register(&testPlugin{
		name:     "responder",
		priority: 10,
		handle: func(e *plugin.Event) bool {
			order = append(order, "responder")
			return false
		},
	})
	m.Register(&testPlugin{
		name:     "pre",
		priority: 1000,
		stage:    &pre,
		handle: func(e *plugin.Event) bool {
			order = append(order, "pre")
			return true
		},
	})

	m.Dispatch(context.Background(), &mockBot{}, &plugin.Event{Type: plugin.EventGroupMessage})

	if len(order) != 1 || order[0] != "pre" {
		t.Fatalf("expected preprocessor to stop pipeline, got %v", order)
	}
}

type contextTestPlugin struct {
	plugin.BasePlugin
	name          string
	stage         plugin.Stage
	handleContext func(*plugin.Context) bool
}

func (p *contextTestPlugin) Name() string { return p.name }

func (p *contextTestPlugin) Stage() plugin.Stage { return p.stage }

func (p *contextTestPlugin) HandleContext(_ context.Context, _ plugin.Bot, pc *plugin.Context) bool {
	if p.handleContext != nil {
		return p.handleContext(pc)
	}
	return false
}

func TestManagerDispatchRunsPostprocessorAfterResponderHandled(t *testing.T) {
	m := plugin.NewManager()
	var order []string
	responder := plugin.StageResponder
	post := plugin.StagePostprocessor

	m.Register(&contextTestPlugin{
		name:  "post",
		stage: post,
		handleContext: func(pc *plugin.Context) bool {
			order = append(order, "post")
			if pc.Get("reply") != "hi" {
				t.Fatalf("expected reply intermediate data, got %v", pc.Get("reply"))
			}
			return false
		},
	})
	m.Register(&contextTestPlugin{
		name:  "responder",
		stage: responder,
		handleContext: func(pc *plugin.Context) bool {
			order = append(order, "responder")
			pc.Set("reply", "hi")
			return true
		},
	})

	m.Dispatch(context.Background(), &mockBot{}, &plugin.Event{})

	if len(order) != 2 || order[0] != "responder" || order[1] != "post" {
		t.Fatalf("expected responder then postprocessor, got %v", order)
	}
}

func TestRegistryRejectsDuplicateNames(t *testing.T) {
	r := &plugin.Registry{}
	r.Register(&testPlugin{name: "x"})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate plugin name")
		}
	}()
	r.Register(&testPlugin{name: "x"})
}
