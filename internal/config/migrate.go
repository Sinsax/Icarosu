package config

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

func migrateNode(root *yaml.Node, from int) error {
	switch from {
	case 1:
		if err := ensureLLMDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 2:
		if err := ensureBotDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 3:
		fallthrough
	case 4:
		if err := ensureFilterDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 5:
		if err := ensureMemoryDefaults(root); err != nil {
			return err
		}
		if err := ensureStoreDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 6:
		if err := mergeTriggerIntoBot(root); err != nil {
			return err
		}
		if err := ensureLLMDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 7:
		if err := ensureVLMDefaults(root); err != nil {
			return err
		}
		if err := ensureMemeDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 8:
		removeVLMAndMemeEnabled(root)
		fallthrough
	case 9:
		if err := ensureLLMDefaults(root); err != nil {
			return err
		}
		if err := ensureVLMDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 10:
		if err := ensureAttentionDefaults(root); err != nil {
			return err
		}
		fallthrough
	case 11:
		if err := ensureAttentionDefaults(root); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported config version %d", from)
	}

	setVersion(root, CurrentVersion)
	return nil
}

func ensureLLMDefaults(root *yaml.Node) error {
	llmNode := findMappingValue(root, "llm")
	if llmNode == nil {
		llmNode = defaultLLMNode()
		setMappingValue(root, "llm", llmNode)
		return nil
	}
	if llmNode.Kind != yaml.MappingNode {
		return fmt.Errorf("config field llm must be a mapping")
	}

	for _, item := range defaultLLMDefaults() {
		if findMappingValue(llmNode, item.key) == nil {
			setMappingValue(llmNode, item.key, item.value)
		}
	}
	return nil
}

// ensureBotDefaults 创建 bot 配置段，并把旧位置的 self_id / system_prompt 迁移过来。
func ensureBotDefaults(root *yaml.Node) error {
	botNode := findMappingValue(root, "bot")
	created := botNode == nil
	if botNode == nil {
		botNode = defaultBotNode()
		setMappingValue(root, "bot", botNode)
	} else if botNode.Kind != yaml.MappingNode {
		return fmt.Errorf("config field bot must be a mapping")
	}

	// 从 qq.onebot.self_id 迁移到 bot.self_id
	qqNode := findMappingValue(root, "qq")
	onebotNode := findMappingValue(qqNode, "onebot")
	if selfIDNode := findMappingValue(onebotNode, "self_id"); selfIDNode != nil {
		if created || findMappingValue(botNode, "self_id") == nil {
			setMappingValue(botNode, "self_id", selfIDNode)
		}
		removeMappingKey(onebotNode, "self_id")
	}

	// 从 llm.system_prompt 迁移到 bot.system_prompt
	llmNode := findMappingValue(root, "llm")
	if sysPromptNode := findMappingValue(llmNode, "system_prompt"); sysPromptNode != nil {
		if created || findMappingValue(botNode, "system_prompt") == nil {
			setMappingValue(botNode, "system_prompt", sysPromptNode)
		}
		removeMappingKey(llmNode, "system_prompt")
	}

	// 补齐缺失的 bot 默认字段
	for _, item := range defaultBotDefaults() {
		if findMappingValue(botNode, item.key) == nil {
			setMappingValue(botNode, item.key, item.value)
		}
	}
	return nil
}

// mergeTriggerIntoBot 把旧版 trigger 配置段合并进 bot，并且始终开启（不保留 enabled 开关）。
func mergeTriggerIntoBot(root *yaml.Node) error {
	botNode := findMappingValue(root, "bot")
	if botNode == nil {
		botNode = defaultBotNode()
		setMappingValue(root, "bot", botNode)
	} else if botNode.Kind != yaml.MappingNode {
		return fmt.Errorf("config field bot must be a mapping")
	}

	triggerNode := findMappingValue(root, "trigger")
	if triggerNode != nil {
		for _, key := range []string{"mention_reply", "keywords", "random_probability", "interest_reply", "cooldown_seconds"} {
			if v := findMappingValue(triggerNode, key); v != nil && findMappingValue(botNode, key) == nil {
				setMappingValue(botNode, key, v)
			}
		}
		removeMappingKey(root, "trigger")
	}

	// 补齐 bot 下缺失的触发字段默认值
	for _, item := range defaultBotDefaults() {
		if findMappingValue(botNode, item.key) == nil {
			setMappingValue(botNode, item.key, item.value)
		}
	}
	return nil
}

// ensureFilterDefaults 创建 filter 配置段并补齐默认字段。
func ensureFilterDefaults(root *yaml.Node) error {
	filterNode := findMappingValue(root, "filter")
	if filterNode == nil {
		filterNode = defaultFilterNode()
		setMappingValue(root, "filter", filterNode)
		return nil
	}
	if filterNode.Kind != yaml.MappingNode {
		return fmt.Errorf("config field filter must be a mapping")
	}

	for _, item := range defaultFilterDefaults() {
		if findMappingValue(filterNode, item.key) == nil {
			setMappingValue(filterNode, item.key, item.value)
		}
	}
	return nil
}

func ensureMemoryDefaults(root *yaml.Node) error {
	return ensureMappingDefaults(root, "memory", defaultMemoryNode, defaultMemoryDefaults)
}

func ensureStoreDefaults(root *yaml.Node) error {
	return ensureMappingDefaults(root, "store", defaultStoreNode, defaultStoreDefaults)
}

// removeVLMAndMemeEnabled 移除旧版 vlm/meme 里的 enabled 开关。
func removeVLMAndMemeEnabled(root *yaml.Node) {
	if node := findMappingValue(root, "vlm"); node != nil {
		removeMappingKey(node, "enabled")
	}
	if node := findMappingValue(root, "meme"); node != nil {
		removeMappingKey(node, "enabled")
	}
}

func ensureVLMDefaults(root *yaml.Node) error {
	return ensureMappingDefaults(root, "vlm", defaultVLMNode, defaultVLMDefaults)
}

func ensureMemeDefaults(root *yaml.Node) error {
	return ensureMappingDefaults(root, "meme", defaultMemeNode, defaultMemeDefaults)
}

func ensureAttentionDefaults(root *yaml.Node) error {
	return ensureMappingDefaults(root, "attention", defaultAttentionNode, defaultAttentionDefaults)
}

func ensureMappingDefaults(root *yaml.Node, name string, makeNode func() *yaml.Node, makeDefaults func() []namedNode) error {
	node := findMappingValue(root, name)
	if node == nil {
		setMappingValue(root, name, makeNode())
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("config field %s must be a mapping", name)
	}

	for _, item := range makeDefaults() {
		if findMappingValue(node, item.key) == nil {
			setMappingValue(node, item.key, item.value)
		}
	}
	return nil
}

type namedNode struct {
	key   string
	value *yaml.Node
}

func setVersion(root *yaml.Node, version int) {
	setMappingValue(root, "version", &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: strconv.Itoa(version),
	})
}

func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}

	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	mapping.Content = append(mapping.Content, keyNode, value)
}

func removeMappingKey(mapping *yaml.Node, key string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// canonicalize 按预设顺序重排配置文件的各段和字段。
