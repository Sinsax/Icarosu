package recent

import (
	"context"
	_ "embed"
	"strings"
	"sync"

	"icarosu/internal/manifest"
	"icarosu/internal/sdk"
)

//go:embed manifest.yaml
var manifestYAML []byte

var parsedManifest = manifest.MustParse(manifestYAML)

const defaultLimit = 20

type entry struct {
	actor string
	text  string
}

type Plugin struct {
	mu     sync.Mutex
	byConv map[sdk.ConversationID][]entry
	limit  int
}

func New() sdk.Plugin {
	return &Plugin{byConv: make(map[sdk.ConversationID][]entry), limit: defaultLimit}
}

func (p *Plugin) Manifest() sdk.Manifest                        { return parsedManifest }
func (p *Plugin) Init(ctx context.Context, host sdk.Host) error { return nil }
func (p *Plugin) Start(ctx context.Context) error               { return nil }
func (p *Plugin) Stop(ctx context.Context) error                { return nil }
func (p *Plugin) Health(ctx context.Context) sdk.Health         { return sdk.Healthy() }

func (p *Plugin) Observe(ctx context.Context, ev sdk.Event) {
	if ev.Kind != "message.created" {
		return
	}
	text := eventText(ev)
	if text == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	list := p.byConv[ev.Conversation.ID]
	list = append(list, entry{actor: ev.Actor.Name, text: text})
	if len(list) > p.limit {
		list = list[len(list)-p.limit:]
	}
	p.byConv[ev.Conversation.ID] = list
}

func (p *Plugin) Provide(ctx context.Context, turn *sdk.Turn) ([]sdk.ContextBlock, error) {
	p.mu.Lock()
	list := append([]entry(nil), p.byConv[turn.Event.Conversation.ID]...)
	p.mu.Unlock()
	if len(list) == 0 {
		return nil, nil
	}
	var lines []string
	for _, e := range list {
		name := e.actor
		if name == "" {
			name = "群友"
		}
		lines = append(lines, name+": "+e.text)
	}
	return []sdk.ContextBlock{{
		Source:   "memory.recent",
		Role:     "system",
		Content:  "最近群聊：\n" + strings.Join(lines, "\n"),
		Priority: 50,
	}}, nil
}

func (p *Plugin) ID() string    { return "memory.recent" }
func (p *Plugin) Priority() int { return 50 }
func (p *Plugin) Budget() int   { return 500 }

func eventText(ev sdk.Event) string {
	var b strings.Builder
	for _, part := range ev.Parts {
		if part.Type == sdk.PartText {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Observer = (*Plugin)(nil)
var _ sdk.ContextProvider = (*Plugin)(nil)
