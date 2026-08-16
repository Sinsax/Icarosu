package dryrun

import (
	"context"
	"log/slog"

	"icarosu/internal/plugin"
)

// Bot 是一个测试用包装器：完整走发送流程，但不会真正把消息发到平台。
// 所有 Send* 方法只记录日志并返回 nil，方便观察真实场景下“本应发送”的内容。
type Bot struct {
	inner  plugin.Bot
	logger *slog.Logger
}

func New(inner plugin.Bot, logger *slog.Logger) *Bot {
	if logger == nil {
		logger = slog.Default()
	}
	return &Bot{inner: inner, logger: logger}
}

func (b *Bot) SendGroupMessage(_ context.Context, groupID int64, message string) error {
	b.logger.Info("DRY RUN would send group message",
		"group_id", groupID,
		"message", message,
	)
	return nil
}

func (b *Bot) SendPrivateMessage(_ context.Context, userID int64, message string) error {
	b.logger.Info("DRY RUN would send private message",
		"user_id", userID,
		"message", message,
	)
	return nil
}

func (b *Bot) SendGroupImage(_ context.Context, groupID int64, file string) error {
	b.logger.Info("DRY RUN would send group image",
		"group_id", groupID,
		"file", file,
	)
	return nil
}

func (b *Bot) SendPrivateImage(_ context.Context, userID int64, file string) error {
	b.logger.Info("DRY RUN would send private image",
		"user_id", userID,
		"file", file,
	)
	return nil
}

var _ plugin.Bot = (*Bot)(nil)
