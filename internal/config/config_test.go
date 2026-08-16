package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"icarosu/internal/config"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir, err := os.MkdirTemp(".", ".config-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	// 默认不加载独立 provider 文件，除非测试显式配置 llm_config_file
	if !strings.Contains(content, "llm_config_file:") {
		content += "\nllm_config_file: \"\"\n"
	}

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestWriteSplitSeparatesLLM(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".config-split-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	mainPath := filepath.Join(dir, "config.yaml")
	cfg := config.Default()
	cfg.LLMConfigFile = "llm.yaml"
	cfg.LLM.Model = "split-llm-model"
	cfg.VLM.Model = "split-vlm-model"

	if err := config.WriteSplit(cfg, mainPath); err != nil {
		t.Fatalf("WriteSplit: %v", err)
	}

	mainData, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main config: %v", err)
	}
	mainStr := string(mainData)
	if strings.Contains(mainStr, "\nllm:") || strings.Contains(mainStr, "\nvlm:") {
		t.Fatalf("main config should not contain llm/vlm sections\n%s", mainStr)
	}
	if !strings.Contains(mainStr, "llm_config_file: llm.yaml") {
		t.Fatalf("main config should reference llm.yaml\n%s", mainStr)
	}

	llmData, err := os.ReadFile(filepath.Join(dir, "llm.yaml"))
	if err != nil {
		t.Fatalf("read llm config: %v", err)
	}
	llmStr := string(llmData)
	if !strings.Contains(llmStr, "split-llm-model") || !strings.Contains(llmStr, "split-vlm-model") {
		t.Fatalf("llm.yaml should contain llm/vlm models\n%s", llmStr)
	}

	loaded, err := config.Load(mainPath)
	if err != nil {
		t.Fatalf("Load after split: %v", err)
	}
	if loaded.LLM.Model != "split-llm-model" || loaded.VLM.Model != "split-vlm-model" {
		t.Fatalf("unexpected loaded llm/vlm: %+v %+v", loaded.LLM, loaded.VLM)
	}
}

func TestLoadMigratesV1ToCurrent(t *testing.T) {
	old := `# keep this comment
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
    access_token: ""
    self_id: 123
log:
  level: "debug"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Version != config.CurrentVersion {
		t.Fatalf("expected version %d, got %d", config.CurrentVersion, cfg.Version)
	}
	if cfg.Bot.SelfID != 123 {
		t.Fatalf("expected self_id to be migrated to bot, got %d", cfg.Bot.SelfID)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("expected log level to be preserved, got %q", cfg.Log.Level)
	}
	if cfg.LLM.BaseURL == "" || cfg.LLM.Model == "" {
		t.Fatalf("expected LLM defaults to be filled, got %+v", cfg.LLM)
	}
	if cfg.Bot.Name == "" || cfg.Bot.Personality == "" || cfg.Bot.SpeakingStyle == "" {
		t.Fatalf("expected bot persona defaults to be filled, got %+v", cfg.Bot)
	}
	if cfg.Filter.Mode != "allow" || cfg.Filter.Enabled || len(cfg.Filter.UserIDs) != 0 {
		t.Fatalf("expected filter defaults to be filled, got %+v", cfg.Filter)
	}
	if !cfg.Bot.MentionReply || !cfg.Bot.InterestReply || len(cfg.Bot.Keywords) != 0 {
		t.Fatalf("expected trigger defaults to be filled, got %+v", cfg.Bot)
	}
	if cfg.Memory.ShortTermSize != 20 || !cfg.Memory.LongTermEnabled {
		t.Fatalf("expected memory defaults to be filled, got %+v", cfg.Memory)
	}
	if cfg.Store.Driver != "sqlite" || cfg.Store.DSN == "" {
		t.Fatalf("expected store defaults to be filled, got %+v", cfg.Store)
	}
	if cfg.VLM.Model == "" || cfg.VLM.Prompt == "" || cfg.VLM.BaseURL == "" {
		t.Fatalf("expected vlm defaults to be filled, got %+v", cfg.VLM)
	}
	if cfg.Meme.Directory == "" || cfg.Meme.StealProbability == 0 {
		t.Fatalf("expected meme defaults to be filled, got %+v", cfg.Meme)
	}
	if !cfg.Attention.Enabled || cfg.Attention.MaxScore == 0 || cfg.Attention.BoostAtBot == 0 || cfg.Attention.TripHoldSeconds == 0 {
		t.Fatalf("expected attention defaults to be filled, got %+v", cfg.Attention)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if !strings.Contains(string(data), "version: 12") {
		t.Fatalf("migrated config should contain version: 12\n%s", data)
	}
	if strings.Contains(string(data), "!!float") {
		t.Fatalf("migrated config should not contain !!float tags\n%s", data)
	}
	if !strings.Contains(string(data), "llm:") {
		t.Fatalf("migrated config should contain llm section\n%s", data)
	}
	if !strings.Contains(string(data), "bot:") {
		t.Fatalf("migrated config should contain bot section\n%s", data)
	}
	if !strings.Contains(string(data), "# keep this comment") {
		t.Fatalf("migrated config should preserve comments\n%s", data)
	}

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected backup file to exist: %v", err)
	}
}

func TestLoadMigratesV2ToCurrent(t *testing.T) {
	old := `version: 2
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
    access_token: ""
    self_id: 123
llm:
  base_url: "https://example.com/v1"
  api_key: "key"
  model: "my-model"
  temperature: 0.5
  max_tokens: 100
  timeout_seconds: 10
  system_prompt: "旧的自定义提示词"
log:
  level: "debug"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Bot.SelfID != 123 {
		t.Fatalf("expected self_id to be migrated to bot, got %d", cfg.Bot.SelfID)
	}
	if cfg.Bot.SystemPrompt != "旧的自定义提示词" {
		t.Fatalf("expected system_prompt to be migrated to bot, got %q", cfg.Bot.SystemPrompt)
	}
	if cfg.LLM.Model != "my-model" || cfg.LLM.Temperature != 0.5 {
		t.Fatalf("expected llm values preserved, got %+v", cfg.LLM)
	}
	if cfg.Filter.Mode != "allow" || cfg.Filter.Enabled || len(cfg.Filter.UserIDs) != 0 {
		t.Fatalf("expected filter defaults to be filled, got %+v", cfg.Filter)
	}
	if !cfg.Bot.MentionReply || cfg.Memory.ShortTermSize != 20 || cfg.Store.Driver != "sqlite" {
		t.Fatalf("expected new feature defaults to be filled, bot=%+v memory=%+v store=%+v", cfg.Bot, cfg.Memory, cfg.Store)
	}
}

