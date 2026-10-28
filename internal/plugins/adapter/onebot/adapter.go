package onebot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"icarosu/internal/qq"
	"icarosu/internal/sdk"
)

// Client 是 adapter 需要的最小 OneBot 客户端能力。
type Client interface {
	SetMessageHandler(qq.MessageHandler)
	Connect(ctx context.Context) error
	Close() error
	SendGroupRaw(ctx context.Context, groupID int64, message any) error
	SendPrivateRaw(ctx context.Context, userID int64, message any) error
	GetGroupMemberList(ctx context.Context, groupID int64) ([]qq.GroupMember, error)
}

// Adapter 把 OneBot v11 事件/动作翻译成 sdk.Event/sdk.Action。
type Adapter struct {
	client Client
	logger *slog.Logger
	emit   sdk.EmitFunc
	admins map[int64]bool
}

func New(client Client, logger *slog.Logger) *Adapter {
	return NewWithAdmins(client, nil, logger)
}

func NewWithAdmins(client Client, adminIDs []int64, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	admins := make(map[int64]bool, len(adminIDs))
	for _, id := range adminIDs {
		admins[id] = true
	}
	return &Adapter{client: client, logger: logger, admins: admins}
}

func (a *Adapter) ID() string { return "onebot" }

func (a *Adapter) Start(ctx context.Context, emit sdk.EmitFunc) error {
	a.emit = emit
	a.client.SetMessageHandler(func(ctx context.Context, evt qq.Event) {
		out, ok := NormalizeWithAdmins(evt, a.admins)
		if !ok {
			return
		}
		if a.emit != nil {
			a.emit(ctx, out)
		}
	})
	return a.client.Connect(ctx)
}

func (a *Adapter) Close() error { return a.client.Close() }

func (a *Adapter) Capabilities() []string {
	return []string{"text", "image", "reply", "mention", "member_list"}
}

// ListMembers 实现 sdk.MemberDirectory。
func (a *Adapter) ListMembers(ctx context.Context, conversation sdk.Conversation) ([]sdk.Member, error) {
	if conversation.Kind != "group" {
		return nil, fmt.Errorf("member list requires a group conversation")
	}
	groupID, err := nativeConversationID(conversation)
	if err != nil {
		return nil, err
	}
	members, err := a.client.GetGroupMemberList(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.Member, 0, len(members))
	for _, m := range members {
		name := m.Card
		if name == "" {
			name = m.Nickname
		}
		out = append(out, sdk.Member{
			ID:   sdk.ActorID(fmt.Sprintf("qq:user:%d", m.UserID)),
			Name: name,
			Card: m.Card,
			Role: m.Role,
		})
	}
	return out, nil
}

var _ sdk.MemberDirectory = (*Adapter)(nil)

func (a *Adapter) Send(ctx context.Context, action sdk.Action) error {
	if action.Silent {
		return nil
	}
	segments, err := segmentsForAction(action)
	if err != nil {
		return err
	}
	nativeID, err := nativeConversationID(action.Conversation)
	if err != nil {
		return err
	}
	switch action.Conversation.Kind {
	case "group":
		return a.client.SendGroupRaw(ctx, nativeID, segments)
	case "private":
		return a.client.SendPrivateRaw(ctx, nativeID, segments)
	default:
		return fmt.Errorf("unsupported conversation kind %q", action.Conversation.Kind)
	}
}

func nativeConversationID(c sdk.Conversation) (int64, error) {
	if v := c.Metadata["native_id"]; v != "" {
		return strconv.ParseInt(v, 10, 64)
	}
	parts := strings.Split(string(c.ID), ":")
	if len(parts) == 0 {
		return 0, fmt.Errorf("conversation id %q has no native id", c.ID)
	}
	return strconv.ParseInt(parts[len(parts)-1], 10, 64)
}

