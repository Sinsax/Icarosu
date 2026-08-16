package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"regexp"
	"strings"

	"icarosu/internal/attention"
	"icarosu/internal/llm"
	"icarosu/internal/meme"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/store"
	"icarosu/internal/style"
	"icarosu/internal/tools"
)

var (
	cqAtRegex     = regexp.MustCompile(`\[CQ:at(?:,[^\]]*)?\]`)
	bulletRegex   = regexp.MustCompile(`^[-*]\s+`)
	numberedRegex = regexp.MustCompile(`^\d+[.、)]\s*`)
)

// ChatPlugin 是 LLM 对话插件。
// 当前策略：
//   - 私聊消息直接回复
//   - 群聊只有被 @ 时才回复
//
// 支持注册工具，让模型可以调用工具后给出更丰富的内容。
type ChatPlugin struct {
	plugin.BasePlugin
	client       *llm.Client
	systemPrompt string
	tools        *tools.Registry
	mem          *memory.Memory
	memes        *meme.Store
	store        *store.Store
	styleLib     *style.Library
	attention    *attention.Tracker
	logger       *slog.Logger
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
		p.mem = mem
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
		p.styleLib = lib
	}
}

// WithStore 设置数据库存储，用于载入活跃时间/互动/事件等上下文。
func WithStore(st *store.Store) Option {
	return func(p *ChatPlugin) {
		p.store = st
	}
}

// WithAttention 设置注意力系统。
func WithAttention(t *attention.Tracker) Option {
	return func(p *ChatPlugin) {
		p.attention = t
	}
}

// WithLogger 设置插件日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *ChatPlugin) {
		p.logger = logger
	}
}

func New(client *llm.Client, systemPrompt string, opts ...Option) plugin.Plugin {
	p := &ChatPlugin{
		client:       client,
		systemPrompt: systemPrompt,
		logger:       slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
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
	if p.client == nil {
		return false
	}

	switch e.Type {
	case plugin.EventGroupMessage, plugin.EventPrivateMessage:
		// 能走到这里说明已经通过过滤器和触发决策。
	default:
		return false
	}

	userMessage := cqAtRegex.ReplaceAllString(e.RawMessage, "")
	userMessage = strings.TrimSpace(userMessage)
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

	messages := p.buildMessages(e, userMessage)

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
		// 当前先简单返回错误提示，后续可做重试和降级。
		reply = "（我脑子刚才短路了一下，等会再聊）"
	}
	reply = postprocessReply(reply)

	// 如果有多个自然分句，按概率选择“拆成多条发送”，而不是截断丢句。
	parts := naturalParts(reply)
	forceSplit := len(parts) >= 2 && rand.Float64() < 0.5

	if !forceSplit {
		truncated, needPolish := smartTruncate(reply, 7)
		if needPolish {
			truncated = p.polishReply(ctx, reply)
		}
		reply = truncated
	}

	if p.mem != nil {
		p.mem.RecordReply(e, reply)
	}

	p.sendReply(ctx, bot, e, reply, forceSplit)
	if p.attention != nil && e.Type == plugin.EventGroupMessage {
		p.attention.OnBotReplied(e.GroupID)
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
		for _, part := range naturalParts(reply) {
			send(part)
		}
		return
	}
	send(reply)
}

// naturalParts 按自然分句拆分回复，返回多个短句。
func naturalParts(reply string) []string {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}

	var parts []string
	var cur []rune
	flush := func() {
		text := strings.TrimSpace(strings.TrimRight(string(cur), "，、；： "))
		if text != "" {
			parts = append(parts, text)
		}
		cur = nil
	}

	for _, r := range reply {
		cur = append(cur, r)
		if strings.ContainsRune("。！？…\n", r) || strings.ContainsRune("，、；：", r) || r == ' ' {
			flush()
		}
	}
	flush()

	if len(parts) == 0 {
		return []string{reply}
	}
	return parts
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
		return p.tools.Execute(ctx, name, arguments)
	}
}

