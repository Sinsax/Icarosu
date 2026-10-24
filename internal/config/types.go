package config
const CurrentVersion = 12

type Config struct {
	Version               int             `yaml:"version"`
	LLMConfigFile         string          `yaml:"llm_config_file"`
	DryRun                bool            `yaml:"dry_run"`
	Bot                   BotConfig       `yaml:"bot"`
	QQ                    QQConfig        `yaml:"qq"`
	LLM                   LLMConfig       `yaml:"llm"`
	Filter                FilterConfig    `yaml:"filter"`
	Memory                MemoryConfig    `yaml:"memory"`
	RateLimit             RateLimitConfig `yaml:"rate_limit"`
	Queue                 QueueConfig     `yaml:"queue"`
	Store                 StoreConfig     `yaml:"store"`
	VLM                   VLMConfig       `yaml:"vlm"`
	Meme                  MemeConfig      `yaml:"meme"`
	Attention             AttentionConfig `yaml:"attention"`
	MemoryShareGroups     []ShareGroup    `yaml:"memory_share_groups"`
	ExpressionShareGroups []ShareGroup    `yaml:"expression_share_groups"`
	WebUI                 WebUIConfig     `yaml:"webui"`
	Log                   LogConfig       `yaml:"log"`
}

// ShareGroup 定义一组共享同一类数据的群。
type ShareGroup struct {
	Name     string  `yaml:"name"`
	GroupIDs []int64 `yaml:"group_ids"`
}

type BotConfig struct {
	SelfID            int64    `yaml:"self_id"`
	AdminIDs          []int64  `yaml:"admin_ids"`
	Name              string   `yaml:"name"`
	Nicknames         []string `yaml:"nicknames"`
	Personality       string   `yaml:"personality"`
	SpeakingStyle     string   `yaml:"speaking_style"`
	Background        string   `yaml:"background"`
	Interests         []string `yaml:"interests"`
	MentionReply      bool     `yaml:"mention_reply"`
	Keywords          []string `yaml:"keywords"`
	RandomProbability float64  `yaml:"random_probability"`
	InterestReply     bool     `yaml:"interest_reply"`
	CooldownSeconds   int      `yaml:"cooldown_seconds"`
	GroupRules        []string `yaml:"group_rules"`
	Forbidden         []string `yaml:"forbidden"`
	SystemPrompt      string   `yaml:"system_prompt"`
}

type FilterConfig struct {
	Enabled  bool    `yaml:"enabled"`
	Mode     string  `yaml:"mode"`
	GroupIDs []int64 `yaml:"group_ids"`
	UserIDs  []int64 `yaml:"user_ids"`
}

type MemoryConfig struct {
	ShortTermSize          int  `yaml:"short_term_size"`
	LongTermEnabled        bool `yaml:"long_term_enabled"`
	ContextMaxAgeMinutes   int  `yaml:"context_max_age_minutes"`
	SummaryThreshold       int  `yaml:"summary_threshold"`
	SummaryIntervalMinutes int  `yaml:"summary_interval_minutes"`
}

type RateLimitConfig struct {
	Rate  float64 `yaml:"rate"`
	Burst float64 `yaml:"burst"`
}

type QueueConfig struct {
	Workers int `yaml:"workers"`
	Buffer  int `yaml:"buffer"`
}

type StoreConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type VLMConfig struct {
	Provider       string `yaml:"provider"`
	BaseURL        string `yaml:"base_url"`
	APIKey         string `yaml:"api_key"`
	Model          string `yaml:"model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	Prompt         string `yaml:"prompt"`
}

type MemeConfig struct {
	Directory        string  `yaml:"directory"`
	StealProbability float64 `yaml:"steal_probability"`
}

type AttentionConfig struct {
	Enabled         bool    `yaml:"enabled"`
	MaxScore        float64 `yaml:"max_score"`
	DecayPerMinute  float64 `yaml:"decay_per_minute"`
	BoostAtBot      float64 `yaml:"boost_at_bot"`
	BoostMention    float64 `yaml:"boost_mention"`
	BoostInterest   float64 `yaml:"boost_interest"`
	BoostBotReplied float64 `yaml:"boost_bot_replied"`
	TripHoldSeconds int     `yaml:"trip_hold_seconds"`
}

type WebUIConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
	Token   string `yaml:"token"`
}

type QQConfig struct {
	OneBot OneBotConfig `yaml:"onebot"`
}

type OneBotConfig struct {
	WSURL       string `yaml:"ws_url"`
	AccessToken string `yaml:"access_token"`
}

type LLMConfig struct {
	Provider       string  `yaml:"provider"`
	BaseURL        string  `yaml:"base_url"`
	APIKey         string  `yaml:"api_key"`
	Model          string  `yaml:"model"`
	Temperature    float64 `yaml:"temperature"`
	MaxTokens      int     `yaml:"max_tokens"`
	TimeoutSeconds int     `yaml:"timeout_seconds"`
	CachePrefix    bool    `yaml:"cache_prefix"`
	CacheMaxTokens int     `yaml:"cache_max_tokens"`
	LogCache       bool    `yaml:"log_cache"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// 配置文件中各段的预设顺序，迁移/规范化时会按这个顺序重排。
var (
	topLevelOrder  = []string{"version", "llm_config_file", "dry_run", "bot", "filter", "memory", "rate_limit", "queue", "store", "vlm", "meme", "attention", "memory_share_groups", "expression_share_groups", "webui", "qq", "llm", "log"}
	botOrder       = []string{"self_id", "admin_ids", "name", "nicknames", "personality", "speaking_style", "background", "interests", "mention_reply", "keywords", "random_probability", "interest_reply", "cooldown_seconds", "group_rules", "forbidden", "system_prompt"}
	filterOrder    = []string{"enabled", "mode", "group_ids", "user_ids"}
	memoryOrder    = []string{"short_term_size", "long_term_enabled", "context_max_age_minutes", "summary_threshold", "summary_interval_minutes"}
	rateLimitOrder = []string{"rate", "burst"}
	queueOrder     = []string{"workers", "buffer"}
	storeOrder     = []string{"driver", "dsn"}
	vlmOrder       = []string{"provider", "base_url", "api_key", "model", "timeout_seconds", "prompt"}
	memeOrder      = []string{"directory", "steal_probability"}
	attentionOrder = []string{"enabled", "max_score", "decay_per_minute", "boost_at_bot", "boost_mention", "boost_interest", "boost_bot_replied", "trip_hold_seconds"}
	webuiOrder     = []string{"enabled", "addr", "token"}
	qqOrder        = []string{"onebot"}
	onebotOrder    = []string{"ws_url", "access_token"}
	llmOrder       = []string{"provider", "base_url", "api_key", "model", "temperature", "max_tokens", "timeout_seconds", "cache_prefix", "cache_max_tokens", "log_cache"}
	logOrder       = []string{"level"}
)


