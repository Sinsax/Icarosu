package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
)

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