func TestLoadCurrentVersionDoesNotRewrite(t *testing.T) {
	current := `version: 12
llm_config_file: ""
bot:
  self_id: 123
  name: "小明"
  nicknames:
    - "小明"
  personality: "安静"
  speaking_style: "简短"
  background: "普通网友"
  interests:
    - "读书"
  mention_reply: true
  keywords:
    - "测试"
  random_probability: 0.1
  interest_reply: true
  cooldown_seconds: 30
  group_rules:
    - "不刷屏"
  forbidden:
    - "不违法"
  system_prompt: "自定义完整提示词"
filter:
  enabled: true
  mode: "allow"
  group_ids:
    - 111
    - 222
  user_ids:
    - 333
memory:
  short_term_size: 50
  long_term_enabled: true
store:
  driver: "sqlite"
  dsn: "data/test.db"
vlm:
  base_url: "https://vlm.example.com/v1"
  api_key: "vlm-key"
  model: "vlm-model"
  timeout_seconds: 20
  prompt: "test prompt"
meme:
  directory: "data/test-memes"
  steal_probability: 0.5
attention:
  enabled: true
  max_score: 80
  decay_per_minute: 3
  boost_at_bot: 15
  boost_mention: 8
  boost_interest: 12
  boost_bot_replied: 6
  trip_hold_seconds: 120
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
    access_token: ""
llm:
  base_url: "https://example.com/v1"
  api_key: "key"
  model: "my-model"
  temperature: 0.5
  max_tokens: 100
  timeout_seconds: 10
  cache_prefix: true
  cache_max_tokens: 2048
  log_cache: true
log:
  level: "debug"
`
	path := writeTempConfig(t, current)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.Model != "my-model" || cfg.LLM.Temperature != 0.5 {
		t.Fatalf("current config values not preserved: %+v", cfg.LLM)
	}
	if cfg.Bot.SelfID != 123 || cfg.Bot.SystemPrompt != "自定义完整提示词" {
		t.Fatalf("current bot config not preserved: %+v", cfg.Bot)
	}
	if !cfg.Filter.Enabled || cfg.Filter.Mode != "allow" || len(cfg.Filter.GroupIDs) != 2 || len(cfg.Filter.UserIDs) != 1 {
		t.Fatalf("current filter config not preserved: %+v", cfg.Filter)
	}
	if !cfg.Bot.MentionReply || cfg.Bot.RandomProbability != 0.1 || cfg.Bot.CooldownSeconds != 30 {
		t.Fatalf("current trigger config not preserved: %+v", cfg.Bot)
	}
	if cfg.Memory.ShortTermSize != 50 || !cfg.Memory.LongTermEnabled {
		t.Fatalf("current memory config not preserved: %+v", cfg.Memory)
	}
	if cfg.Store.Driver != "sqlite" || cfg.Store.DSN != "data/test.db" {
		t.Fatalf("current store config not preserved: %+v", cfg.Store)
	}
	if cfg.VLM.Model != "vlm-model" || cfg.VLM.Prompt != "test prompt" {
		t.Fatalf("current vlm config not preserved: %+v", cfg.VLM)
	}
	if cfg.Meme.Directory != "data/test-memes" || cfg.Meme.StealProbability != 0.5 {
		t.Fatalf("current meme config not preserved: %+v", cfg.Meme)
	}
	if !cfg.Attention.Enabled || cfg.Attention.MaxScore != 80 || cfg.Attention.BoostAtBot != 15 || cfg.Attention.TripHoldSeconds != 120 {
		t.Fatalf("current attention config not preserved: %+v", cfg.Attention)
	}
	if !cfg.LLM.CachePrefix || cfg.LLM.CacheMaxTokens != 2048 || !cfg.LLM.LogCache {
		t.Fatalf("current llm cache config not preserved: %+v", cfg.LLM)
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("current version should not create backup, stat err: %v", err)
	}
}

