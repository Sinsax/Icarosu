package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
)

func TestManagerReloadRejectsInvalidConfigAndKeepsOld(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".config-manager-invalid-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "config.yaml")
	valid := `version: 12
llm_config_file: ""
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
bot:
  self_id: 1
  name: "old"
`
	if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	invalid := strings.Replace(valid, "self_id: 1", "self_id: 0", 1)
	if err := os.WriteFile(path, []byte(invalid), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	if err := m.Reload(); err == nil {
		t.Fatal("expected reload to reject invalid config")
	}
	if m.Get().Bot.SelfID != 1 || m.Get().Bot.Name != "old" {
		t.Fatalf("invalid reload should keep old config, got %+v", m.Get().Bot)
	}
}

func TestManagerReloadListenerPanicKeepsOldConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "version: 12\nllm_config_file: \"\"\nqq:\n  onebot:\n    ws_url: \"ws://127.0.0.1:6700\"\nbot:\n  self_id: 1\n  name: \"old\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	m.OnReload(func(cfg *config.Config) { panic("listener boom") })
	updated := strings.Replace(content, "old", "new", 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("update config: %v", err)
	}
	if err := m.Reload(); err == nil {
		t.Fatal("expected listener panic error")
	}
	if got := m.Get().Bot.Name; got != "old" {
		t.Fatalf("listener failure should keep old config, got %q", got)
	}
}

func TestManagerReload(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".config-manager-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "config.yaml")
	content := `version: 12
llm_config_file: ""
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
bot:
  self_id: 1
  name: "old"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	m, err := config.NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.Get().Bot.Name != "old" {
		t.Fatalf("expected old name, got %q", m.Get().Bot.Name)
	}

	updated := strings.Replace(content, `name: "old"`, `name: "new"`, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	var notified int
	m.OnReload(func(cfg *config.Config) {
		notified++
		if cfg.Bot.Name != "new" {
			t.Fatalf("reload callback got %q", cfg.Bot.Name)
		}
	})

	if err := m.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if m.Get().Bot.Name != "new" {
		t.Fatalf("expected new name after reload, got %q", m.Get().Bot.Name)
	}
	if notified != 1 {
		t.Fatalf("expected 1 reload notification, got %d", notified)
	}
}
