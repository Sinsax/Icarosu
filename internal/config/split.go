package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	oldLLM, oldLLMErr := os.ReadFile(llmPath)

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

	if err := atomicWriteFile(mainPath, splitData, 0o600); err != nil {
		// provider 已更新，主配置失败时尽量回滚 provider，避免两文件不一致。
		if oldLLMErr == nil {
			_ = atomicWriteFile(llmPath, oldLLM, 0o600)
		} else {
			_ = os.Remove(llmPath)
		}
		return fmt.Errorf("write main config: %w", err)
	}
	return nil
}

// RestoreSecretsYAML 用旧配置恢复被 WebUI 脱敏的 YAML 字段，
// 同时保留用户提交的注释/结构。
func RestoreSecretsYAML(data []byte, old *Config) ([]byte, error) {
	if old == nil {
		return data, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return data, nil
	}
	root := doc.Content[0]
	setSecret(root, []string{"llm", "api_key"}, old.LLM.APIKey)
	setSecret(root, []string{"vlm", "api_key"}, old.VLM.APIKey)
	setSecret(root, []string{"qq", "onebot", "access_token"}, old.QQ.OneBot.AccessToken)
	setSecret(root, []string{"webui", "token"}, old.WebUI.Token)
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("marshal config yaml: %w", err)
	}
	return out, nil
}

func setSecret(node *yaml.Node, path []string, value string) {
	if value == "" {
		// 旧密钥为空时保留用户本次提交的新值，不能反清空。
		return
	}
	for i, key := range path {
		child := mappingValue(node, key)
		if child == nil {
			// 只补当前父 mapping 中缺失的叶子字段；不凭空创建
			// llm/vlm 等可能属于拆分文件的整段，避免把 main 配置重新混入。
			if i != len(path)-1 || node.Kind != yaml.MappingNode {
				return
			}
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: value},
			)
			return
		}
		if i == len(path)-1 {
			child.Value = value
			child.Tag = "!!str"
			child.Style = yaml.DoubleQuotedStyle
			return
		}
		node = child
	}
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// WriteRawFile 原子写入原始配置文本。
func WriteRawFile(path string, data []byte) error {
	return atomicWriteFile(path, data, 0o600)
}

// ProviderPath 返回 llm 配置文件的实际路径。
func ProviderPath(mainPath, llmConfigFile string) string {
	return resolveProviderWritePath(mainPath, llmConfigFile)
}

// ValidateProviderPath 限制 llm_config_file 只能指向主配置目录内的相对路径，
// 防止 WebUI 借绝对路径写任意 YAML 文件。
func ValidateProviderPath(mainPath, llmConfigFile string) error {
	if llmConfigFile == "" {
		return nil
	}
	if filepath.IsAbs(llmConfigFile) {
		return fmt.Errorf("llm_config_file must be relative to the main config directory")
	}
	base := filepath.Dir(mainPath)
	target := filepath.Clean(filepath.Join(base, llmConfigFile))
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return fmt.Errorf("resolve llm_config_file: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("llm_config_file escapes the config directory")
	}
	return nil
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

	// providers 由用户维护；llm/vlm 段是当前生效值。密钥轮换时
	// applyProviderFile 会以 providers.<name>.api_key 优先，不会被旧值遮蔽。
	existing["llm"] = cfg.LLM
	existing["vlm"] = cfg.VLM

	data, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal llm config: %w", err)
	}
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write llm config %q: %w", path, err)
	}
	return nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
