package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file %q not found: %w", path, err)
		}
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("config file %q is empty", path)
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config file %q root must be a mapping", path)
	}

	version, err := nodeVersion(root)
	if err != nil {
		return nil, err
	}
	if version > CurrentVersion {
		return nil, fmt.Errorf("config version %d is newer than supported version %d", version, CurrentVersion)
	}

	if version < CurrentVersion {
		if err := migrateNode(root, version); err != nil {
			return nil, err
		}
		canonicalize(root)

		migrated, err := yaml.Marshal(&doc)
		if err != nil {
			return nil, fmt.Errorf("marshal migrated config %q: %w", path, err)
		}
		if err := writeWithBackup(path, data, migrated); err != nil {
			return nil, err
		}
		data = migrated
	} else if !isCanonical(root) {
		canonicalize(root)

		normalized, err := yaml.Marshal(&doc)
		if err != nil {
			return nil, fmt.Errorf("marshal normalized config %q: %w", path, err)
		}
		if err := writeWithBackup(path, data, normalized); err != nil {
			return nil, err
		}
		data = normalized
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse migrated config %q: %w", path, err)
	}
	if findMappingValue(root, "llm_config_file") == nil {
		cfg.LLMConfigFile = Default().LLMConfigFile
	}

	if cfg.LLMConfigFile != "" {
		providerPath := cfg.LLMConfigFile
		if !filepath.IsAbs(providerPath) {
			if _, err := os.Stat(providerPath); err != nil {
				providerPath = filepath.Join(filepath.Dir(path), providerPath)
			}
		}
		providerCfg, err := loadProviderFile(providerPath)
		if err != nil {
			return nil, err
		}
		if providerCfg.LLM.Provider != "" || providerCfg.LLM.BaseURL != "" || providerCfg.LLM.Model != "" || providerCfg.LLM.APIKey != "" {
			cfg.LLM = providerCfg.LLM
		}
		if providerCfg.VLM.Provider != "" || providerCfg.VLM.BaseURL != "" || providerCfg.VLM.Model != "" || providerCfg.VLM.APIKey != "" {
			cfg.VLM = providerCfg.VLM
		}
		if err := applyProviderFile(cfg, providerCfg); err != nil {
			return nil, err
		}
	}

	applyProviderDefaults(cfg)
	applyDefaults(cfg, root)

	if cfg.QQ.OneBot.WSURL == "" {
		return nil, fmt.Errorf("qq.onebot.ws_url must not be empty")
	}

	return cfg, nil
}

