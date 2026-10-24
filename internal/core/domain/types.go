// Package domain 定义核心业务领域模型，避免 core 依赖具体插件/平台事件结构。
package domain

// MessageType 表示核心层关心的消息类型。
type MessageType string

const (
	// MessageGroup 群聊消息。
	MessageGroup MessageType = "group"
	// MessagePrivate 私聊消息。
	MessagePrivate MessageType = "private"
)

// Image 是消息中的图片信息。
type Image struct {
	URL  string
	File string
}

// ChatMessage 是核心层使用的聊天消息模型。
// 插件层负责把平台 Event 转换成该模型。
type ChatMessage struct {
	Type             MessageType
	GroupID          int64
	UserID           int64
	MessageID        int64
	RawMessage       string
	SenderName       string
	AtBot            bool
	Images           []Image
	ReplyToMessageID int64
	ReplyToUserID    int64
	OperatorID       int64
	ImageContext     string
}