func TestLoadPreservesExplicitZeroTemperature(t *testing.T) {
	content := `version: 12
bot:
  self_id: 0
  name: "测试"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
llm:
  base_url: "https://example.com/v1"
  model: "my-model"
  temperature: 0
  max_tokens: 200
  timeout_seconds: 10
log:
  level: "info"
`
	path := writeTempConfig(t, content)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.Temperature != 0 {
		t.Fatalf("expected explicit temperature 0 to be preserved, got %v", cfg.LLM.Temperature)
	}
	if cfg.LLM.MaxTokens != 200 {
		t.Fatalf("expected max_tokens 200, got %d", cfg.LLM.MaxTokens)
	}
}

func TestLoadMigratesV3ToCurrent(t *testing.T) {
	old := `version: 3
bot:
  self_id: 123
  name: "小明"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
llm:
  base_url: "https://example.com/v1"
  model: "my-model"
log:
  level: "debug"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Filter.Enabled {
		t.Fatal("expected filter to default to disabled")
	}
	if cfg.Filter.Mode != "allow" || len(cfg.Filter.UserIDs) != 0 {
		t.Fatalf("expected filter defaults to be filled, got %+v", cfg.Filter)
	}
	if !cfg.Bot.MentionReply || cfg.Memory.ShortTermSize != 20 || cfg.Store.Driver != "sqlite" {
		t.Fatalf("expected new feature defaults, bot=%+v memory=%+v store=%+v", cfg.Bot, cfg.Memory, cfg.Store)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if !strings.Contains(string(data), "filter:") {
		t.Fatalf("migrated config should contain filter section\n%s", data)
	}
}

func TestLoadMigratesV4ToCurrent(t *testing.T) {
	old := `version: 4
filter:
  enabled: true
  mode: "allow"
  group_ids:
    - 111
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Filter.UserIDs) != 0 {
		t.Fatalf("expected user_ids to default to empty, got %v", cfg.Filter.UserIDs)
	}
	if !cfg.Bot.MentionReply || cfg.Memory.ShortTermSize != 20 || cfg.Store.Driver != "sqlite" {
		t.Fatalf("expected new feature defaults, bot=%+v memory=%+v store=%+v", cfg.Bot, cfg.Memory, cfg.Store)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if !strings.Contains(string(data), "user_ids:") {
		t.Fatalf("migrated config should contain user_ids\n%s", data)
	}
}

