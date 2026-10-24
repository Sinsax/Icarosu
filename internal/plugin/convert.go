package plugin

import (
	"icarosu/internal/core/domain"
)

// ToDomainMessage 把插件层 Event 转换成 core 使用的领域消息模型。
func ToDomainMessage(e *Event) *domain.ChatMessage {
	if e == nil {
		return &domain.ChatMessage{}
	}

	msgType := domain.MessagePrivate
	if e.Type == EventGroupMessage {
		msgType = domain.MessageGroup
	}

	images := make([]domain.Image, 0, len(e.Images))
	for _, img := range e.Images {
		images = append(images, domain.Image{URL: img.URL, File: img.File})
	}

	return &domain.ChatMessage{
		Type:             msgType,
		GroupID:          e.GroupID,
		UserID:           e.UserID,
		MessageID:        e.MessageID,
		RawMessage:       e.RawMessage,
		SenderName:       e.SenderName,
		AtBot:            e.AtBot,
		Images:           images,
		ReplyToMessageID: e.ReplyToMessageID,
		ReplyToUserID:    e.ReplyToUserID,
		OperatorID:       e.OperatorID,
		ImageContext:     e.ImageContext,
	}
}

// keyForMessage 返回群聊/私聊对应的会话 key。
func keyForMessage(m *domain.ChatMessage) int64 {
	if m.Type == domain.MessageGroup {
		return m.GroupID
	}
	return m.UserID
}