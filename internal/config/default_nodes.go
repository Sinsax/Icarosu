package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)
func defaultLLMNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultLLMDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

type llmDefaultItem struct {
	key   string
	value *yaml.Node
}

func defaultLLMDefaults() []llmDefaultItem {
	def := Default().LLM
	return []llmDefaultItem{
		{key: "provider", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Provider}},
		{key: "base_url", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.BaseURL}},
		{key: "api_key", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.APIKey}},
		{key: "model", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Model}},
		{key: "temperature", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.Temperature, 'f', -1, 64)}},
		{key: "max_tokens", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.MaxTokens)}},
		{key: "timeout_seconds", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.TimeoutSeconds)}},
		{key: "cache_prefix", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.CachePrefix)}},
		{key: "cache_max_tokens", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.CacheMaxTokens)}},
		{key: "log_cache", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.LogCache)}},
	}
}

func defaultBotNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultBotDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

type botDefaultItem struct {
	key   string
	value *yaml.Node
}

func defaultBotDefaults() []botDefaultItem {
	def := Default().Bot
	return []botDefaultItem{
		{key: "self_id", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(def.SelfID, 10)}},
		{key: "admin_ids", value: int64SliceNode(def.AdminIDs)},
		{key: "name", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Name}},
		{key: "nicknames", value: stringSliceNode(def.Nicknames)},
		{key: "personality", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Personality}},
		{key: "speaking_style", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.SpeakingStyle}},
		{key: "background", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Background}},
		{key: "interests", value: stringSliceNode(def.Interests)},
		{key: "mention_reply", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.MentionReply)}},
		{key: "keywords", value: stringSliceNode(def.Keywords)},
		{key: "random_probability", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.RandomProbability, 'f', -1, 64)}},
		{key: "interest_reply", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.InterestReply)}},
		{key: "cooldown_seconds", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.CooldownSeconds)}},
		{key: "group_rules", value: stringSliceNode(def.GroupRules)},
		{key: "forbidden", value: stringSliceNode(def.Forbidden)},
		{key: "system_prompt", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.SystemPrompt}},
	}
}

func defaultFilterNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultFilterDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

type filterDefaultItem struct {
	key   string
	value *yaml.Node
}

func defaultFilterDefaults() []filterDefaultItem {
	def := Default().Filter
	return []filterDefaultItem{
		{key: "enabled", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.Enabled)}},
		{key: "mode", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Mode}},
		{key: "group_ids", value: int64SliceNode(def.GroupIDs)},
		{key: "user_ids", value: int64SliceNode(def.UserIDs)},
	}
}

func defaultMemoryNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultMemoryDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

func defaultMemoryDefaults() []namedNode {
	def := Default().Memory
	return []namedNode{
		{key: "short_term_size", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.ShortTermSize)}},
		{key: "long_term_enabled", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.LongTermEnabled)}},
		{key: "context_max_age_minutes", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.ContextMaxAgeMinutes)}},
		{key: "summary_threshold", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.SummaryThreshold)}},
		{key: "summary_interval_minutes", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.SummaryIntervalMinutes)}},
	}
}

func defaultStoreNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultStoreDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

func defaultStoreDefaults() []namedNode {
	def := Default().Store
	return []namedNode{
		{key: "driver", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Driver}},
		{key: "dsn", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.DSN}},
	}
}

func defaultVLMNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultVLMDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

func defaultVLMDefaults() []namedNode {
	def := Default().VLM
	return []namedNode{
		{key: "provider", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Provider}},
		{key: "base_url", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.BaseURL}},
		{key: "api_key", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.APIKey}},
		{key: "model", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Model}},
		{key: "timeout_seconds", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.TimeoutSeconds)}},
		{key: "prompt", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Prompt}},
	}
}

func defaultMemeNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultMemeDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

func defaultMemeDefaults() []namedNode {
	def := Default().Meme
	return []namedNode{
		{key: "directory", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: def.Directory}},
		{key: "steal_probability", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.StealProbability, 'f', -1, 64)}},
	}
}

func defaultAttentionNode() *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, item := range defaultAttentionDefaults() {
		setMappingValue(node, item.key, item.value)
	}
	return node
}

func defaultAttentionDefaults() []namedNode {
	def := Default().Attention
	return []namedNode{
		{key: "enabled", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(def.Enabled)}},
		{key: "max_score", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.MaxScore, 'f', -1, 64)}},
		{key: "decay_per_minute", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.DecayPerMinute, 'f', -1, 64)}},
		{key: "boost_at_bot", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.BoostAtBot, 'f', -1, 64)}},
		{key: "boost_mention", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.BoostMention, 'f', -1, 64)}},
		{key: "boost_interest", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.BoostInterest, 'f', -1, 64)}},
		{key: "boost_bot_replied", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: strconv.FormatFloat(def.BoostBotReplied, 'f', -1, 64)}},
		{key: "trip_hold_seconds", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(def.TripHoldSeconds)}},
	}
}

func int64SliceNode(values []int64) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}
	for _, v := range values {
		node.Content = append(node.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!int",
			Value: strconv.FormatInt(v, 10),
		})
	}
	return node
}

func stringSliceNode(values []string) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}
	for _, v := range values {
		node.Content = append(node.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v,
		})
	}
	return node
}

func writeWithBackup(path string, original, migrated []byte) error {
	backup := path + ".bak"
	if err := os.WriteFile(backup, original, 0o644); err != nil {
		return fmt.Errorf("write config backup %q: %w", backup, err)
	}
	if err := os.WriteFile(path, migrated, 0o644); err != nil {
		return fmt.Errorf("write migrated config %q: %w", path, err)
	}
	return nil
}

