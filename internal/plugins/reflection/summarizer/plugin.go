package summarizer

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
	Threshold  int `json:"threshold"`
	MaxHistory int `json:"max_history"`
}

type Plugin struct {
	mu        sync.Mutex
	llm       sdk.LLM
	logger    *slog.Logger
	cfg       Config
	history   map[sdk.ConversationID][]string
	summaries map[sdk.ConversationID]string
	inflight  map[sdk.ConversationID]bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        async.Group
}

func New() sdk.Plugin {
	return &Plugin{
		logger:    slog.Default(),
		cfg:       Config{Threshold: 50, MaxHistory: 100},
		history:   make(map[sdk.ConversationID][]string),
		summaries: make(map[sdk.ConversationID]string),
		inflight:  make(map[sdk.ConversationID]bool),
	}
}

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil {
		p.llm = host.LLM()
		if host.Logger() != nil {
			p.logger = host.Logger()
		}
		_ = host.Config("reflection.summarizer", &p.cfg)
	}
	if p.cfg.Threshold <= 0 {
		p.cfg.Threshold = 50
	}
	if p.cfg.MaxHistory <= 0 || p.cfg.MaxHistory < p.cfg.Threshold {
		p.cfg.MaxHistory = p.cfg.Threshold * 2
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
	if ev.Kind != "message.created" {
		return
	}
	text := eventText(ev)
	if text == "" {
		return
	}
	p.mu.Lock()
	list := append(p.history[ev.Conversation.ID], text)
	if len(list) > p.cfg.MaxHistory {
		list = list[len(list)-p.cfg.MaxHistory:]
	}
	p.history[ev.Conversation.ID] = list
	if len(list) < p.cfg.Threshold || p.inflight[ev.Conversation.ID] {
		p.mu.Unlock()
		return
	}
	snapshot := append([]string(nil), list...)
	p.inflight[ev.Conversation.ID] = true
	p.mu.Unlock()

	if p.llm == nil {
		p.mu.Lock()
		delete(p.inflight, ev.Conversation.ID)
		p.mu.Unlock()
		return
	}
	if !p.wg.Go(func() { p.summarize(ev.Conversation.ID, snapshot) }) {
		p.mu.Lock()
		delete(p.inflight, ev.Conversation.ID)
		p.mu.Unlock()
	}
}

func (p *Plugin) summarize(conv sdk.ConversationID, lines []string) {
	defer func() {
		p.mu.Lock()
		delete(p.inflight, conv)
		p.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(p.ctx, 20*time.Second)
	defer cancel()

	messages := []sdk.Message{
		{Role: "system", Content: "你是群聊总结助手。用简洁中文总结核心话题、关键信息和人物关系，不超过 200 字。"},
		{Role: "user", Content: strings.Join(lines, "\n")},
	}
	summary, err := p.llm.Chat(ctx, messages)
	if err != nil {
		p.logger.Warn("summarize failed", "conversation", conv, "error", err)
	} else if strings.TrimSpace(summary) != "" {
		p.mu.Lock()
		p.summaries[conv] = strings.TrimSpace(summary)
		if history := p.history[conv]; len(history) >= len(lines) {
			p.history[conv] = append([]string(nil), history[len(lines):]...)
		} else {
			p.history[conv] = nil
		}
		p.mu.Unlock()
	}
}

func (p *Plugin) Provide(ctx context.Context, turn *sdk.Turn) ([]sdk.ContextBlock, error) {
	p.mu.Lock()
	summary := p.summaries[turn.Event.Conversation.ID]
	p.mu.Unlock()
	if summary == "" {
		return nil, nil
	}
	return []sdk.ContextBlock{{
		Source:   "memory.summary",
		Role:     "system",
		Content:  "群聊总结：" + summary,
		Priority: 40,
	}}, nil
}

func (p *Plugin) ID() string    { return "reflection.summarizer" }
func (p *Plugin) Priority() int { return 40 }
func (p *Plugin) Budget() int   { return 300 }

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
