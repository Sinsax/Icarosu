package builtin

import (
	"context"
	"strings"

	"icarosu/internal/plugin"
)

// PingPlugin 是最小示例插件，用于验证 OneBot 连接和插件分发是否正常。
type PingPlugin struct {
	plugin.BasePlugin
}

func NewPingPlugin() plugin.Plugin {
	return &PingPlugin{}
}

func init() {
	plugin.RegisterGlobal(NewPingPlugin())
}

func (p *PingPlugin) Name() string {
	return "ping"
}

func (p *PingPlugin) Description() string {
	return "收到 ping 时回复 pong，用于连通性测试"
}

func (p *PingPlugin) Priority() int {
	return 100
}

func (p *PingPlugin) Handle(ctx context.Context, bot plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage && e.Type != plugin.EventPrivateMessage {
		return false
	}

	msg := strings.TrimSpace(e.RawMessage)
	if msg == "ping" {
		if e.Type == plugin.EventGroupMessage {
			_ = bot.SendGroupMessage(ctx, e.GroupID, "pong")
		} else {
			_ = bot.SendPrivateMessage(ctx, e.UserID, "pong")
		}
		return true
	}
	return false
}
