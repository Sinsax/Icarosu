package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WriteSplit 把配置拆成两个文件写入：
//   - LLM/VLM 相关配置始终写入独立的 llm.yaml（保留已有 providers）
//   - 主配置只保留非 LLM 配置，不再包含 llm/vlm 段
func WriteSplit(cfg *Config, mainPath string) error {
	if cfg.LLMConfigFile == "" {
		cfg.LLMConfigFile = "llm.yaml"
	}
	llmPath := resolveProviderWritePath(mainPath, cfg.LLMConfigFile)

	if err := writeProviderFile(llmPath, cfg); err != nil {
		return err
	}

	mainData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal main config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(mainData, &doc); err != nil {
		return fmt.Errorf("parse main config for split: %w", err)
	}
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		removeMappingKey(root, "llm")
		removeMappingKey(root, "vlm")
	}

	splitData, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal split main config: %w", err)
	}

	if err := os.WriteFile(mainPath, splitData, 0o644); err != nil {
		return fmt.Errorf("write main config: %w", err)
	}
	return nil
}

// ProviderPath 返回 llm 配置文件的实际路径。
func ProviderPath(mainPath, llmConfigFile string) string {
	return resolveProviderWritePath(mainPath, llmConfigFile)
}

func resolveProviderWritePath(mainPath, llmConfigFile string) string {
	if filepath.IsAbs(llmConfigFile) {
		return llmConfigFile
	}
	if _, err := os.Stat(llmConfigFile); err == nil {
		return llmConfigFile
	}
	return filepath.Join(filepath.Dir(mainPath), llmConfigFile)
}

func writeProviderFile(path string, cfg *Config) error {
	existing := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = map[string]any{}
	}
	existing["llm"] = cfg.LLM
	existing["vlm"] = cfg.VLM

	data, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal llm config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write llm config %q: %w", path, err)
	}
	return nil
}
