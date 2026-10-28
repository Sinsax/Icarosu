package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"icarosu/internal/sdk"
)

// FakeClock 是可推进的测试时钟。
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// FakeLLM 是返回固定回复的 LLM。
type FakeLLM struct {
	mu        sync.Mutex
	Reply     string
	Err       error
	Calls     int
	ToolCalls []sdk.LLMToolCall
}

func (f *FakeLLM) Chat(ctx context.Context, messages []sdk.Message) (string, error) {
	f.mu.Lock()
	f.Calls++
	f.mu.Unlock()
	return f.Reply, f.Err
}

func (f *FakeLLM) ChatWithTools(ctx context.Context, messages []sdk.Message, tools []sdk.ToolDefinition, exec sdk.ToolExecutor) (string, error) {
	f.mu.Lock()
	f.Calls++
	calls := append([]sdk.LLMToolCall(nil), f.ToolCalls...)
	f.mu.Unlock()
	for _, call := range calls {
		if exec == nil {
			return "", fmt.Errorf("tool executor is nil")
		}
		if _, err := exec(ctx, call.Name, call.Arguments); err != nil {
			return "", err
		}
	}
	return f.Reply, f.Err
}

// FakeBus 记录发布的事件并支持同步订阅。
type FakeBus struct {
	mu          sync.Mutex
	Published   []sdk.Event
	subscribers map[string][]sdk.Observer
}

func NewFakeBus() *FakeBus {
	return &FakeBus{subscribers: make(map[string][]sdk.Observer)}
}

func (b *FakeBus) Publish(ctx context.Context, ev sdk.Event) error {
	b.mu.Lock()
	b.Published = append(b.Published, ev)
	subs := append([]sdk.Observer(nil), b.subscribers[ev.Kind]...)
	b.mu.Unlock()
	for _, fn := range subs {
		fn.Observe(ctx, ev)
	}
	return nil
}

func (b *FakeBus) Subscribe(kind string, fn sdk.Observer) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[kind] = append(b.subscribers[kind], fn)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subscribers[kind]
		for i, s := range list {
			if s == fn {
				b.subscribers[kind] = append(list[:i], list[i+1:]...)
				return
			}
		}
	}, nil
}

// FakeMemory 是内存记忆。
type FakeMemory struct {
	mu    sync.Mutex
	Items []sdk.MemoryItem
}

func (m *FakeMemory) Recall(ctx context.Context, query string, limit int) ([]sdk.MemoryItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]sdk.MemoryItem(nil), m.Items...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *FakeMemory) Remember(ctx context.Context, item sdk.MemoryItem) error {
	m.mu.Lock()
	m.Items = append(m.Items, item)
	m.mu.Unlock()
	return nil
}

// FakeToolRegistry 是工具注册表。
type FakeToolRegistry struct {
	mu    sync.Mutex
	tools map[string]sdk.Tool
	order []string
}

func NewFakeToolRegistry() *FakeToolRegistry {
	return &FakeToolRegistry{tools: make(map[string]sdk.Tool)}
}

func (r *FakeToolRegistry) Register(t sdk.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[t.Name]; ok {
		return fmt.Errorf("duplicate tool %q", t.Name)
	}
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
	return nil
}

func (r *FakeToolRegistry) List() []sdk.ToolSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]sdk.ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, sdk.ToolSpec{Name: t.Name, Summary: t.Summary, Permissions: t.Permissions})
	}
	return out
}

func (r *FakeToolRegistry) Get(name string) (sdk.Tool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tools[name]
	return t, ok
}

// FakeActionSink 记录发送的动作。
type FakeActionSink struct {
	mu      sync.Mutex
	Actions []sdk.Action
	Err     error
}

func (s *FakeActionSink) Send(ctx context.Context, a sdk.Action) error {
	s.mu.Lock()
	s.Actions = append(s.Actions, a)
	s.mu.Unlock()
	return s.Err
}

// FakeStorage 是内存 KV。
type FakeStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewFakeStorage() *FakeStorage {
	return &FakeStorage{data: make(map[string][]byte)}
}

func (s *FakeStorage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	return append([]byte(nil), v...), nil
}

func (s *FakeStorage) Put(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	s.data[key] = append([]byte(nil), value...)
	s.mu.Unlock()
	return nil
}

func (s *FakeStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
	return nil
}

