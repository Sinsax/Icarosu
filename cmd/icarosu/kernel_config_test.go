package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKernelFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kernel.yaml")
	content := `kernel:
  config: config.yaml
  plugins_dir: plugins
  scheduler_workers: 2
  scheduler_buffer: 64
plugins:
  enabled:
    - adapter.onebot
    - persona.default
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	kf, err := loadKernelFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if kf.Kernel.Config != "config.yaml" || kf.Kernel.PluginsDir != "plugins" {
		t.Fatalf("unexpected kernel config: %+v", kf.Kernel)
	}
	if kf.Kernel.SchedulerWorkers != 2 || kf.Kernel.SchedulerBuffer != 64 {
		t.Fatalf("unexpected scheduler config: %+v", kf.Kernel)
	}
	set := kf.enabledSet()
	if !set["adapter.onebot"] || !set["persona.default"] {
		t.Fatalf("unexpected enabled set: %+v", set)
	}
	if got := resolvePath(dir, "config.yaml"); got != filepath.Join(dir, "config.yaml") {
		t.Fatalf("unexpected resolved path: %s", got)
	}
}