func TestMigratedConfigUsesPresetOrder(t *testing.T) {
	old := `log:
  level: "debug"
qq:
  onebot:
    access_token: ""
    ws_url: "ws://127.0.0.1:6700"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	_ = cfg

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}

	content := string(data)
	order := []string{"version:", "bot:", "filter:", "memory:", "store:", "vlm:", "meme:", "attention:", "qq:", "llm:", "log:"}
	last := -1
	for _, key := range order {
		idx := strings.Index(content, key)
		if idx < 0 {
			t.Fatalf("migrated config missing %q\n%s", key, content)
		}
		if idx < last {
			t.Fatalf("migrated config order wrong for %q\n%s", key, content)
		}
		last = idx
	}
}

func TestCurrentNonCanonicalConfigIsRewritten(t *testing.T) {
	content := `version: 12
log:
  level: "debug"
meme:
  directory: "data/memes"
  steal_probability: 0.8
attention:
  enabled: true
  max_score: 100
  decay_per_minute: 5
  boost_at_bot: 20
  boost_mention: 10
  boost_interest: 15
  boost_bot_replied: 10
store:
  driver: "sqlite"
  dsn: "data/bot.db"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
llm:
  base_url: "https://example.com/v1"
vlm:
  base_url: "https://example.com/v1"
  api_key: ""
  model: "vlm-model"
  timeout_seconds: 30
  prompt: "test"
filter:
  enabled: false
  mode: "allow"
  group_ids: []
  user_ids: []
bot:
  self_id: 0
  name: "小智"
  mention_reply: true
  keywords: []
  random_probability: 0.05
  interest_reply: true
  cooldown_seconds: 60
memory:
  short_term_size: 20
  long_term_enabled: true
`
	path := writeTempConfig(t, content)

	if _, err := config.Load(path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read normalized config: %v", err)
	}
	content = string(data)

	order := []string{"version:", "bot:", "filter:", "memory:", "store:", "vlm:", "meme:", "attention:", "qq:", "llm:", "log:"}
	last := -1
	for _, key := range order {
		idx := strings.Index(content, key)
		if idx < 0 {
			t.Fatalf("normalized config missing %q\n%s", key, content)
		}
		if idx < last {
			t.Fatalf("normalized config order wrong for %q\n%s", key, content)
		}
		last = idx
	}

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected backup file to exist: %v", err)
	}
}

func TestLoadMigratesV6ToCurrent(t *testing.T) {
	old := `version: 6
bot:
  self_id: 123
  name: "小明"
filter:
  enabled: true
  mode: "allow"
  group_ids:
    - 111
trigger:
  enabled: false
  mention_reply: true
  keywords:
    - "测试"
  random_probability: 0.2
  interest_reply: true
  cooldown_seconds: 30
memory:
  short_term_size: 20
  long_term_enabled: true
store:
  driver: "sqlite"
  dsn: "data/bot.db"
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
llm:
  base_url: "https://example.com/v1"
  model: "my-model"
log:
  level: "debug"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Bot.MentionReply || cfg.Bot.RandomProbability != 0.2 || cfg.Bot.CooldownSeconds != 30 {
		t.Fatalf("trigger fields should be merged into bot, got %+v", cfg.Bot)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(data), "\ntrigger:") {
		t.Fatalf("migrated config should not contain trigger section\n%s", data)
	}
}

func TestLoadMigratesV8ToCurrent(t *testing.T) {
	old := `version: 8
vlm:
  enabled: true
  base_url: "https://example.com/v1"
  model: "vlm-model"
meme:
  enabled: true
  directory: "data/memes"
  steal_probability: 0.8
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
`
	path := writeTempConfig(t, old)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	_ = cfg

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(data), "vlm:\n  enabled") || strings.Contains(string(data), "meme:\n  enabled") {
		t.Fatalf("migrated config should not contain vlm/meme enabled\n%s", data)
	}
	if !strings.Contains(string(data), "version: 12") {
		t.Fatalf("migrated config should contain version: 12\n%s", data)
	}
}

func TestLoadSeparateProviderFile(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".config-provider-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	llmPath := filepath.Join(dir, "llm.yaml")
	if err := os.WriteFile(llmPath, []byte(`providers:
  qwen:
    base_url: "https://custom.example.com/v1"
    api_key: "provider-key"
    models:
      - name: "qwen-plus"
        kind: "llm"
      - name: "qwen-vl-plus"
        kind: "vlm"
llm:
  provider: "qwen"
  model: "qwen-plus"
vlm:
  provider: "qwen"
  model: "qwen-vl-plus"
`), 0o644); err != nil {
		t.Fatalf("write llm config: %v", err)
	}

	mainPath := filepath.Join(dir, "config.yaml")
	mainContent := fmt.Sprintf("version: 12\nllm_config_file: %q\nqq:\n  onebot:\n    ws_url: \"ws://127.0.0.1:6700\"\n", filepath.ToSlash(llmPath))
	if err := os.WriteFile(mainPath, []byte(mainContent), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	cfg, err := config.Load(mainPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.Provider != "qwen" || cfg.LLM.BaseURL != "https://custom.example.com/v1" || cfg.LLM.APIKey != "provider-key" || cfg.LLM.Model != "qwen-plus" {
		t.Fatalf("llm provider config not applied: %+v", cfg.LLM)
	}
	if cfg.VLM.Provider != "qwen" || cfg.VLM.BaseURL != "https://custom.example.com/v1" || cfg.VLM.APIKey != "provider-key" || cfg.VLM.Model != "qwen-vl-plus" {
		t.Fatalf("vlm provider config not applied: %+v", cfg.VLM)
	}
}

func TestLoadRejectsNewerVersion(t *testing.T) {
	path := writeTempConfig(t, `version: 999
qq:
  onebot:
    ws_url: "ws://127.0.0.1:6700"
`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for newer config version")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected newer version error, got: %v", err)
	}
}
