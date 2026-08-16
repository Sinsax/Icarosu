package membersync

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"icarosu/internal/plugin"
	"icarosu/internal/qq"
	"icarosu/internal/store"
)

// MemberSyncPlugin 定时同步群成员昵称/群名片到数据库。
type MemberSyncPlugin struct {
	plugin.BasePlugin
	client *qq.Client
	store  *store.Store
	logger *slog.Logger

	mu     sync.Mutex
	synced map[int64]time.Time
}

// Option 用于配置 MemberSyncPlugin。
type Option func(*MemberSyncPlugin)

// WithLogger 设置日志器。
func WithLogger(logger *slog.Logger) Option {
	return func(p *MemberSyncPlugin) {
		p.logger = logger
	}
}

func New(client *qq.Client, st *store.Store, opts ...Option) plugin.Plugin {
	p := &MemberSyncPlugin{
		client: client,
		store:  st,
		logger: slog.Default(),
		synced: make(map[int64]time.Time),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *MemberSyncPlugin) Name() string {
	return "membersync"
}

func (p *MemberSyncPlugin) Description() string {
	return "同步群成员昵称和群名片到数据库"
}

func (p *MemberSyncPlugin) Priority() int {
	return 930
}

func (p *MemberSyncPlugin) Stage() plugin.Stage {
	return plugin.StagePreprocessor
}

func (p *MemberSyncPlugin) Handle(ctx context.Context, _ plugin.Bot, e *plugin.Event) bool {
	if e.Type != plugin.EventGroupMessage {
		return false
	}
	if p.client == nil || p.store == nil {
		return false
	}

	p.mu.Lock()
	last, ok := p.synced[e.GroupID]
	needSync := !ok || time.Since(last) > time.Hour
	p.mu.Unlock()
	if !needSync {
		return false
	}

	members, err := p.client.GetGroupMemberList(ctx, e.GroupID)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("sync group members failed", "group_id", e.GroupID, "error", err)
		}
		return false
	}

	for _, m := range members {
		_ = p.store.UpsertMemberInfo(e.GroupID, m.UserID, m.Nickname, m.Card)
	}

	p.mu.Lock()
	p.synced[e.GroupID] = time.Now()
	p.mu.Unlock()

	if p.logger != nil {
		p.logger.Info("synced group members", "group_id", e.GroupID, "count", len(members))
	}
	return false
}
