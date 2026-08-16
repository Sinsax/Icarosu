package platform

import (
	"context"

	"icarosu/internal/plugin"
	"icarosu/internal/qq"
)

// Event 当前暂时复用 OneBot 事件类型，后续可扩展为平台中立事件。
type Event = qq.Event

// MessageHandler 是平台消息回调。
type MessageHandler = qq.MessageHandler

// Adapter 是平台接入层抽象。
// 目前由 OneBot 实现，后续可按需扩展其他平台。
type Adapter interface {
	plugin.Bot
	SetMessageHandler(handler MessageHandler)
	Connect(ctx context.Context) error
	Close() error
}