func (s *FakeStorage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for k := range s.data {
		if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// FakeVision 是可配置的图片理解。
type FakeVision struct {
	mu       sync.Mutex
	Analysis sdk.ImageAnalysis
	Err      error
	Calls    int
}

func (v *FakeVision) AnalyzeImage(ctx context.Context, url, persona string) (sdk.ImageAnalysis, error) {
	v.mu.Lock()
	v.Calls++
	v.mu.Unlock()
	return v.Analysis, v.Err
}

// FakeMemeStore 记录收藏的图片。
type FakeMemeStore struct {
	mu    sync.Mutex
	Saved []string
	Err   error
}

func (m *FakeMemeStore) SaveFromURL(ctx context.Context, url string) (string, error) {
	m.mu.Lock()
	m.Saved = append(m.Saved, url)
	m.mu.Unlock()
	return url, m.Err
}

func (m *FakeMemeStore) SavedURLs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.Saved...)
}

// FakeMemberDirectory 返回固定成员列表。
type FakeMemberDirectory struct {
	Members []sdk.Member
	Err     error
}

func (d *FakeMemberDirectory) ListMembers(ctx context.Context, c sdk.Conversation) ([]sdk.Member, error) {
	return d.Members, d.Err
}

// FakeHost 是插件测试用的 Host。
type FakeHost struct {
	IDValue     string
	Caps        map[string]bool
	BusValue    *FakeBus
	LLMValue    *FakeLLM
	MemValue    *FakeMemory
	ToolValue   *FakeToolRegistry
	ActValue    *FakeActionSink
	StoreValue  *FakeStorage
	VisionValue *FakeVision
	MemeValue   *FakeMemeStore
	MemberValue *FakeMemberDirectory
	LoggerValue *slog.Logger
	ClockValue  *FakeClock
}

func NewFakeHost() *FakeHost {
	return &FakeHost{
		IDValue:     "test.host",
		Caps:        map[string]bool{},
		BusValue:    NewFakeBus(),
		LLMValue:    &FakeLLM{Reply: "ok"},
		MemValue:    &FakeMemory{},
		ToolValue:   NewFakeToolRegistry(),
		ActValue:    &FakeActionSink{},
		StoreValue:  NewFakeStorage(),
		VisionValue: &FakeVision{},
		MemeValue:   &FakeMemeStore{},
		MemberValue: &FakeMemberDirectory{},
		LoggerValue: slog.Default(),
		ClockValue:  NewFakeClock(time.Unix(0, 0)),
	}
}

func (h *FakeHost) ID() string { return h.IDValue }

func (h *FakeHost) Has(capability string) bool {
	if h.Caps == nil {
		return false
	}
	return h.Caps[capability]
}

func (h *FakeHost) Bus() sdk.Bus            { return h.BusValue }
func (h *FakeHost) LLM() sdk.LLM            { return h.LLMValue }
func (h *FakeHost) Memory() sdk.Memory      { return h.MemValue }
func (h *FakeHost) Tools() sdk.ToolRegistry { return h.ToolValue }
func (h *FakeHost) Actions() sdk.ActionSink { return h.ActValue }

func (h *FakeHost) Storage(namespace string) (sdk.Storage, error) { return h.StoreValue, nil }

func (h *FakeHost) Config(section string, dst any) error { return nil }

func (h *FakeHost) Logger() *slog.Logger { return h.LoggerValue }
func (h *FakeHost) Clock() sdk.Clock     { return h.ClockValue }

func (h *FakeHost) Members() sdk.MemberDirectory { return h.MemberValue }
func (h *FakeHost) Vision() sdk.Vision           { return h.VisionValue }
func (h *FakeHost) Memes() sdk.MemeStore         { return h.MemeValue }

// MustJSON 是测试里构造 JSON schema 的小工具。
func MustJSON(s string) json.RawMessage {
	if !json.Valid([]byte(s)) {
		panic("invalid json: " + s)
	}
	return json.RawMessage(s)
}

var (
	_ sdk.Host            = (*FakeHost)(nil)
	_ sdk.Bus             = (*FakeBus)(nil)
	_ sdk.LLM             = (*FakeLLM)(nil)
	_ sdk.Memory          = (*FakeMemory)(nil)
	_ sdk.ToolRegistry    = (*FakeToolRegistry)(nil)
	_ sdk.ActionSink      = (*FakeActionSink)(nil)
	_ sdk.Storage         = (*FakeStorage)(nil)
	_ sdk.Clock           = (*FakeClock)(nil)
	_ sdk.MemberDirectory = (*FakeMemberDirectory)(nil)
	_ sdk.Vision          = (*FakeVision)(nil)
	_ sdk.MemeStore       = (*FakeMemeStore)(nil)
)
