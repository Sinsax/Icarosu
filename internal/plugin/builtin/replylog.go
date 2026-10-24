package builtin

import (
	"context"
	"log/slog"

	"icarosu/internal/plugin"
)

// ReplyLogPlugin 是一个 Postprocessor 示例插件。
// 它读取流水线中间产物中的 reply，用于验证 Postprocessor 阶段会在 Responder 之后执行。
type ReplyLogPlugin struct {
	plugin.BasePlugin
}

func NewReplyLogPlugin() plugin.Plugin {
	return &ReplyLogPlugin{}
}

func init() {
	plugin.RegisterGlobal(NewReplyLogPlugin())
}

func (p *ReplyLogPlugin) Name() string {
	return "replylog"
}

func (p *ReplyLogPlugin) Description() string {
	return "在 Postprocessor 阶段记录聊天回复，用于验证回复后处理流水线"
}

func (p *ReplyLogPlugin) Priority() int {
	return 0
}

func (p *ReplyLogPlugin) Stage() plugin.Stage {
	return plugin.StagePostprocessor
}

func (p *ReplyLogPlugin) Handle(ctx context.Context, bot plugin.Bot, e *plugin.Event) bool {
	return p.HandleContext(ctx, bot, plugin.NewContext(e))
}

func (p *ReplyLogPlugin) HandleContext(_ context.Context, _ plugin.Bot, pc *plugin.Context) bool {
	reply, _ := pc.Get("reply").(string)
	if reply == "" {
		return false
	}
	slog.Info("postprocessor reply", "reply", reply, "force_split", pc.Get("force_split"))
	return false
}
