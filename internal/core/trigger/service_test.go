package trigger

import (
	"context"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/core/domain"
)

func TestShouldReplyPrivateMessageAlwaysTrue(t *testing.T) {
	s := New(config.BotConfig{})
	ok := s.ShouldReply(context.Background(), &domain.ChatMessage{
		Type:       domain.MessagePrivate,
		UserID:     1,
		RawMessage: "你好",
	})
	if !ok {
		t.Fatal("private message should always pass trigger")
	}
}

func TestShouldReplyGroupWithoutTriggerFalse(t *testing.T) {
	s := New(config.BotConfig{})
	ok := s.ShouldReply(context.Background(), &domain.ChatMessage{
		Type:       domain.MessageGroup,
		GroupID:    1,
		UserID:     2,
		RawMessage: "今天天气不错",
	})
	if ok {
		t.Fatal("non-mentioned group message should not pass by default")
	}
}

func TestShouldReplyAtBotTrue(t *testing.T) {
	s := New(config.BotConfig{MentionReply: true})
	ok := s.ShouldReply(context.Background(), &domain.ChatMessage{
		Type:       domain.MessageGroup,
		GroupID:    1,
		UserID:     2,
		RawMessage: "在吗",
		AtBot:      true,
	})
	if !ok {
		t.Fatal("at-bot message should pass trigger")
	}
}

func TestShouldReplyKeywordTrue(t *testing.T) {
	s := New(config.BotConfig{Keywords: []string{"小智"}})
	ok := s.ShouldReply(context.Background(), &domain.ChatMessage{
		Type:       domain.MessageGroup,
		GroupID:    1,
		UserID:     2,
		RawMessage: "小智在吗",
	})
	if !ok {
		t.Fatal("keyword message should pass trigger")
	}
}
