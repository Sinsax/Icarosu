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
	handle   func(e *plugin.Event) bool
}

func (p *testPlugin) Name() string  { return p.name }
func (p *testPlugin) Priority() int { return p.priority }
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
