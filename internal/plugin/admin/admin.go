package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"icarosu/internal/attention"
	"icarosu/internal/memory"
	"icarosu/internal/plugin"
	"icarosu/internal/store"
)

// AdminPlugin 提供简单的管理员命令。
type AdminPlugin struct {
	plugin.BasePlugin
	adminIDs  []int64
	mem       *memory.Memory
	store     store.AdminStore
	attention *attention.Tracker
	logger    *slog.Logger
}

// Option 用于配置 AdminPlugin。
type Option func(*AdminPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *AdminPlugin) {
		p.logger = logger
	}
}

func New(adminIDs []int64, mem *memory.Memory, st store.AdminStore, at *attention.Tracker, opts ...Option) plugin.Plugin {
	p := &AdminPlugin{
		adminIDs:  adminIDs,
		mem:       mem,
		store:     st,
		attention: at,
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *AdminPlugin) Name() string {
	return "admin"
}

func (p *AdminPlugin) Description() string {
	return "管理员命令：!status / !clear / !mem"
}

func (p *AdminPlugin) Priority() int {
	return 800
}

func (p *AdminPlugin) Handle(ctx context.Context, bot plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage {
		return false
	}
	if !p.isAdmin(e.UserID) {
		return false
	}

	msg := strings.TrimSpace(e.RawMessage)
	switch {
	case strings.HasPrefix(msg, "!status"):
		p.status(ctx, bot, e)
		return true
	case strings.HasPrefix(msg, "!clear"):
		if p.mem != nil {
			p.mem.Clear(e)
		}
		_ = bot.SendGroupMessage(ctx, e.GroupID, "已清空短期上下文")
		return true
	case strings.HasPrefix(msg, "!mem"):
		p.memories(ctx, bot, e)
		return true
	}
	return false
}

func (p *AdminPlugin) isAdmin(userID int64) bool {
	for _, id := range p.adminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func (p *AdminPlugin) status(ctx context.Context, bot plugin.Bot, e *plugin.Event) {
	var b strings.Builder
	b.WriteString("状态\n")
	if p.attention != nil {
		fmt.Fprintf(&b, "注意力: %.0f\n", p.attention.Score(e.GroupID))
	}
	if p.store != nil {
		members, _ := p.store.GetMembers(e.GroupID)
		fmt.Fprintf(&b, "成员数: %d\n", len(members))
	}
	_ = bot.SendGroupMessage(ctx, e.GroupID, strings.TrimSpace(b.String()))
}

func (p *AdminPlugin) memories(ctx context.Context, bot plugin.Bot, e *plugin.Event) {
	if p.store == nil {
		_ = bot.SendGroupMessage(ctx, e.GroupID, "未启用长期存储")
		return
	}
	records, err := p.store.GetMemories(e.GroupID, 5)
	if err != nil || len(records) == 0 {
		_ = bot.SendGroupMessage(ctx, e.GroupID, "暂无长期记忆")
		return
	}
	var b strings.Builder
	for _, r := range records {
		b.WriteString("- ")
		b.WriteString(r.Content)
		b.WriteString("\n")
	}
	_ = bot.SendGroupMessage(ctx, e.GroupID, strings.TrimSpace(b.String()))
}
