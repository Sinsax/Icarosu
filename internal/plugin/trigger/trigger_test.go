package trigger_test

import (
	"context"
	"testing"

	"icarosu/internal/config"
	"icarosu/internal/plugin"
	"icarosu/internal/plugin/trigger"
)

func TestTriggerMentionAlwaysPasses(t *testing.T) {
	p := trigger.New(config.BotConfig{
		MentionReply:    true,
		CooldownSeconds: 60,
	})

	handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:    plugin.EventGroupMessage,
		GroupID: 1,
		AtBot:   true,
	})
	if handled {
		t.Fatal("mention should not be blocked")
	}
}

func TestTriggerKeywordPasses(t *testing.T) {
	p := trigger.New(config.BotConfig{
		MentionReply:    true,
		Keywords:        []string{"摸鱼"},
		CooldownSeconds: 60,
	})

	handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    1,
		RawMessage: "今天摸鱼吗",
	})
	if handled {
		t.Fatal("keyword should not be blocked")
	}
}

func TestTriggerInterestPasses(t *testing.T) {
	p := trigger.New(config.BotConfig{
		MentionReply:    true,
		InterestReply:   true,
		Interests:       []string{"看番"},
		CooldownSeconds: 60,
	})

	handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    1,
		RawMessage: "最近在看番",
	})
	if handled {
		t.Fatal("interest message should not be blocked")
	}
}

func TestTriggerCooldownBlocksNonMention(t *testing.T) {
	p := trigger.New(config.BotConfig{
		MentionReply:      true,
		Keywords:          []string{"触发"},
		InterestReply:     false,
		RandomProbability: 0,
		CooldownSeconds:   60,
	})

	// 第一次命中关键词，放行并进入冷却
	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    1,
		RawMessage: "触发一下",
	}); handled {
		t.Fatal("first keyword should pass")
	}

	// 冷却期内普通消息应被拦截
	if handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:       plugin.EventGroupMessage,
		GroupID:    1,
		RawMessage: "普通消息",
	}); !handled {
		t.Fatal("message during cooldown should be blocked")
	}
}

func TestTriggerPrivateAlwaysPasses(t *testing.T) {
	p := trigger.New(config.BotConfig{
		RandomProbability: 0,
	})

	handled := p.Handle(context.Background(), nil, &plugin.Event{
		Type:   plugin.EventPrivateMessage,
		UserID: 1,
	})
	if handled {
		t.Fatal("private message should always pass trigger")
	}
}
