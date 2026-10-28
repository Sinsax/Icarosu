package membersync

import (
	"context"
	_ "embed"
	"log/slog"
	"strings"
	"sync"
	"time"

	"icarosu/internal/manifest"
	"icarosu/internal/plugins/reflection/internal/async"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

type Config struct {
	IntervalMinutes int `json:"interval_minutes"`
}

type Plugin struct {
	mu          sync.Mutex
	host        sdk.Host
	logger      *slog.Logger
	cfg         Config
	members     map[sdk.ConversationID][]sdk.Member
	lastSync    map[sdk.ConversationID]time.Time
	lastAttempt map[sdk.ConversationID]time.Time
	ctx         context.Context
	cancel      context.CancelFunc
	wg          async.Group
}

func New() sdk.Plugin {
	return &Plugin{
		logger:      slog.Default(),
		cfg:         Config{IntervalMinutes: 60},
		members:     make(map[sdk.ConversationID][]sdk.Member),
		lastSync:    make(map[sdk.ConversationID]time.Time),
		lastAttempt: make(map[sdk.ConversationID]time.Time),
	}
}

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	p.host = host
	if host != nil {
		if host.Logger() != nil {
			p.logger = host.Logger()
		}
		_ = host.Config("reflection.membersync", &p.cfg)
	}
	if p.cfg.IntervalMinutes <= 0 {
		p.cfg.IntervalMinutes = 60
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.wg = async.Group{}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	return nil
}

func (p *Plugin) Start(ctx context.Context) error { return nil }
func (p *Plugin) Stop(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	return p.wg.Close(ctx)
}
func (p *Plugin) Health(ctx context.Context) sdk.Health { return sdk.Healthy() }

func (p *Plugin) Observe(ctx context.Context, ev sdk.Event) {
	if ev.Kind != "message.created" || ev.Conversation.Kind != "group" {
		return
	}
	now := time.Now()
	p.mu.Lock()
	last := p.lastSync[ev.Conversation.ID]
	if !last.IsZero() && now.Sub(last) < time.Duration(p.cfg.IntervalMinutes)*time.Minute {
		p.mu.Unlock()
		return
	}
	attempt := p.lastAttempt[ev.Conversation.ID]
	if !attempt.IsZero() && now.Sub(attempt) < time.Minute {
		p.mu.Unlock()
		return
	}
	p.lastAttempt[ev.Conversation.ID] = now
	p.mu.Unlock()
	p.wg.Go(func() { p.sync(ev.Conversation) })
}

func (p *Plugin) sync(conv sdk.Conversation) {
	host, ok := p.host.(sdk.MemberHost)
	if !ok || host.Members() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(p.ctx, 10*time.Second)
	defer cancel()
	members, err := host.Members().ListMembers(ctx, conv)
	if err != nil {
		p.logger.Debug("member sync failed", "conversation", conv.ID, "error", err)
		return
	}
	p.mu.Lock()
	p.members[conv.ID] = members
	p.lastSync[conv.ID] = time.Now()
	p.mu.Unlock()
}

func (p *Plugin) Provide(ctx context.Context, turn *sdk.Turn) ([]sdk.ContextBlock, error) {
	p.mu.Lock()
	members := append([]sdk.Member(nil), p.members[turn.Event.Conversation.ID]...)
	p.mu.Unlock()
	if len(members) == 0 {
		return nil, nil
	}
	const max = 30
	var lines []string
	for i, m := range members {
		if i >= max {
			lines = append(lines, "...")
			break
		}
		name := m.Name
		if name == "" {
			name = string(m.ID)
		}
		lines = append(lines, name+"（"+string(m.ID)+"）")
	}
	return []sdk.ContextBlock{{
		Source:   "members.synced",
		Role:     "system",
		Content:  "群成员：\n- " + strings.Join(lines, "\n- "),
		Priority: 30,
	}}, nil
}

func (p *Plugin) ID() string    { return "reflection.membersync" }
func (p *Plugin) Priority() int { return 30 }
func (p *Plugin) Budget() int   { return 400 }

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Observer = (*Plugin)(nil)
var _ sdk.ContextProvider = (*Plugin)(nil)
