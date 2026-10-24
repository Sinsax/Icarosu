// Package chat 提供聊天回复的核心业务逻辑。
// 这里定义核心服务所需的最小接口，并且只依赖 core/domain 领域模型。
package chat

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"icarosu/internal/core/domain"
	"icarosu/internal/llm"
	"icarosu/internal/memory"
	"icarosu/internal/store"
)

var (
	cqAtRegex     = regexp.MustCompile(`\[CQ:at(?:,[^\]]*)?\]`)
	bulletRegex   = regexp.MustCompile(`^[-*]\s+`)
	numberedRegex = regexp.MustCompile(`^\d+[.、)]\s*`)
)

// GroupScope 是群维度隔离与互通组解析能力。
type GroupScope interface {
	MemoryGroupIDs(groupID int64) []int64
	ExpressionGroupIDs(groupID int64) []int64
}

// JargonProvider 是群黑话查询能力。
type JargonProvider interface {
	LoadJargonsByGroup(groupID int64, limit int) ([]store.JargonRecord, error)
}

// ProfileProvider 是群用户画像查询能力。
type ProfileProvider interface {
	LoadUserProfiles(groupID int64, limit int) ([]store.UserProfileRecord, error)
}

// Memory 是聊天核心依赖的短期/长期记忆能力。
type Memory interface {
	Recent(m *domain.ChatMessage, limit int) []memory.Entry
	LongTermMemoriesForGroups(groupIDs []int64, limit int) []string
	RecordReply(m *domain.ChatMessage, reply string)
}

// SemanticSearcher 是语义记忆检索能力。
type SemanticSearcher interface {
	Search(ctx context.Context, groupID int64, query string, limit int) ([]store.MemoryRecord, error)
}

// TopicTracker 是话题一致性跟踪能力。
type TopicTracker interface {
	Hint(ctx context.Context, key int64, currentMessage string) string
	RecordReply(ctx context.Context, key int64, reply string)
}

// AttentionTracker 是注意力系统能力。
type AttentionTracker interface {
	OnBotReplied(groupID int64)
}

// StyleLibrary 是表达方式库能力。
type StyleLibrary interface {
	SampleByTagsForGroup(groupID int64, tags []string, limit int) []string
}

// Service 是聊天回复核心服务，负责构造送入 LLM 的上下文消息和对回复做拟人化后处理。
type Service struct {
	systemPrompt string
	mem          Memory
	store        store.ChatContextStore
	semantic     SemanticSearcher
	styleLib     StyleLibrary
	attention    AttentionTracker
	topic        TopicTracker
	scope        GroupScope
	jargons      JargonProvider
	profiles     ProfileProvider
	logger       *slog.Logger
}

// Option 用于配置 Service。
type Option func(*Service)

// WithMemory 设置短期/长期记忆服务。
func WithMemory(mem Memory) Option {
	return func(s *Service) {
		s.mem = mem
	}
}

// WithStyleLibrary 设置表达方式库。
func WithStyleLibrary(lib StyleLibrary) Option {
	return func(s *Service) {
		s.styleLib = lib
	}
}

// WithStore 设置数据库存储，用于载入活跃时间/互动/事件等上下文。
func WithStore(st store.ChatContextStore) Option {
	return func(s *Service) {
		s.store = st
	}
}

// WithSemantic 设置语义记忆服务。
func WithSemantic(svc SemanticSearcher) Option {
	return func(s *Service) {
		s.semantic = svc
	}
}

// WithTopicTracker 设置话题稳定性跟踪器。
func WithTopicTracker(t TopicTracker) Option {
	return func(s *Service) {
		s.topic = t
	}
}

// WithAttention 设置注意力系统。
func WithAttention(t AttentionTracker) Option {
	return func(s *Service) {
		s.attention = t
	}
}

// WithGroupScope 设置群维度隔离与互通组解析器。
func WithGroupScope(sc GroupScope) Option {
	return func(s *Service) {
		s.scope = sc
	}
}

// WithJargons 设置群黑话查询能力。
func WithJargons(p JargonProvider) Option {
	return func(s *Service) {
		s.jargons = p
	}
}

