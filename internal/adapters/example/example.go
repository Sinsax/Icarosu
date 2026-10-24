// Package example 提供一个最小平台适配器示例，展示如何接入非 OneBot 平台。
package example

import (
	"context"
	"log/slog"

	"icarosu/internal/platform"
	"icarosu/internal/qq"
)

// Adapter 是一个空实现平台适配器，只记录日志，不真正连接任何平台。
// 可作为新平台适配器的起点。
type Adapter struct {
	logger *slog.Logger
}

// New 创建示例适配器。
func New(logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{logger: logger}
}

// SetMessageHandler 保存消息回调（示例中仅记录）。
func (a *Adapter) SetMessageHandler(qq.MessageHandler) {
	a.logger.Info("example adapter SetMessageHandler called")
}

// Connect 模拟建立连接。
func (a *Adapter) Connect(ctx context.Context) error {
	a.logger.Info("example adapter connected")
	<-ctx.Done()
	return nil
}

// Close 关闭连接。
func (a *Adapter) Close() error {
	a.logger.Info("example adapter closed")
	return nil
}

// SendGroupMessage 示例发送群消息。
func (a *Adapter) SendGroupMessage(_ context.Context, groupID int64, message string) error {
	a.logger.Info("example send group message", "group_id", groupID, "message", message)
	return nil
}

// SendPrivateMessage 示例发送私聊消息。
func (a *Adapter) SendPrivateMessage(_ context.Context, userID int64, message string) error {
	a.logger.Info("example send private message", "user_id", userID, "message", message)
	return nil
}

// SendGroupImage 示例发送群图片。
func (a *Adapter) SendGroupImage(_ context.Context, groupID int64, file string) error {
	a.logger.Info("example send group image", "group_id", groupID, "file", file)
	return nil
}

// SendPrivateImage 示例发送私聊图片。
func (a *Adapter) SendPrivateImage(_ context.Context, userID int64, file string) error {
	a.logger.Info("example send private image", "user_id", userID, "file", file)
	return nil
}

var _ platform.Adapter = (*Adapter)(nil)