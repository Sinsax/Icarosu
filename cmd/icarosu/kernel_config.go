package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// KernelFile 是新的 kernel.yaml：只放内核/装载信息，插件业务配置在 plugins/<id>.yaml。
type KernelFile struct {
	Kernel struct {
		Config           string `yaml:"config"`
		PluginsDir       string `yaml:"plugins_dir"`
		SchedulerWorkers int    `yaml:"scheduler_workers"`
		SchedulerBuffer  int    `yaml:"scheduler_buffer"`
	} `yaml:"kernel"`
	Plugins struct {
		Enabled []string `yaml:"enabled"`
	} `yaml:"plugins"`
}

func loadKernelFile(path string) (KernelFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return KernelFile{}, err
	}
	var kf KernelFile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		return KernelFile{}, fmt.Errorf("parse kernel config: %w", err)
	}
	return kf, nil
}

func (kf KernelFile) enabledSet() map[string]bool {
	if len(kf.Plugins.Enabled) == 0 {
		return nil
	}
	out := make(map[string]bool, len(kf.Plugins.Enabled))
	for _, id := range kf.Plugins.Enabled {
		out[id] = true
	}
	return out
}

func resolvePath(base, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