// applyDefaults 只补充配置文件中“缺失”的字段，避免覆盖用户显式设置的零值。
func applyDefaults(cfg *Config, root *yaml.Node) {
	def := Default()

	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}
	if cfg.LLMConfigFile == "" && findMappingValue(root, "llm_config_file") == nil {
		cfg.LLMConfigFile = def.LLMConfigFile
	}
	if !cfg.DryRun && findMappingValue(root, "dry_run") == nil {
		cfg.DryRun = def.DryRun
	}

	botNode := findMappingValue(root, "bot")
	if cfg.Bot.SelfID == 0 && (botNode == nil || findMappingValue(botNode, "self_id") == nil) {
		cfg.Bot.SelfID = def.Bot.SelfID
	}
	if len(cfg.Bot.AdminIDs) == 0 && (botNode == nil || findMappingValue(botNode, "admin_ids") == nil) {
		cfg.Bot.AdminIDs = append([]int64(nil), def.Bot.AdminIDs...)
	}
	if cfg.Bot.Name == "" && (botNode == nil || findMappingValue(botNode, "name") == nil) {
		cfg.Bot.Name = def.Bot.Name
	}
	if len(cfg.Bot.Nicknames) == 0 && (botNode == nil || findMappingValue(botNode, "nicknames") == nil) {
		cfg.Bot.Nicknames = append([]string(nil), def.Bot.Nicknames...)
	}
	if cfg.Bot.Personality == "" && (botNode == nil || findMappingValue(botNode, "personality") == nil) {
		cfg.Bot.Personality = def.Bot.Personality
	}
	if cfg.Bot.SpeakingStyle == "" && (botNode == nil || findMappingValue(botNode, "speaking_style") == nil) {
		cfg.Bot.SpeakingStyle = def.Bot.SpeakingStyle
	}
	if cfg.Bot.Background == "" && (botNode == nil || findMappingValue(botNode, "background") == nil) {
		cfg.Bot.Background = def.Bot.Background
	}
	if len(cfg.Bot.Interests) == 0 && (botNode == nil || findMappingValue(botNode, "interests") == nil) {
		cfg.Bot.Interests = append([]string(nil), def.Bot.Interests...)
	}
	if !cfg.Bot.MentionReply && (botNode == nil || findMappingValue(botNode, "mention_reply") == nil) {
		cfg.Bot.MentionReply = def.Bot.MentionReply
	}
	if len(cfg.Bot.Keywords) == 0 && (botNode == nil || findMappingValue(botNode, "keywords") == nil) {
		cfg.Bot.Keywords = append([]string(nil), def.Bot.Keywords...)
	}
	if cfg.Bot.RandomProbability == 0 && (botNode == nil || findMappingValue(botNode, "random_probability") == nil) {
		cfg.Bot.RandomProbability = def.Bot.RandomProbability
	}
	if !cfg.Bot.InterestReply && (botNode == nil || findMappingValue(botNode, "interest_reply") == nil) {
		cfg.Bot.InterestReply = def.Bot.InterestReply
	}
	if cfg.Bot.CooldownSeconds == 0 && (botNode == nil || findMappingValue(botNode, "cooldown_seconds") == nil) {
		cfg.Bot.CooldownSeconds = def.Bot.CooldownSeconds
	}
	if len(cfg.Bot.GroupRules) == 0 && (botNode == nil || findMappingValue(botNode, "group_rules") == nil) {
		cfg.Bot.GroupRules = append([]string(nil), def.Bot.GroupRules...)
	}
	if len(cfg.Bot.Forbidden) == 0 && (botNode == nil || findMappingValue(botNode, "forbidden") == nil) {
		cfg.Bot.Forbidden = append([]string(nil), def.Bot.Forbidden...)
	}
	if cfg.Bot.SystemPrompt == "" && (botNode == nil || findMappingValue(botNode, "system_prompt") == nil) {
		cfg.Bot.SystemPrompt = def.Bot.SystemPrompt
	}

	filterNode := findMappingValue(root, "filter")
	if !cfg.Filter.Enabled && (filterNode == nil || findMappingValue(filterNode, "enabled") == nil) {
		cfg.Filter.Enabled = def.Filter.Enabled
	}
	if cfg.Filter.Mode == "" && (filterNode == nil || findMappingValue(filterNode, "mode") == nil) {
		cfg.Filter.Mode = def.Filter.Mode
	}
	if len(cfg.Filter.GroupIDs) == 0 && (filterNode == nil || findMappingValue(filterNode, "group_ids") == nil) {
		cfg.Filter.GroupIDs = append([]int64(nil), def.Filter.GroupIDs...)
	}
	if len(cfg.Filter.UserIDs) == 0 && (filterNode == nil || findMappingValue(filterNode, "user_ids") == nil) {
		cfg.Filter.UserIDs = append([]int64(nil), def.Filter.UserIDs...)
	}

	memoryNode := findMappingValue(root, "memory")
	if cfg.Memory.ShortTermSize == 0 && (memoryNode == nil || findMappingValue(memoryNode, "short_term_size") == nil) {
		cfg.Memory.ShortTermSize = def.Memory.ShortTermSize
	}
	if !cfg.Memory.LongTermEnabled && (memoryNode == nil || findMappingValue(memoryNode, "long_term_enabled") == nil) {
		cfg.Memory.LongTermEnabled = def.Memory.LongTermEnabled
	}
	if cfg.Memory.ContextMaxAgeMinutes == 0 && (memoryNode == nil || findMappingValue(memoryNode, "context_max_age_minutes") == nil) {
		cfg.Memory.ContextMaxAgeMinutes = def.Memory.ContextMaxAgeMinutes
	}
	if cfg.Memory.SummaryThreshold == 0 && (memoryNode == nil || findMappingValue(memoryNode, "summary_threshold") == nil) {
		cfg.Memory.SummaryThreshold = def.Memory.SummaryThreshold
	}
	if cfg.Memory.SummaryIntervalMinutes == 0 && (memoryNode == nil || findMappingValue(memoryNode, "summary_interval_minutes") == nil) {
		cfg.Memory.SummaryIntervalMinutes = def.Memory.SummaryIntervalMinutes
	}

	rateLimitNode := findMappingValue(root, "rate_limit")
	if cfg.RateLimit.Rate == 0 && (rateLimitNode == nil || findMappingValue(rateLimitNode, "rate") == nil) {
		cfg.RateLimit.Rate = def.RateLimit.Rate
	}
	if cfg.RateLimit.Burst == 0 && (rateLimitNode == nil || findMappingValue(rateLimitNode, "burst") == nil) {
		cfg.RateLimit.Burst = def.RateLimit.Burst
	}

	queueNode := findMappingValue(root, "queue")
	if cfg.Queue.Workers == 0 && (queueNode == nil || findMappingValue(queueNode, "workers") == nil) {
		cfg.Queue.Workers = def.Queue.Workers
	}
	if cfg.Queue.Buffer == 0 && (queueNode == nil || findMappingValue(queueNode, "buffer") == nil) {
		cfg.Queue.Buffer = def.Queue.Buffer
	}

	storeNode := findMappingValue(root, "store")
	if cfg.Store.Driver == "" && (storeNode == nil || findMappingValue(storeNode, "driver") == nil) {
		cfg.Store.Driver = def.Store.Driver
	}
	if cfg.Store.DSN == "" && (storeNode == nil || findMappingValue(storeNode, "dsn") == nil) {
		cfg.Store.DSN = def.Store.DSN
	}

	vlmNode := findMappingValue(root, "vlm")
	if cfg.VLM.Provider == "" && (vlmNode == nil || findMappingValue(vlmNode, "provider") == nil) {
		cfg.VLM.Provider = def.VLM.Provider
	}
	if cfg.VLM.BaseURL == "" && (vlmNode == nil || findMappingValue(vlmNode, "base_url") == nil) {
		cfg.VLM.BaseURL = def.VLM.BaseURL
	}
	if cfg.VLM.Model == "" && (vlmNode == nil || findMappingValue(vlmNode, "model") == nil) {
		cfg.VLM.Model = def.VLM.Model
	}
	if cfg.VLM.TimeoutSeconds == 0 && (vlmNode == nil || findMappingValue(vlmNode, "timeout_seconds") == nil) {
		cfg.VLM.TimeoutSeconds = def.VLM.TimeoutSeconds
	}
	if cfg.VLM.Prompt == "" && (vlmNode == nil || findMappingValue(vlmNode, "prompt") == nil) {
		cfg.VLM.Prompt = def.VLM.Prompt
	}

	memeNode := findMappingValue(root, "meme")
	if cfg.Meme.Directory == "" && (memeNode == nil || findMappingValue(memeNode, "directory") == nil) {
		cfg.Meme.Directory = def.Meme.Directory
	}
	if cfg.Meme.StealProbability == 0 && (memeNode == nil || findMappingValue(memeNode, "steal_probability") == nil) {
		cfg.Meme.StealProbability = def.Meme.StealProbability
	}

	attentionNode := findMappingValue(root, "attention")
	if !cfg.Attention.Enabled && (attentionNode == nil || findMappingValue(attentionNode, "enabled") == nil) {
		cfg.Attention.Enabled = def.Attention.Enabled
	}
	if cfg.Attention.MaxScore == 0 && (attentionNode == nil || findMappingValue(attentionNode, "max_score") == nil) {
		cfg.Attention.MaxScore = def.Attention.MaxScore
	}
	if cfg.Attention.DecayPerMinute == 0 && (attentionNode == nil || findMappingValue(attentionNode, "decay_per_minute") == nil) {
		cfg.Attention.DecayPerMinute = def.Attention.DecayPerMinute
	}
	if cfg.Attention.BoostAtBot == 0 && (attentionNode == nil || findMappingValue(attentionNode, "boost_at_bot") == nil) {
		cfg.Attention.BoostAtBot = def.Attention.BoostAtBot
	}
	if cfg.Attention.BoostMention == 0 && (attentionNode == nil || findMappingValue(attentionNode, "boost_mention") == nil) {
		cfg.Attention.BoostMention = def.Attention.BoostMention
	}
	if cfg.Attention.BoostInterest == 0 && (attentionNode == nil || findMappingValue(attentionNode, "boost_interest") == nil) {
		cfg.Attention.BoostInterest = def.Attention.BoostInterest
	}
	if cfg.Attention.BoostBotReplied == 0 && (attentionNode == nil || findMappingValue(attentionNode, "boost_bot_replied") == nil) {
		cfg.Attention.BoostBotReplied = def.Attention.BoostBotReplied
	}
	if cfg.Attention.TripHoldSeconds == 0 && (attentionNode == nil || findMappingValue(attentionNode, "trip_hold_seconds") == nil) {
		cfg.Attention.TripHoldSeconds = def.Attention.TripHoldSeconds
	}

	webuiNode := findMappingValue(root, "webui")
	if !cfg.WebUI.Enabled && (webuiNode == nil || findMappingValue(webuiNode, "enabled") == nil) {
		cfg.WebUI.Enabled = def.WebUI.Enabled
	}
	if cfg.WebUI.Addr == "" && (webuiNode == nil || findMappingValue(webuiNode, "addr") == nil) {
		cfg.WebUI.Addr = def.WebUI.Addr
	}
	if cfg.WebUI.Token == "" && (webuiNode == nil || findMappingValue(webuiNode, "token") == nil) {
		cfg.WebUI.Token = def.WebUI.Token
	}

	qqNode := findMappingValue(root, "qq")
	onebotNode := findMappingValue(qqNode, "onebot")
	if findMappingValue(onebotNode, "ws_url") == nil && cfg.QQ.OneBot.WSURL == "" {
		cfg.QQ.OneBot.WSURL = def.QQ.OneBot.WSURL
	}

	llmNode := findMappingValue(root, "llm")
	if cfg.LLM.Provider == "" && (llmNode == nil || findMappingValue(llmNode, "provider") == nil) {
		cfg.LLM.Provider = def.LLM.Provider
	}
	if cfg.LLM.BaseURL == "" && (llmNode == nil || findMappingValue(llmNode, "base_url") == nil) {
		cfg.LLM.BaseURL = def.LLM.BaseURL
	}
	if cfg.LLM.Model == "" && (llmNode == nil || findMappingValue(llmNode, "model") == nil) {
		cfg.LLM.Model = def.LLM.Model
	}
	if cfg.LLM.Temperature == 0 && (llmNode == nil || findMappingValue(llmNode, "temperature") == nil) {
		cfg.LLM.Temperature = def.LLM.Temperature
	}
	if cfg.LLM.MaxTokens == 0 && (llmNode == nil || findMappingValue(llmNode, "max_tokens") == nil) {
		cfg.LLM.MaxTokens = def.LLM.MaxTokens
	}
	if cfg.LLM.TimeoutSeconds == 0 && (llmNode == nil || findMappingValue(llmNode, "timeout_seconds") == nil) {
		cfg.LLM.TimeoutSeconds = def.LLM.TimeoutSeconds
	}
	if !cfg.LLM.CachePrefix && (llmNode == nil || findMappingValue(llmNode, "cache_prefix") == nil) {
		cfg.LLM.CachePrefix = def.LLM.CachePrefix
	}
	if cfg.LLM.CacheMaxTokens == 0 && (llmNode == nil || findMappingValue(llmNode, "cache_max_tokens") == nil) {
		cfg.LLM.CacheMaxTokens = def.LLM.CacheMaxTokens
	}
	if !cfg.LLM.LogCache && (llmNode == nil || findMappingValue(llmNode, "log_cache") == nil) {
		cfg.LLM.LogCache = def.LLM.LogCache
	}

	logNode := findMappingValue(root, "log")
	if findMappingValue(logNode, "level") == nil && cfg.Log.Level == "" {
		cfg.Log.Level = def.Log.Level
	}
}

// Validate 检查关键配置是否满足启动条件。
func (c *Config) Validate() error {
	if c.Bot.SelfID == 0 {
		return fmt.Errorf("bot.self_id must not be empty")
	}
	if c.QQ.OneBot.WSURL == "" {
		return fmt.Errorf("qq.onebot.ws_url must not be empty")
	}
	if c.Memory.LongTermEnabled && c.Store.DSN == "" {
		return fmt.Errorf("store.dsn must not be empty when long_term_enabled is true")
	}
	if c.WebUI.Enabled && c.WebUI.Addr == "" {
		return fmt.Errorf("webui.addr must not be empty when webui.enabled is true")
	}
	return nil
}

func nodeVersion(root *yaml.Node) (int, error) {
	v := findMappingValue(root, "version")
	if v == nil {
		// 没有 version 字段视为 v1，触发自动迁移。
		return 1, nil
	}

	var n int
	if err := v.Decode(&n); err != nil {
		return 0, fmt.Errorf("invalid config version: %w", err)
	}
	return n, nil
}

// migrateNode 把旧版本节点迁移到当前版本。
// 使用 yaml.Node 操作可以尽量保留原配置文件里的注释。