// detectStyleTags 从最近聊天中粗略识别群聊风格标签。
func (p *ChatPlugin) detectStyleTags(e *plugin.Event) []string {
	if p.mem == nil {
		return nil
	}
	recent := p.mem.Recent(e, 12)
	var tags []string
	addTag := func(t string) {
		for _, x := range tags {
			if x == t {
				return
			}
		}
		tags = append(tags, t)
	}

	for _, entry := range recent {
		text := strings.ToLower(entry.Content)
		switch {
		case strings.Contains(text, "哈哈"), strings.Contains(text, "笑死"), strings.Contains(text, "草"):
			addTag("funny")
			addTag("laugh")
		case strings.Contains(text, "？"), strings.Contains(text, "真的假的"), strings.Contains(text, "细说"):
			addTag("question")
		case strings.Contains(text, "睡"), strings.Contains(text, "晚安"), strings.Contains(text, "夜"), strings.Contains(text, "困"):
			addTag("night")
			addTag("sleep")
		case strings.Contains(text, "吃"), strings.Contains(text, "饭"), strings.Contains(text, "饿"):
			addTag("food")
		case strings.Contains(text, "摸鱼"), strings.Contains(text, "躺"), strings.Contains(text, "懒"):
			addTag("lazy")
		}
	}
	return tags
}

