package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"

	corechat "icarosu/internal/core/chat"
	"icarosu/internal/core/domain"
	"icarosu/internal/attention"
	"icarosu/internal/llm"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/semantic"
	"icarosu/internal/store"
	"icarosu/internal/style"
	"icarosu/internal/tools"
	"icarosu/internal/topic"
)

// memoryAdapter 把 *memory.Memory 适配成 corechat.Memory 接口。
type memoryAdapter struct {
	inner *memory.Memory
}

func (a memoryAdapter) Recent(m *domain.ChatMessage, limit int) []memory.Entry {
	return a.inner.Recent(toPluginEvent(m), limit)
}

func (a memoryAdapter) LongTermMemoriesForGroups(groupIDs []int64, limit int) []string {
	return a.inner.LongTermMemoriesForGroups(groupIDs, limit)
}

func (a memoryAdapter) RecordReply(m *domain.ChatMessage, reply string) {
	a.inner.RecordReply(toPluginEvent(m), reply)
}

func toPluginEvent(m *domain.ChatMessage) *plugin.Event {
	typ := plugin.EventPrivateMessage
	if m.Type == domain.MessageGroup {
		typ = plugin.EventGroupMessage
	}
	images := make([]plugin.Image, 0, len(m.Images))
	for _, img := range m.Images {
		images = append(images, plugin.Image{URL: img.URL, File: img.File})
	}
	return &plugin.Event{
		Type:             typ,
		GroupID:          m.GroupID,
		UserID:           m.UserID,
		MessageID:        m.MessageID,
		RawMessage:       m.RawMessage,
		SenderName:       m.SenderName,
		AtBot:            m.AtBot,
		Images:           images,
		ReplyToMessageID: m.ReplyToMessageID,
		ReplyToUserID:    m.ReplyToUserID,
		OperatorID:       m.OperatorID,
		ImageContext:     m.ImageContext,
	}
}

// ChatPlugin 是 LLM 对话插件。
// 核心回复逻辑已经下沉到 internal/core/chat，这里保留事件适配和少量发送/工具逻辑。
type ChatPlugin struct {
	plugin.BasePlugin
	svc    *corechat.Service
	client llm.Provider
	memes  *meme.Store
	tools  *tools.Registry
	logger *slog.Logger
}

// Option 用于配置 ChatPlugin。
type Option func(*ChatPlugin)

// WithTools 设置可用的工具注册表。
func WithTools(registry *tools.Registry) Option {
	return func(p *ChatPlugin) {
		p.tools = registry
	}
}

// WithMemory 设置短期/长期记忆服务。
func WithMemory(mem *memory.Memory) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithMemory(memoryAdapter{inner: mem})(p.svc)
		}
	}
}

// WithMemes 设置表情包库。
func WithMemes(store *meme.Store) Option {
	return func(p *ChatPlugin) {
		p.memes = store
	}
}

// WithStyleLibrary 设置表达方式库。
func WithStyleLibrary(lib *style.Library) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithStyleLibrary(lib)(p.svc)
		}
	}
}

// WithStore 设置数据库存储，用于载入活跃时间/互动/事件等上下文。
func WithStore(st store.ChatContextStore) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithStore(st)(p.svc)
		}
	}
}

// WithSemantic 设置语义记忆服务。
func WithSemantic(svc *semantic.Service) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithSemantic(svc)(p.svc)
		}
	}
}

// WithTopicTracker 设置话题稳定性跟踪器。
func WithTopicTracker(t *topic.Tracker) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithTopicTracker(t)(p.svc)
		}
	}
}

// WithAttention 设置注意力系统。
func WithAttention(t *attention.Tracker) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithAttention(t)(p.svc)
		}
	}
}

// WithGroupScope 设置群维度隔离与互通组解析器。
func WithGroupScope(sc corechat.GroupScope) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithGroupScope(sc)(p.svc)
		}
	}
}

// WithJargons 设置群黑话查询能力。
func WithJargons(provider corechat.JargonProvider) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithJargons(provider)(p.svc)
		}
	}
}