func segmentsForAction(action sdk.Action) ([]map[string]any, error) {
	var segments []map[string]any
	if action.ReplyTo != "" {
		segments = append(segments, map[string]any{
			"type": "reply",
			"data": map[string]any{"id": action.ReplyTo},
		})
	}
	for _, part := range action.Parts {
		switch part.Type {
		case sdk.PartText:
			if part.Text == "" {
				continue
			}
			segments = append(segments, map[string]any{
				"type": "text",
				"data": map[string]any{"text": part.Text},
			})
		case sdk.PartImage:
			file := part.URL
			if file == "" {
				file = part.Meta["file"]
			}
			if file == "" {
				return nil, fmt.Errorf("image part has no url/file")
			}
			segments = append(segments, map[string]any{
				"type": "image",
				"data": map[string]any{"file": file},
			})
		case sdk.PartMention:
			qqID := part.Meta["qq"]
			if qqID == "" {
				qqID = part.Text
			}
			if qqID == "" {
				continue
			}
			segments = append(segments, map[string]any{
				"type": "at",
				"data": map[string]any{"qq": qqID},
			})
		case sdk.PartReply:
			id := part.Meta["id"]
			if id == "" {
				id = part.Text
			}
			if id != "" {
				segments = append(segments, map[string]any{
					"type": "reply",
					"data": map[string]any{"id": id},
				})
			}
		}
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("action has no sendable parts")
	}
	return segments, nil
}

// Normalize 把 OneBot 事件转换成平台中立的 sdk.Event。
func Normalize(evt qq.Event) (sdk.Event, bool) {
	return NormalizeWithAdmins(evt, nil)
}

// NormalizeWithAdmins 与 Normalize 相同，但会按管理员列表标记 Actor.IsAdmin。
func NormalizeWithAdmins(evt qq.Event, admins map[int64]bool) (sdk.Event, bool) {
	raw, _ := json.Marshal(evt)
	switch evt.PostType {
	case "message":
		if evt.MessageType != "group" && evt.MessageType != "private" {
			return sdk.Event{}, false
		}
		if evt.SelfID != 0 && evt.UserID == evt.SelfID {
			return sdk.Event{}, false
		}
		conv := conversationFor(evt)
		actor := actorFor(evt.UserID, evt.Sender, admins)
		parts, mentions, replyTo := parseMessage(evt.Message, evt.RawMessage)
		kind := "message.created"
		scope := evt.GroupID
		if evt.MessageType == "private" {
			scope = evt.UserID
		}
		messageID := evt.MessageID
		if messageID == 0 {
			messageID = time.Now().UnixNano()
		}
		id := fmt.Sprintf("onebot:%s:%d:%d", kind, scope, messageID)
		return sdk.Event{
			ID:           sdk.EventID(id),
			Kind:         kind,
			OccurredAt:   time.Now(),
			Source:       sdk.SourceRef{AdapterID: "onebot", Platform: "qq"},
			Conversation: conv,
			Actor:        actor,
			Self:         sdk.ActorID(fmt.Sprintf("qq:user:%d", evt.SelfID)),
			Parts:        parts,
			ReplyTo:      replyTo,
			Mentions:     mentions,
			Meta: map[string]string{
				"native_message_id": strconv.FormatInt(evt.MessageID, 10),
				"native_group_id":   strconv.FormatInt(evt.GroupID, 10),
				"native_user_id":    strconv.FormatInt(evt.UserID, 10),
				"native_self_id":    strconv.FormatInt(evt.SelfID, 10),
				"native_sender":     evt.Sender.Nickname,
			},
			Raw: raw,
		}, true
	case "notice":
		conv := conversationFor(evt)
		actor := actorFor(evt.UserID, evt.Sender, admins)
		kind := "notice.received"
		if evt.NoticeType == "group_recall" {
			kind = "notice.recalled"
		}
		messageID := evt.MessageID
		if messageID == 0 {
			messageID = time.Now().UnixNano()
		}
		id := fmt.Sprintf("onebot:%s:%d:%d:%d", kind, evt.GroupID, evt.UserID, messageID)
		return sdk.Event{
			ID:           sdk.EventID(id),
			Kind:         kind,
			OccurredAt:   time.Now(),
			Source:       sdk.SourceRef{AdapterID: "onebot", Platform: "qq"},
			Conversation: conv,
			Actor:        actor,
			Self:         sdk.ActorID(fmt.Sprintf("qq:user:%d", evt.SelfID)),
			Parts:        nil,
			Meta: map[string]string{
				"native_message_id":  strconv.FormatInt(evt.MessageID, 10),
				"native_group_id":    strconv.FormatInt(evt.GroupID, 10),
				"native_user_id":     strconv.FormatInt(evt.UserID, 10),
				"native_self_id":     strconv.FormatInt(evt.SelfID, 10),
				"native_operator_id": strconv.FormatInt(evt.OperatorID, 10),
				"notice_type":        evt.NoticeType,
			},
			Raw: raw,
		}, true
	default:
		return sdk.Event{}, false
	}
}

