package bridge

import (
	"context"
	"strconv"
	"strings"

	"icarosu/internal/plugin"
	"icarosu/internal/sdk"
)

// ToLegacyEvent 把 sdk.Event 转成旧 plugin.Event，供迁移期旧流水线使用。
func ToLegacyEvent(ev sdk.Event) (*plugin.Event, bool) {
	groupID := nativeID(ev.Meta["native_group_id"], ev.Conversation.ID, "qq:group:")
	userID := nativeID(ev.Meta["native_user_id"], ev.Conversation.ID, "qq:user:")
	messageID := parseInt(ev.Meta["native_message_id"])

	switch ev.Kind {
	case "message.created":
		typ := plugin.EventPrivateMessage
		if ev.Conversation.Kind == "group" {
			typ = plugin.EventGroupMessage
		}
		var text []string
		var images []plugin.Image
		for _, p := range ev.Parts {
			switch p.Type {
			case sdk.PartText:
				text = append(text, p.Text)
			case sdk.PartImage:
				images = append(images, plugin.Image{URL: p.URL, File: p.Meta["file"]})
			}
		}
		atBot := false
		for _, m := range ev.Mentions {
			if m == ev.Self {
				atBot = true
			}
		}
		return &plugin.Event{
			Type:             typ,
			GroupID:          groupID,
			UserID:           userID,
			MessageID:        messageID,
			RawMessage:       strings.Join(text, ""),
			SenderName:       ev.Actor.Name,
			AtBot:            atBot,
			Images:           images,
			ReplyToMessageID: parseInt(ev.ReplyTo),
			Raw:              ev.Raw,
		}, true
	case "notice.recalled":
		return &plugin.Event{
			Type:       plugin.EventGroupRecall,
			GroupID:    groupID,
			UserID:     userID,
			MessageID:  messageID,
			OperatorID: parseInt(ev.Meta["native_operator_id"]),
			Raw:        ev.Raw,
		}, true
	case "notice.received":
		return &plugin.Event{
			Type:    plugin.EventGroupNotice,
			GroupID: groupID,
			UserID:  userID,
			Raw:     ev.Raw,
		}, true
	default:
		return nil, false
	}
}

// Dispatch 把 sdk.Event 翻译后交给旧 Manager；迁移期使用。
func Dispatch(ctx context.Context, manager *plugin.Manager, bot plugin.Bot, ev sdk.Event) bool {
	legacy, ok := ToLegacyEvent(ev)
	if !ok {
		return false
	}
	manager.Dispatch(ctx, bot, legacy)
	return true
}

func nativeID(meta string, conversationID sdk.ConversationID, prefix string) int64 {
	if meta != "" {
		return parseInt(meta)
	}
	return parseInt(strings.TrimPrefix(string(conversationID), prefix))
}

func parseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
