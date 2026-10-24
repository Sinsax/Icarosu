package jargonlearn

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"strings"

	"icarosu/internal/llm"
	"icarosu/internal/plugin"
)

// GroupScope 是黑话互通范围解析能力。
type GroupScope interface {
	ExpressionGroupIDs(groupID int64) []int64
}

// JargonStore 是黑话持久化能力。
type JargonStore interface {
	SaveJargon(groupID int64, term, meaning string, sourceUserID int64) error
}

// JargonLearnPlugin 随机采样群消息，让 LLM 识别并保存群内黑话/梗。
type JargonLearnPlugin struct {
	plugin.BasePlugin
	client llm.Provider
	scope  GroupScope
	store  JargonStore
	logger *slog.Logger
}

// Option 用于配置 JargonLearnPlugin。
type Option func(*JargonLearnPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *JargonLearnPlugin) {
		p.logger = logger
	}
}

// WithGroupScope 设置互通范围。
func WithGroupScope(sc GroupScope) Option {
	return func(p *JargonLearnPlugin) {
		p.scope = sc
	}
}

// WithStore 设置黑话持久化器。
func WithStore(st JargonStore) Option {
	return func(p *JargonLearnPlugin) {
		p.store = st
	}
}

// New 创建黑话学习插件。
func New(client llm.Provider, opts ...Option) plugin.Plugin {
	p := &JargonLearnPlugin{
		client: client,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *JargonLearnPlugin) Name() string {
	return "jargonlearn"
}

func (p *JargonLearnPlugin) Description() string {
	return "随机采样群消息，学习群内黑话/梗"
}

func (p *JargonLearnPlugin) Priority() int {
	return 905
}

func (p *JargonLearnPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *JargonLearnPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage {
		return false
	}
	if p.client == nil {
		return false
	}
	if rand.Float64() > 0.02 {
		return false
	}

	text := strings.TrimSpace(e.RawMessage)
	if text == "" || len([]rune(text)) > 50 {
		return false
	}

	result, err := p.analyze(ctx, text)
	if err != nil || result.Term == "" {
		return false
	}

	groupIDs := []int64{e.GroupID}
	if p.scope != nil {
		groupIDs = p.scope.ExpressionGroupIDs(e.GroupID)
	}
	if p.store != nil {
		for _, gid := range groupIDs {
			_ = p.store.SaveJargon(gid, result.Term, result.Meaning, e.UserID)
		}
	}
	if p.logger != nil {
		p.logger.Info("learned jargon", "term", result.Term, "meaning", result.Meaning, "group_ids", groupIDs)
	}
	return false
}

type jargonResult struct {
	Term    string `json:"term"`
	Meaning string `json:"meaning"`
}

func (p *JargonLearnPlugin) analyze(ctx context.Context, text string) (*jargonResult, error) {
	prompt := `判断这句话里是否包含群聊黑话、缩写或内部梗。
如果有，输出JSON：{"term":"黑话","meaning":"含义"}
如果没有，输出JSON：{"term":"","meaning":""}
只输出JSON，不要解释。
句子：` + text

	messages := []llm.Message{
		{Role: "system", Content: "你是群聊黑话识别助手。"},
		{Role: "user", Content: prompt},
	}
	out, err := p.client.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	out = strings.TrimPrefix(out, "```json")
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimSuffix(out, "```")

	var result jargonResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