// WithProfiles 设置群用户画像查询能力。
func WithProfiles(provider corechat.ProfileProvider) Option {
	return func(p *ChatPlugin) {
		if p.svc != nil {
			corechat.WithProfiles(provider)(p.svc)
		}
	}
}

// WithLogger 设置插件日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *ChatPlugin) {
		p.logger = logger
		if p.svc != nil {
			corechat.WithLogger(logger)(p.svc)
		}
	}
}

func New(client llm.Provider, systemPrompt string, opts ...Option) *ChatPlugin {
	p := &ChatPlugin{
		client: client,
		logger: slog.Default(),
	}
	p.svc = corechat.New(systemPrompt, corechat.WithLogger(p.logger))
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// UpdatePersona 热更新系统人设提示词。
func (p *ChatPlugin) UpdatePersona(prompt string) {
	if p.svc != nil {
		p.svc.UpdatePersona(prompt)
	}
}

func (p *ChatPlugin) Name() string {
	return "chat"
}

func (p *ChatPlugin) Description() string {
	return "接入 OpenAI 兼容 LLM，支持工具调用，被 @ 或私聊时回复"
}

func (p *ChatPlugin) Priority() int {
	return 50
}

func (p *ChatPlugin) Handle(ctx context.Context, bot plugin.Bot, e *plugin.Event) bool {
	pc := plugin.NewContext(e)
	handled := p.HandleContext(ctx, bot, pc)
	if handled {
		if reply, ok := pc.Get("reply").(string); ok && reply != "" {
			forceSplit, _ := pc.Get("force_split").(bool)
			p.sendReply(ctx, bot, e, reply, forceSplit)
		}
	}
	return handled
}

// HandleContext 实现 ContextualPlugin。
// 它只负责生成回复并写入流水线中间产物，由 Postprocessor sender 负责最终发送。
func (p *ChatPlugin) HandleContext(ctx context.Context, bot plugin.Bot, pc *plugin.Context) bool {
	e := pc.Event
	dm := plugin.ToDomainMessage(e)
	if p.client == nil {
		return false
	}

	switch e.Type {
	case plugin.EventGroupMessage, plugin.EventPrivateMessage:
		// 能走到这里说明已经通过过滤器和触发决策。
	default:
		return false
	}

	userMessage := corechat.CleanMessage(e.RawMessage)
	if e.ImageContext != "" {
		if userMessage == "" {
			userMessage = e.ImageContext
		} else {
			userMessage += "\n[图片内容] " + e.ImageContext
		}
	}
	if userMessage == "" {
		userMessage = "（对方没有说话，随便聊点什么吧）"
	}

	if p.logger != nil {
		p.logger.Debug("chat incoming message",
			"type", e.Type,
			"group_id", e.GroupID,
			"user_id", e.UserID,
			"message", userMessage,
		)
	}

	messages := p.svc.BuildMessages(ctx, dm, userMessage)

	var reply string
	var err error
	if p.tools != nil {
		reply, err = p.client.ChatWithTools(ctx, messages, p.tools.Definitions(), p.toolExecutor(e, bot))
	} else {
		reply, err = p.client.Chat(ctx, messages)
	}
	if err != nil {
		if p.logger != nil {
			p.logger.Error("chat llm error", "error", err)
		}
		reply = "（我脑子刚才短路了一下，等会再聊）"
	}
	reply = corechat.PostprocessReply(reply)

	// 如果有多个自然分句，按概率选择“拆成多条发送”，而不是截断丢句。
	parts := corechat.NaturalParts(reply)
	forceSplit := len(parts) >= 2 && rand.Float64() < 0.5

	if !forceSplit {
		truncated, needPolish := corechat.SmartTruncate(reply, 7)
		if needPolish {
			truncated = p.polishReply(ctx, reply)
		}
		reply = truncated
	}

	if p.svc != nil {
		p.svc.RecordReply(ctx, dm, reply)
	}

	if p.logger != nil {
		p.logger.Info("bot reply",
			"type", e.Type,
			"group_id", e.GroupID,
			"user_id", e.UserID,
			"reply", reply,
			"force_split", forceSplit,
		)
	}

	pc.Set("reply", reply)
	pc.Set("force_split", forceSplit)

	if p.svc != nil {
		p.svc.OnBotReplied(dm)
	}
	return true
}

// sendReply 发送回复。forceSplit 为 true 时按自然分句拆成多条全部发送。
func (p *ChatPlugin) sendReply(ctx context.Context, bot plugin.Bot, e *plugin.Event, reply string, forceSplit bool) {
	send := func(text string) {
		if e.Type == plugin.EventGroupMessage {
			_ = bot.SendGroupMessage(ctx, e.GroupID, text)
		} else {
			_ = bot.SendPrivateMessage(ctx, e.UserID, text)
		}
	}

	if forceSplit {
		for _, part := range corechat.NaturalParts(reply) {
			send(part)
		}
		return
	}
	send(reply)
}

// toolExecutor 包装工具注册表，并支持发送图片到当前聊天。
func (p *ChatPlugin) toolExecutor(e *plugin.Event, bot plugin.Bot) llm.ToolExecutor {
	return func(ctx context.Context, name, arguments string) (string, error) {
		if name == "send_meme" {
			if p.memes == nil || p.memes.Count() == 0 {
				return "", fmt.Errorf("meme library is empty")
			}
			path, err := p.memes.Random()
			if err != nil {
				return "", err
			}
			data, err := p.memes.Read(path)
			if err != nil {
				return "", err
			}
			file := "base64://" + base64.StdEncoding.EncodeToString(data)
			if e.Type == plugin.EventGroupMessage {
				if err := bot.SendGroupImage(ctx, e.GroupID, file); err != nil {
					return "", err
				}
			} else {
				if err := bot.SendPrivateImage(ctx, e.UserID, file); err != nil {
					return "", err
				}
			}
			return "表情包已发送", nil
		}
		if name == "send_image" {
			var args struct {
				File string `json:"file"`
			}
			if err := json.Unmarshal([]byte(arguments), &args); err != nil {
				return "", fmt.Errorf("parse send_image arguments: %w", err)
			}
			if args.File == "" {
				return "", fmt.Errorf("send_image requires file")
			}
			if e.Type == plugin.EventGroupMessage {
				if err := bot.SendGroupImage(ctx, e.GroupID, args.File); err != nil {
					return "", err
				}
			} else {
				if err := bot.SendPrivateImage(ctx, e.UserID, args.File); err != nil {
					return "", err
				}
			}
			return "图片已发送", nil
		}
		if p.tools == nil {
			return "", fmt.Errorf("tools registry is nil")
		}
		return p.tools.Execute(ctx, name, arguments)
	}
}

// polishReply 用一次轻量 LLM 调用把回复压缩到 max 字以内，并保持口语化。
func (p *ChatPlugin) polishReply(ctx context.Context, reply string) string {
	if p.client == nil {
		return reply
	}
	messages := []llm.Message{
		{Role: "system", Content: "把这句话压缩到7个字以内，口语化，像懒得打字的人说的，不要解释，不要加装饰符号，直接输出压缩结果。"},
		{Role: "user", Content: reply},
	}
	out, err := p.client.Chat(ctx, messages)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("polish reply failed", "error", err)
		}
		return reply
	}
	out = corechat.PostprocessReply(out)
	out, _ = corechat.SmartTruncate(out, 7)
	return out
}

// 以下薄包装保留给现有测试与外部调用方，核心实现位于 internal/core/chat。

func postprocessReply(s string) string {
	return corechat.PostprocessReply(s)
}

func smartTruncate(s string, max int) (string, bool) {
	return corechat.SmartTruncate(s, max)
}

func naturalParts(reply string) []string {
	return corechat.NaturalParts(reply)
}

func cleanMessage(s string) string {
	return corechat.CleanMessage(s)
}