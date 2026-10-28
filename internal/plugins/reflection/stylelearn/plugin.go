package stylelearn

import (
	"context"
	_ "embed"
	"encoding/json"
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
	SampleEvery int `json:"sample_every"`
	MaxSamples  int `json:"max_samples"`
}

type Plugin struct {
	mu          sync.Mutex
	llm         sdk.LLM
	logger      *slog.Logger
	cfg         Config
	counts      map[sdk.ConversationID]int
	expressions map[sdk.ConversationID][]string
	inflight    map[sdk.ConversationID]bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          async.Group
}

func New() sdk.Plugin {
	return &Plugin{
		logger:      slog.Default(),
		cfg:         Config{SampleEvery: 20, MaxSamples: 10},
		counts:      make(map[sdk.ConversationID]int),
		expressions: make(map[sdk.ConversationID][]string),
		inflight:    make(map[sdk.ConversationID]bool),
	}
}

func (p *Plugin) Manifest() sdk.Manifest { return parsedManifest }

func (p *Plugin) Init(ctx context.Context, host sdk.Host) error {
	if host != nil {
		p.llm = host.LLM()
		if host.Logger() != nil {
			p.logger = host.Logger()
		}
		_ = host.Config("reflection.stylelearn", &p.cfg)
	}
	if p.cfg.SampleEvery <= 0 {
		p.cfg.SampleEvery = 20
	}
	if p.cfg.MaxSamples <= 0 {
		p.cfg.MaxSamples = 10
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
	if text == "" || len([]rune(text)) > 30 {
		return
	}
	p.mu.Lock()
	p.counts[ev.Conversation.ID]++
	should := p.counts[ev.Conversation.ID]%p.cfg.SampleEvery == 0 && !p.inflight[ev.Conversation.ID]
	if should {
		p.inflight[ev.Conversation.ID] = true
	}
	p.mu.Unlock()
	if !should || p.llm == nil {
		if should {
			p.mu.Lock()
			delete(p.inflight, ev.Conversation.ID)
			p.mu.Unlock()
		}
		return
	}
	if !p.wg.Go(func() { p.learn(ev.Conversation.ID, text) }) {
		p.mu.Lock()
		delete(p.inflight, ev.Conversation.ID)
		p.mu.Unlock()
	}
}

func (p *Plugin) learn(conv sdk.ConversationID, text string) {
	defer func() {
		p.mu.Lock()
		delete(p.inflight, conv)
		p.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(p.ctx, 20*time.Second)
	defer cancel()
	prompt := "判断下面这句话是否适合作为群聊表达方式/口癖/万能回复。如果适合，输出JSON：" + `{"collect":true,"expression":"短句","tags":["标签"]}` + "；不适合输出" + `{"collect":false}` + "。只输出JSON，不要解释。句子：" + text
	out, err := p.llm.Chat(ctx, []sdk.Message{
		{Role: "system", Content: "你是群聊表达方式筛选助手。"},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		p.logger.Warn("style learn failed", "conversation", conv, "error", err)
		return
	}
	var result struct {
		Collect    bool     `json:"collect"`
		Expression string   `json:"expression"`
		Tags       []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(out)), &result); err != nil || !result.Collect || result.Expression == "" {
		return
	}
	p.mu.Lock()
	list := append(p.expressions[conv], result.Expression)
	if len(list) > p.cfg.MaxSamples {
		list = list[len(list)-p.cfg.MaxSamples:]
	}
	p.expressions[conv] = list
	p.mu.Unlock()
}

func (p *Plugin) Provide(ctx context.Context, turn *sdk.Turn) ([]sdk.ContextBlock, error) {
	p.mu.Lock()
	list := append([]string(nil), p.expressions[turn.Event.Conversation.ID]...)
	p.mu.Unlock()
	if len(list) == 0 {
		return nil, nil
	}
	return []sdk.ContextBlock{{
		Source:   "style.learned",
		Role:     "system",
		Content:  "群内表达参考（可模仿，不要照抄）：\n- " + strings.Join(list, "\n- "),
		Priority: 60,
	}}, nil
}

func (p *Plugin) ID() string    { return "reflection.stylelearn" }
func (p *Plugin) Priority() int { return 60 }
func (p *Plugin) Budget() int   { return 200 }

func eventText(ev sdk.Event) string {
	var b strings.Builder
	for _, part := range ev.Parts {
		if part.Type == sdk.PartText {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\x60\x60\x60json")
	s = strings.TrimPrefix(s, "\x60\x60\x60")
	s = strings.TrimSuffix(s, "\x60\x60\x60")
	return strings.TrimSpace(s)
}

var _ sdk.Plugin = (*Plugin)(nil)
var _ sdk.Observer = (*Plugin)(nil)
var _ sdk.ContextProvider = (*Plugin)(nil)
