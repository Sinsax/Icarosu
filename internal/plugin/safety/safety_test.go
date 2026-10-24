package safety

import (
	"context"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/plugin"
)

type noopBot struct{}

func (noopBot) SendGroupMessage(context.Context, int64, string) error   { return nil }
func (noopBot) SendPrivateMessage(context.Context, int64, string) error { return nil }
func (noopBot) SendGroupImage(context.Context, int64, string) error     { return nil }
func (noopBot) SendPrivateImage(context.Context, int64, string) error   { return nil }

func TestSafetyBlocksForbiddenKeyword(t *testing.T) {
	p := New(config.BotConfig{Forbidden: []string{"违法内容"}})
	blocked := p.Handle(context.Background(), noopBot{}, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    1,
		RawMessage: "这句话包含违法内容",
	})
	if !blocked {
		t.Fatal("expected forbidden message to be blocked")
	}
}

func TestSafetyAllowsNormalMessage(t *testing.T) {
	p := New(config.BotConfig{Forbidden: []string{"违法内容"}})
	blocked := p.Handle(context.Background(), noopBot{}, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    1,
		RawMessage: "今天天气不错",
	})
	if blocked {
		t.Fatal("normal message should not be blocked")
	}
}