// maybeSimplifyReply 小概率把回复简化成一个很懒的短词/字。
func (p *ChatPlugin) maybeSimplifyReply(reply string, tags []string) (string, bool) {
	if p.styleLib == nil {
		return reply, false
	}
	if rand.Float64() > 0.12 {
		return reply, false
	}

	candidates := []string{"确实", "？", "哈哈", "行吧", "啊？", "好", "嗯", "对", "没吃"}
	if hasTag(tags, "funny") {
		candidates = append(candidates, "哈哈", "笑死")
	}
	if hasTag(tags, "question") {
		candidates = append(candidates, "？", "真的假的")
	}
	if hasTag(tags, "food") {
		candidates = append(candidates, "没吃", "行吧", "确实")
	}
	if hasTag(tags, "lazy") {
		candidates = append(candidates, "行吧", "嗯", "懒得动")
	}
	return candidates[rand.Intn(len(candidates))], true
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

// groupDataContext 把已收集的活跃时间/互动/事件作为上下文载入。
func (p *ChatPlugin) groupDataContext(e *plugin.Event) string {
	var hints []string

	if hours, err := p.store.GetUserActiveHours(e.GroupID, e.UserID, 3); err == nil && len(hours) > 0 {
		var parts []string
		for _, h := range hours {
			parts = append(parts, fmt.Sprintf("周%d %d点", h.Weekday, h.Hour))
		}
		hints = append(hints, e.SenderName+"常活跃: "+strings.Join(parts, "、"))
	}

	if interactions, err := p.store.GetTopInteractions(e.GroupID, 3); err == nil && len(interactions) > 0 {
		var parts []string
		for _, it := range interactions {
			parts = append(parts, fmt.Sprintf("%d↔%d x%d", it.FromUserID, it.ToUserID, it.Count))
		}
		hints = append(hints, "群内互动: "+strings.Join(parts, "、"))
	}

	if events, err := p.store.GetEventsByUser(e.GroupID, e.UserID, 3); err == nil && len(events) > 0 {
		var parts []string
		for _, ev := range events {
			parts = append(parts, ev.Title)
		}
		hints = append(hints, "相关事件: "+strings.Join(parts, "、"))
	}

	if len(hints) == 0 {
		return ""
	}
	return "群数据参考：\n" + strings.Join(hints, "\n")
}

// buildMessages 组装发给 LLM 的消息，包含系统人设、长期记忆和最近群聊上下文。
func (p *ChatPlugin) buildMessages(e *plugin.Event, currentMessage string) []llm.Message {
	messages := []llm.Message{
		{Role: "system", Content: p.systemPrompt},
	}

	tags := p.detectStyleTags(e)
	if len(tags) > 0 {
		hint := "当前群聊风格标签：" + strings.Join(tags, ",") + "。请贴合这种氛围，不要客气，不要完整收尾。"
		messages = append(messages, llm.Message{Role: "system", Content: hint})

		if p.styleLib != nil {
			if samples := p.styleLib.SampleByTags(tags, 3); len(samples) > 0 {
				messages = append(messages, llm.Message{Role: "system", Content: "参考表达方式：\n" + strings.Join(samples, "\n")})
			}
		}
	}

	if p.mem == nil {
		messages = append(messages, llm.Message{Role: "user", Content: currentMessage})
		return messages
	}

	if e.Type == plugin.EventGroupMessage {
		if memories := p.mem.LongTermMemories(e.GroupID, 5); len(memories) > 0 {
			memoryText := "你对这个群的长期记忆：\n" + strings.Join(memories, "\n")
			messages = append(messages, llm.Message{Role: "system", Content: memoryText})
		}

		if p.store != nil {
			if dataText := p.groupDataContext(e); dataText != "" {
				messages = append(messages, llm.Message{Role: "system", Content: dataText})
			}
		}
	}

	recent := p.mem.Recent(e, 12)
	if len(recent) == 0 {
		messages = append(messages, llm.Message{Role: "user", Content: currentMessage})
		return messages
	}

	for _, entry := range recent {
		content := cleanMessage(entry.Content)
		if content == "" {
			continue
		}
		switch entry.Role {
		case "assistant":
			messages = append(messages, llm.Message{Role: "assistant", Content: content})
		default:
			if e.Type == plugin.EventGroupMessage && entry.SenderName != "" {
				content = entry.SenderName + ": " + content
			}
			messages = append(messages, llm.Message{Role: "user", Content: content})
		}
	}

	return messages
}

func cleanMessage(s string) string {
	s = cqAtRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

var aiPhraseReplacer = strings.NewReplacer(
	"作为一名AI", "我",
	"作为一个AI助手", "我",
	"作为一个AI", "我",
	"作为AI", "我",
	"祝你", "",
	"希望你能", "",
	"如果还有其他问题", "",
	"随时可以问我", "",
	"总的来说", "",
	"综上所述", "",
)

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
	out = postprocessReply(out)
	out, _ = smartTruncate(out, 7)
	return out
}

// smartTruncate 智能截断：
//   - 长度未超限：直接返回，不需要二次润色
//   - 超限时优先在句末标点、其次在逗号/顿号处截断
//   - 如果 max 字以内没有任何自然断点，返回 true 表示需要二次 LLM 润色
func smartTruncate(s string, max int) (string, bool) {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}

	// 优先找 max 之前的句末标点
	for i := max - 1; i >= 0; i-- {
		if isSentenceEnd(runes[i]) {
			return strings.TrimSpace(string(runes[:i+1])), false
		}
	}
	// 其次找 max 之前的逗号/顿号/分号/冒号
	for i := max - 1; i >= 0; i-- {
		if isClauseBreak(runes[i]) {
			return strings.TrimSpace(string(runes[:i])), false
		}
	}

	// 没有任何自然断点：不要硬切，交给二次 LLM 润色
	return strings.TrimSpace(string(runes[:max])), true
}

func isSentenceEnd(r rune) bool {
	return strings.ContainsRune("。！？…\n", r)
}

func isClauseBreak(r rune) bool {
	return strings.ContainsRune("，、；：,;: ", r)
}

// postprocessReply 做轻量拟人化清理，减少“机器感”。
func postprocessReply(s string) string {
	s = strings.TrimSpace(s)
	s = aiPhraseReplacer.Replace(s)
	s = strings.TrimSpace(s)

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		line = bulletRegex.ReplaceAllString(line, "")
		line = numberedRegex.ReplaceAllString(line, "")
		lines[i] = line
	}
	s = strings.Join(lines, "\n")
	s = strings.TrimSpace(s)

	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
