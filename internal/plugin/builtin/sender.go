package builtin

import (
	"context"

	corechat "icarosu/internal/core/chat"
	"icarosu/internal/obs"
	"icarosu/internal/plugin"
)

// senderMetrics 是全局默认回复发送指标，由 app 组装时注入。
var senderMetrics *obs.Metrics

// SetSenderMetrics 设置回复发送指标收集器。
func SetSenderMetrics(m *obs.Metrics) {
	senderMetrics = m
}

// SenderPlugin 是 Postprocessor 发送器。
// chat 等 Responder 只负责把回复写入 Context.Data，由本插件统一发送。
type SenderPlugin struct {
	plugin.BasePlugin
}

func NewSenderPlugin() plugin.Plugin {
	return &SenderPlugin{}
}

func init() {
	plugin.RegisterGlobal(NewSenderPlugin())
}

func (p *SenderPlugin) Name() string {
	return "sender"
}

func (p *SenderPlugin) Description() string {
	return "在 Postprocessor 阶段统一发送聊天回复"
}

func (p *SenderPlugin) Priority() int {
	return 10
}

func (p *SenderPlugin) Stage() plugin.Stage {
	return plugin.StagePostprocessor
}

func (p *SenderPlugin) Handle(ctx context.Context, bot plugin.Bot, e *plugin.Event) bool {
	return p.HandleContext(ctx, bot, plugin.NewContext(e))
}

func (p *SenderPlugin) HandleContext(ctx context.Context, bot plugin.Bot, pc *plugin.Context) bool {
	reply, _ := pc.Get("reply").(string)
	if reply == "" {
		return false
	}
	forceSplit, _ := pc.Get("force_split").(bool)
	e := pc.Event

	send := func(text string) {
		if e.Type == plugin.EventGroupMessage {
			_ = bot.SendGroupMessage(ctx, e.GroupID, text)
		} else {
			_ = bot.SendPrivateMessage(ctx, e.UserID, text)
		}
	}

	if senderMetrics != nil {
		senderMetrics.IncRepliesSent()
	}

	if forceSplit {
		for _, part := range corechat.NaturalParts(reply) {
			send(part)
		}
		return false
	}
	send(reply)
	return false
}