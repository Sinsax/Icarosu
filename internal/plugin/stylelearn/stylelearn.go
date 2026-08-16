package stylelearn

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"strings"

	"icarosu/internal/llm"
	"icarosu/internal/plugin"
	"icarosu/internal/style"
)

// StyleLearnPlugin 随机采样消息，让 LLM 判断是否值得收入表达方式库。
type StyleLearnPlugin struct {
	plugin.BasePlugin
	client *llm.Client
	lib    *style.Library
	logger *slog.Logger
}

// Option 用于配置 StyleLearnPlugin。
type Option func(*StyleLearnPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *StyleLearnPlugin) {
		p.logger = logger
	}
}

func New(client *llm.Client, lib *style.Library, opts ...Option) plugin.Plugin {
	p := &StyleLearnPlugin{
		client: client,
		lib:    lib,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *StyleLearnPlugin) Name() string {
	return "stylelearn"
}

func (p *StyleLearnPlugin) Description() string {
	return "随机采样消息，学习表达方式并打标签"
}

func (p *StyleLearnPlugin) Priority() int {
	return 910
}

func (p *StyleLearnPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage {
		return false
	}
	if p.client == nil || p.lib == nil {
		return false
	}
	if rand.Float64() > 0.02 {
		return false
	}

	text := strings.TrimSpace(e.RawMessage)
	if text == "" || len([]rune(text)) > 30 {
		return false
	}

	result, err := p.analyze(ctx, text)
	if err != nil || !result.Collect {
		return false
	}

	p.lib.Add(result.Tags, result.Expression)
	if p.logger != nil {
		p.logger.Info("learned expression", "expression", result.Expression, "tags", result.Tags)
	}
	return false
}

type learnResult struct {
	Collect    bool     `json:"collect"`
	Expression string   `json:"expression"`
	Tags       []string `json:"tags"`
}

func (p *StyleLearnPlugin) analyze(ctx context.Context, text string) (*learnResult, error) {
	prompt := `判断下面这句话是否适合作为群聊里的表达方式/口癖/万能回复。
如果适合，输出JSON：{"collect":true,"expression":"要收藏的短句","tags":["标签1","标签2"]}
如果不适合，输出JSON：{"collect":false}
只输出JSON，不要解释。
句子：` + text

	messages := []llm.Message{
		{Role: "system", Content: "你是群聊表达方式筛选助手。"},
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

	var result learnResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
