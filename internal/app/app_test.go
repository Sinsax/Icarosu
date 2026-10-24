package app

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"icarosu/internal/platform"
	"icarosu/internal/qq"
)

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 12
llm_config_file: ""
bot:
  self_id: 10001
  name: "测试机器人"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:1"
store:
  driver: "sqlite"
  dsn: "` + filepath.ToSlash(filepath.Join(dir, "bot.db")) + `"
memory:
  short_term_size: 20
  long_term_enabled: true
  context_max_age_minutes: 30
  summary_threshold: 50
  summary_interval_minutes: 60
vlm:
  base_url: ""
  model: ""
webui:
  enabled: false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestAppNewAndClose(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".app-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := writeTestConfig(t, dir)
	a, err := New(path)
	if err != nil {
		t.Fatalf("New app: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil app")
	}
	if a.manager == nil {
		t.Fatal("expected plugin manager to be initialized")
	}
	if a.chatPlugin == nil {
		t.Fatal("expected chat plugin to be initialized")
	}
	if a.st == nil {
		t.Fatal("expected store to be opened when long_term_enabled")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("close app: %v", err)
	}
}

type fakeAdapter struct {
	started atomic.Bool
}

func (f *fakeAdapter) SetMessageHandler(qq.MessageHandler) {}

func (f *fakeAdapter) Connect(ctx context.Context) error {
	f.started.Store(true)
	return nil
}

func (f *fakeAdapter) Close() error { return nil }

func (f *fakeAdapter) SendGroupMessage(context.Context, int64, string) error  { return nil }
func (f *fakeAdapter) SendPrivateMessage(context.Context, int64, string) error { return nil }
func (f *fakeAdapter) SendGroupImage(context.Context, int64, string) error     { return nil }
func (f *fakeAdapter) SendPrivateImage(context.Context, int64, string) error   { return nil }

var _ platform.Adapter = (*fakeAdapter)(nil)

func TestRunWithFakeAdapter(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".app-run-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := writeTestConfig(t, dir)
	adapter := &fakeAdapter{}
	a, err := NewWithPlatform(path, adapter)
	if err != nil {
		t.Fatalf("NewWithPlatform: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- a.Run(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for !adapter.started.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !adapter.started.Load() {
		cancel()
		t.Fatal("fake adapter was not started")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}