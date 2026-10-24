package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"icarosu/internal/obs"
	"icarosu/internal/plugin"
	"icarosu/internal/qq"
	"icarosu/internal/ratelimit"
	"icarosu/internal/session"
)

// Handler 负责把 OneBot 原始事件转换为插件系统的事件，并交给 Manager 分发。
type Handler struct {
	bot     plugin.Bot
	manager *plugin.Manager
	selfID  int64
	logger  *slog.Logger

	sessions *session.Manager
	limiter  *ratelimit.Limiter
	metrics  *obs.Metrics
}

// WithRateLimiter 设置全局/按群限流器。
func WithRateLimiter(l *ratelimit.Limiter) func(*Handler) {
	return func(h *Handler) {
		h.limiter = l
	}
}

// WithSessionManager 设置会话管理器。
func WithSessionManager(sm *session.Manager) func(*Handler) {
	return func(h *Handler) {
		h.sessions = sm
	}
}

// WithMetrics 设置运行时指标。
func WithMetrics(m *obs.Metrics) func(*Handler) {
	return func(h *Handler) {
		h.metrics = m
	}
}

func New(bot plugin.Bot, manager *plugin.Manager, selfID int64, logger *slog.Logger, opts ...func(*Handler)) *Handler {
	h := &Handler{
		bot:      bot,
		manager:  manager,
		selfID:   selfID,
		logger:   logger,
		sessions: session.New(0),
		metrics:  obs.NewMetrics(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// lockFor 返回某个会话的串行锁，保证同一个会话内的消息按顺序处理。
func (h *Handler) lockFor(key int64) *session.Session {
	return h.sessions.Get(key)
}

// Handle 处理一个 OneBot 事件。
func (h *Handler) Handle(ctx context.Context, evt qq.Event) {
	if h.metrics != nil {
		h.metrics.IncMessagesReceived()
	}
	switch evt.PostType {
	case "message":
		h.handleMessage(ctx, evt)
	case "meta_event":
		h.handleMeta(ctx, evt)
	case "notice":
		h.handleNotice(ctx, evt)
	default:
		h.logger.Debug("unhandled post_type", "post_type", evt.PostType)
	}
}

func (h *Handler) handleMessage(ctx context.Context, evt qq.Event) {
	switch evt.MessageType {
	case "group":
		e := &plugin.Event{
			Type:             plugin.EventGroupMessage,
			GroupID:          evt.GroupID,
			UserID:           evt.UserID,
			MessageID:        evt.MessageID,
			RawMessage:       evt.RawMessage,
			SenderName:       senderName(evt),
			AtBot:            h.isAtBot(evt),
			Images:           extractImages(evt),
			ReplyToMessageID: extractReplyMessageID(evt),
			ReplyToUserID:    extractReplyUserID(evt),
			Raw:              mustRaw(evt),
		}
		if h.limiter != nil && !h.limiter.Allow(fmt.Sprintf("group:%d", evt.GroupID)) {
			h.logger.Debug("message rate limited", "group_id", evt.GroupID)
			return
		}
		lock := h.lockFor(evt.GroupID)
		lock.Lock()
		h.manager.Dispatch(ctx, h.bot, e)
		lock.Unlock()
	case "private":
		e := &plugin.Event{
			Type:             plugin.EventPrivateMessage,
			UserID:           evt.UserID,
			MessageID:        evt.MessageID,
			RawMessage:       evt.RawMessage,
			SenderName:       senderName(evt),
			Images:           extractImages(evt),
			ReplyToMessageID: extractReplyMessageID(evt),
			ReplyToUserID:    extractReplyUserID(evt),
			Raw:              mustRaw(evt),
		}
		lock := h.lockFor(evt.UserID)
		lock.Lock()
		h.manager.Dispatch(ctx, h.bot, e)
		lock.Unlock()
	default:
		h.logger.Debug("unhandled message_type", "message_type", evt.MessageType)
	}
}

func (h *Handler) handleMeta(ctx context.Context, evt qq.Event) {
	// 当前只记录日志，后续可在这里处理心跳、生命周期等。
	h.logger.Debug("meta event", "sub_type", evt.SubType)
}

func (h *Handler) handleNotice(ctx context.Context, evt qq.Event) {
	if evt.NoticeType == "group_recall" {
		e := &plugin.Event{
			Type:       plugin.EventGroupRecall,
			GroupID:    evt.GroupID,
			UserID:     evt.UserID,
			MessageID:  evt.MessageID,
			OperatorID: evt.OperatorID,
			Raw:        mustRaw(evt),
		}
		h.manager.Dispatch(ctx, h.bot, e)
		return
	}

	e := &plugin.Event{
		Type:    plugin.EventGroupNotice,
		GroupID: evt.GroupID,
		UserID:  evt.UserID,
		Raw:     mustRaw(evt),
	}
	h.manager.Dispatch(ctx, h.bot, e)
}

func senderName(evt qq.Event) string {
	if evt.Sender.Card != "" {
		return evt.Sender.Card
	}
	if evt.Sender.Nickname != "" {
		return evt.Sender.Nickname
	}
	return ""
}

func extractImages(evt qq.Event) []plugin.Image {
	var segments []qq.Segment
	if err := json.Unmarshal(evt.Message, &segments); err != nil {
		return nil
	}

	var images []plugin.Image
	for _, seg := range segments {
		if seg.Type != "image" {
			continue
		}
		img := plugin.Image{}
		if v, ok := seg.Data["url"].(string); ok {
			img.URL = v
		}
		if v, ok := seg.Data["file"].(string); ok {
			img.File = v
		}
		if img.URL != "" || img.File != "" {
			images = append(images, img)
		}
	}
	return images
}

func extractReplyMessageID(evt qq.Event) int64 {
	var segments []qq.Segment
	if err := json.Unmarshal(evt.Message, &segments); err != nil {
		return 0
	}
	for _, seg := range segments {
		if seg.Type != "reply" {
			continue
		}
		return segmentInt64(seg.Data["id"])
	}
	return 0
}

func extractReplyUserID(evt qq.Event) int64 {
	var segments []qq.Segment
	if err := json.Unmarshal(evt.Message, &segments); err != nil {
		return 0
	}
	for _, seg := range segments {
		if seg.Type != "reply" {
			continue
		}
		return segmentInt64(seg.Data["qq"])
	}
	return 0
}

func segmentInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case string:
		var out int64
		_, _ = fmt.Sscanf(n, "%d", &out)
		return out
	}
	return 0
}

// isAtBot 检查消息里是否 @ 了机器人自己。
func (h *Handler) isAtBot(evt qq.Event) bool {
	var segments []qq.Segment
	if err := json.Unmarshal(evt.Message, &segments); err != nil {
		return false
	}

	for _, seg := range segments {
		if seg.Type != "at" {
			continue
		}
		qq, ok := seg.Data["qq"]
		if !ok {
			continue
		}
		switch v := qq.(type) {
		case float64:
			if int64(v) == h.selfID {
				return true
			}
		case string:
			if v == fmt.Sprintf("%d", h.selfID) {
				return true
			}
		}
	}
	return false
}

func mustRaw(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