// WithProfiles 设置群用户画像查询能力。
func WithProfiles(p ProfileProvider) Option {
	return func(s *Service) {
		s.profiles = p
	}
}

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// New 创建聊天核心服务。
func New(systemPrompt string, opts ...Option) *Service {
	s := &Service{
		systemPrompt: systemPrompt,
		logger:       slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// UpdatePersona 热更新系统人设提示词。
func (s *Service) UpdatePersona(prompt string) {
	if prompt == "" {
		return
	}
	s.systemPrompt = prompt
}

// detectStyleTags 从最近聊天中粗略识别群聊风格标签。
func (s *Service) detectStyleTags(m *domain.ChatMessage) []string {
	if s.mem == nil {
		return nil
	}
	recent := s.mem.Recent(m, 12)
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

// semanticMemories 优先按当前消息语义检索长期记忆；没有向量结果时退回最近长期记忆。
// groupIDs 支持互通组，默认只包含当前群。
func (s *Service) semanticMemories(ctx context.Context, groupIDs []int64, query string) string {
	if s.semantic != nil {
		var contents []string
		for _, groupID := range groupIDs {
			records, err := s.semantic.Search(ctx, groupID, query, 5)
			if err != nil {
				continue
			}
			for _, r := range records {
				contents = append(contents, r.Content)
			}
		}
		if len(contents) > 0 {
			return "你对这些群的长期记忆：\n" + strings.Join(contents, "\n")
		}
	}

	if s.mem == nil {
		return ""
	}
	memories := s.mem.LongTermMemoriesForGroups(groupIDs, 5)
	if len(memories) == 0 {
		return ""
	}
	return "你对这些群的长期记忆：\n" + strings.Join(memories, "\n")
}

// groupDataContext 把已收集的活跃时间/互动/事件作为上下文载入。
func (s *Service) groupDataContext(m *domain.ChatMessage) string {
	var hints []string

	if s.store != nil {
		if hours, err := s.store.GetUserActiveHours(m.GroupID, m.UserID, 3); err == nil && len(hours) > 0 {
			var parts []string
			for _, h := range hours {
				parts = append(parts, fmt.Sprintf("周%d %d点", h.Weekday, h.Hour))
			}
			hints = append(hints, m.SenderName+"常活跃: "+strings.Join(parts, "、"))
		}

		if interactions, err := s.store.GetTopInteractions(m.GroupID, 3); err == nil && len(interactions) > 0 {
			var parts []string
			for _, it := range interactions {
				parts = append(parts, fmt.Sprintf("%d↔%d x%d", it.FromUserID, it.ToUserID, it.Count))
			}
			hints = append(hints, "群内互动: "+strings.Join(parts, "、"))
		}

		if events, err := s.store.GetEventsByUser(m.GroupID, m.UserID, 3); err == nil && len(events) > 0 {
			var parts []string
			for _, ev := range events {
				parts = append(parts, ev.Title)
			}
			hints = append(hints, "相关事件: "+strings.Join(parts, "、"))
		}
	}

	if len(hints) == 0 {
		return ""
	}
	return "群数据参考：\n" + strings.Join(hints, "\n")
}

// groupJargonContext 返回当前群可见的黑话列表。
func (s *Service) groupJargonContext(m *domain.ChatMessage) string {
	if s.jargons == nil {
		return ""
	}
	groupIDs := []int64{m.GroupID}
	if s.scope != nil {
		groupIDs = s.scope.ExpressionGroupIDs(m.GroupID)
	}
	var lines []string
	for _, gid := range groupIDs {
		records, err := s.jargons.LoadJargonsByGroup(gid, 20)
		if err != nil {
			continue
		}
		for _, r := range records {
			lines = append(lines, fmt.Sprintf("- %s = %s", r.Term, r.Meaning))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "群内黑话/梗：\n" + strings.Join(lines, "\n")
}

// groupProfileContext 返回当前群可见的用户画像。
func (s *Service) groupProfileContext(m *domain.ChatMessage) string {
	if s.profiles == nil {
		return ""
	}
	groupIDs := []int64{m.GroupID}
	if s.scope != nil {
		groupIDs = s.scope.MemoryGroupIDs(m.GroupID)
	}
	var lines []string
	for _, gid := range groupIDs {
		records, err := s.profiles.LoadUserProfiles(gid, 20)
		if err != nil {
			continue
		}
		for _, r := range records {
			lines = append(lines, fmt.Sprintf("- %s（%d）：发言 %d 次", r.Nickname, r.UserID, r.MessageCount))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "群友画像：\n" + strings.Join(lines, "\n")
}

// BuildMessages 组装发给 LLM 的消息，包含系统人设、长期记忆和最近群聊上下文。
func (s *Service) BuildMessages(ctx context.Context, m *domain.ChatMessage, currentMessage string) []llm.Message {
	return buildMessages(ctx, s, m, currentMessage)
}

// RecordReply 记录机器人的回复到短期记忆和话题状态。
func (s *Service) RecordReply(ctx context.Context, m *domain.ChatMessage, reply string) {
	key := keyFor(m)
	if s.topic != nil {
		s.topic.RecordReply(ctx, key, reply)
	}

	if s.mem != nil {
		s.mem.RecordReply(m, reply)
	}
}

// OnBotReplied 在机器人回复后更新注意力。
func (s *Service) OnBotReplied(m *domain.ChatMessage) {
	if s.attention != nil && m.Type == domain.MessageGroup {
		s.attention.OnBotReplied(m.GroupID)
	}
}

// CleanMessage 清理 CQ 码并去除首尾空白。
func CleanMessage(s string) string {
	return strings.TrimSpace(cqAtRegex.ReplaceAllString(s, ""))
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

// PostprocessReply 做轻量拟人化清理，减少“机器感”。
func PostprocessReply(s string) string {
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

// SmartTruncate 智能截断：
//   - 长度未超限：直接返回，不需要二次润色
//   - 超限时优先在句末标点、其次在逗号/顿号处截断
//   - 如果 max 字以内没有任何自然断点，返回 true 表示需要二次 LLM 润色
func SmartTruncate(s string, max int) (string, bool) {
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

// NaturalParts 按自然分句拆分回复，返回多个短句。
func NaturalParts(reply string) []string {
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

func isSentenceEnd(r rune) bool {
	return strings.ContainsRune("。！？…\n", r)
}

func isClauseBreak(r rune) bool {
	return strings.ContainsRune("，、；：,;: ", r)
}

func keyFor(m *domain.ChatMessage) int64 {
	if m.Type == domain.MessageGroup {
		return m.GroupID
	}
	return m.UserID
}

// buildMessages 是 BuildMessages 的内部实现。
func buildMessages(ctx context.Context, s *Service, m *domain.ChatMessage, currentMessage string) []llm.Message {
	messages := []llm.Message{
		{Role: "system", Content: s.systemPrompt},
	}

	tags := s.detectStyleTags(m)
	if len(tags) > 0 {
		hint := "当前群聊风格标签：" + strings.Join(tags, ",") + "。请贴合这种氛围，不要客气，不要完整收尾。"
		messages = append(messages, llm.Message{Role: "system", Content: hint})

		if s.styleLib != nil {
			if samples := s.styleLib.SampleByTagsForGroup(m.GroupID, tags, 3); len(samples) > 0 {
				messages = append(messages, llm.Message{Role: "system", Content: "参考表达方式：\n" + strings.Join(samples, "\n")})
			}
		}
	}

	if s.topic != nil {
		key := keyFor(m)
		if hint := s.topic.Hint(ctx, key, currentMessage); hint != "" {
			messages = append(messages, llm.Message{Role: "system", Content: hint})
		}
	}

	if s.mem == nil {
		messages = append(messages, llm.Message{Role: "user", Content: currentMessage})
		return messages
	}

	if m.Type == domain.MessageGroup {
		memoryGroupIDs := []int64{m.GroupID}
		if s.scope != nil {
			memoryGroupIDs = s.scope.MemoryGroupIDs(m.GroupID)
		}
		memoryText := s.semanticMemories(ctx, memoryGroupIDs, currentMessage)
		if memoryText != "" {
			messages = append(messages, llm.Message{Role: "system", Content: memoryText})
		}

		if s.store != nil {
			if dataText := s.groupDataContext(m); dataText != "" {
				messages = append(messages, llm.Message{Role: "system", Content: dataText})
			}
		}

		if text := s.groupJargonContext(m); text != "" {
			messages = append(messages, llm.Message{Role: "system", Content: text})
		}
		if text := s.groupProfileContext(m); text != "" {
			messages = append(messages, llm.Message{Role: "system", Content: text})
		}
	}

	recent := s.mem.Recent(m, 12)
	if len(recent) == 0 {
		messages = append(messages, llm.Message{Role: "user", Content: currentMessage})
		return messages
	}

	for _, entry := range recent {
		content := CleanMessage(entry.Content)
		if content == "" {
			continue
		}
		switch entry.Role {
		case "assistant":
			messages = append(messages, llm.Message{Role: "assistant", Content: content})
		default:
			if m.Type == domain.MessageGroup && entry.SenderName != "" {
				content = entry.SenderName + ": " + content
			}
			messages = append(messages, llm.Message{Role: "user", Content: content})
		}
	}

	return messages
}