func conversationFor(evt qq.Event) sdk.Conversation {
	if evt.GroupID != 0 {
		return sdk.Conversation{
			ID:        sdk.ConversationID(fmt.Sprintf("qq:group:%d", evt.GroupID)),
			AdapterID: "onebot",
			Kind:      "group",
			Metadata:  map[string]string{"native_id": strconv.FormatInt(evt.GroupID, 10)},
		}
	}
	return sdk.Conversation{
		ID:        sdk.ConversationID(fmt.Sprintf("qq:user:%d", evt.UserID)),
		AdapterID: "onebot",
		Kind:      "private",
		Metadata:  map[string]string{"native_id": strconv.FormatInt(evt.UserID, 10)},
	}
}

func actorFor(userID int64, sender qq.Sender, admins map[int64]bool) sdk.Actor {
	name := sender.Card
	if name == "" {
		name = sender.Nickname
	}
	return sdk.Actor{
		ID:      sdk.ActorID(fmt.Sprintf("qq:user:%d", userID)),
		Name:    name,
		Card:    sender.Card,
		IsAdmin: admins[userID],
	}
}

// segmentID 兼容 OneBot 的 int64、float64、json.Number 和字符串形式。
// JSON 数字在 map[string]any 中默认是 float64；直接 fmt.Sprint 会把
// 9 位以上 QQ 号变成科学计数法，导致 @ 判定失效。
func segmentID(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), n == float64(int64(n))
	case json.Number:
		id, err := n.Int64()
		return id, err == nil
	case string:
		id, err := strconv.ParseInt(n, 10, 64)
		return id, err == nil
	default:
		return 0, false
	}
}

func parseMessage(raw json.RawMessage, fallback string) (parts []sdk.Part, mentions []sdk.ActorID, replyTo string) {
	var segments []qq.Segment
	if err := json.Unmarshal(raw, &segments); err != nil || len(segments) == 0 {
		if fallback != "" {
			return []sdk.Part{{Type: sdk.PartText, Text: fallback}}, nil, ""
		}
		return nil, nil, ""
	}
	for _, seg := range segments {
		switch seg.Type {
		case "text":
			if s, ok := seg.Data["text"].(string); ok && s != "" {
				parts = append(parts, sdk.Part{Type: sdk.PartText, Text: s})
			}
		case "image":
			url, _ := seg.Data["url"].(string)
			file, _ := seg.Data["file"].(string)
			parts = append(parts, sdk.Part{
				Type: sdk.PartImage,
				URL:  url,
				Meta: map[string]string{"file": file},
			})
		case "at":
			qqID, ok := segmentID(seg.Data["qq"])
			if !ok {
				continue
			}
			id := strconv.FormatInt(qqID, 10)
			parts = append(parts, sdk.Part{Type: sdk.PartMention, Meta: map[string]string{"qq": id}})
			mentions = append(mentions, sdk.ActorID("qq:user:"+id))
		case "reply":
			id := ""
			if n, ok := segmentID(seg.Data["id"]); ok {
				id = strconv.FormatInt(n, 10)
			} else if s, ok := seg.Data["id"].(string); ok {
				id = s
			}
			if id == "" {
				continue
			}
			replyTo = id
			parts = append(parts, sdk.Part{Type: sdk.PartReply, Meta: map[string]string{"id": id}})
		}
	}
	return parts, mentions, replyTo
}
