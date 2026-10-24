package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type providerPreset struct {
	BaseURL string
	Model   string
}

var llmProviderPresets = map[string]providerPreset{
	"deepseek": {BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"},
	"openai":   {BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
	"qwen":     {BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-plus"},
	"zhipu":    {BaseURL: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4-flash"},
	"ollama":   {BaseURL: "http://127.0.0.1:11434/v1", Model: "llama3.1"},
}

var vlmProviderPresets = map[string]providerPreset{
	"deepseek": {BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-vl2"},
	"openai":   {BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
	"qwen":     {BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-vl-plus"},
	"zhipu":    {BaseURL: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4v-flash"},
	"ollama":   {BaseURL: "http://127.0.0.1:11434/v1", Model: "llava"},
}

type ProviderModel struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"`
}

type ProviderConfig struct {
	BaseURL string          `yaml:"base_url"`
	APIKey  string          `yaml:"api_key"`
	Models  []ProviderModel `yaml:"models"`
}

type providerFileConfig struct {
	Providers map[string]ProviderConfig `yaml:"providers"`
	LLM       LLMConfig                 `yaml:"llm"`
	VLM       VLMConfig                 `yaml:"vlm"`
}

func loadProviderFile(path string) (*providerFileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load provider config %q: %w", path, err)
	}
	var p providerFileConfig
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse provider config %q: %w", path, err)
	}
	return &p, nil
}

// applyProviderFile 根据独立配置文件中的 providers 列表，解析 llm/vlm 的 base_url/api_key 并校验模型。
func applyProviderFile(cfg *Config, p *providerFileConfig) error {
	if p == nil {
		return nil
	}

	if p.LLM.Provider != "" {
		providerName := strings.ToLower(p.LLM.Provider)
		provider, ok := p.Providers[providerName]
		if ok {
			if cfg.LLM.BaseURL == "" {
				cfg.LLM.BaseURL = provider.BaseURL
			}
			if cfg.LLM.APIKey == "" {
				cfg.LLM.APIKey = provider.APIKey
			}
			if cfg.LLM.Model != "" && !providerHasModel(provider, cfg.LLM.Model, "llm") {
				return fmt.Errorf("provider %q does not list llm model %q", providerName, cfg.LLM.Model)
			}
		}
	}

	if p.VLM.Provider != "" {
		providerName := strings.ToLower(p.VLM.Provider)
		provider, ok := p.Providers[providerName]
		if ok {
			if cfg.VLM.BaseURL == "" {
				cfg.VLM.BaseURL = provider.BaseURL
			}
			if cfg.VLM.APIKey == "" {
				cfg.VLM.APIKey = provider.APIKey
			}
			if cfg.VLM.Model != "" && !providerHasModel(provider, cfg.VLM.Model, "vlm") {
				return fmt.Errorf("provider %q does not list vlm model %q", providerName, cfg.VLM.Model)
			}
		}
	}

	return nil
}

func providerHasModel(p ProviderConfig, model, kind string) bool {
	if len(p.Models) == 0 {
		return true
	}
	for _, m := range p.Models {
		if m.Name == model && (m.Kind == "" || m.Kind == kind) {
			return true
		}
	}
	return false
}

func applyProviderDefaults(cfg *Config) {
	if cfg.LLM.Provider != "" {
		if preset, ok := llmProviderPresets[strings.ToLower(cfg.LLM.Provider)]; ok {
			if cfg.LLM.BaseURL == "" {
				cfg.LLM.BaseURL = preset.BaseURL
			}
			if cfg.LLM.Model == "" {
				cfg.LLM.Model = preset.Model
			}
		}
	}
	if cfg.VLM.Provider != "" {
		if preset, ok := vlmProviderPresets[strings.ToLower(cfg.VLM.Provider)]; ok {
			if cfg.VLM.BaseURL == "" {
				cfg.VLM.BaseURL = preset.BaseURL
			}
			if cfg.VLM.Model == "" {
				cfg.VLM.Model = preset.Model
			}
		}
	}
}

// Load 读取配置文件。
