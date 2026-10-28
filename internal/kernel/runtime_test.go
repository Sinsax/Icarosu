package kernel_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"icarosu/internal/kernel"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

type fakeProvider struct {
	blocks []sdk.ContextBlock
}

func (p *fakeProvider) ID() string    { return "fake.context" }
func (p *fakeProvider) Priority() int { return 10 }
func (p *fakeProvider) Budget() int   { return 100 }
func (p *fakeProvider) Provide(ctx context.Context, t *sdk.Turn) ([]sdk.ContextBlock, error) {
	return p.blocks, nil
}

type fakeDecider struct {
	decision sdk.Decision
	err      error
}

func (d *fakeDecider) Decide(ctx context.Context, t *sdk.Turn) (sdk.Decision, error) {
	return d.decision, d.err
}

type fakePolicy struct {
	err error
}

func (p *fakePolicy) Check(ctx context.Context, t *sdk.Turn) error { return p.err }

func messageEvent() sdk.Event {
	return sdk.Event{
		ID:   "e1",
		Kind: "message.created",
		Conversation: sdk.Conversation{
			ID:        "qq:group:100",
			AdapterID: "onebot",
			Kind:      "group",
			Metadata:  map[string]string{"native_id": "100"},
		},
		Actor:    sdk.Actor{ID: "qq:user:200", Name: "小明"},
		Self:     "qq:user:1",
		Mentions: []sdk.ActorID{"qq:user:1"},
		Parts:    []sdk.Part{{Type: sdk.PartText, Text: "在吗"}},
		Meta:     map[string]string{"native_message_id": "9"},
	}
}

func TestTurnEngineReplyWithContextAndTool(t *testing.T) {
	llm := &testkit.FakeLLM{
		Reply:     "在的",
		ToolCalls: []sdk.LLMToolCall{{Name: "echo", Arguments: "{}"}},
	}
	sink := &testkit.FakeActionSink{}
	engine := kernel.NewTurnEngine(kernel.TurnOptions{
		LLM:     llm,
		Actions: sink,
		Persona: sdk.Persona{SystemPrompt: "你是群友"},
		Providers: []sdk.ContextProvider{&fakeProvider{blocks: []sdk.ContextBlock{{
			Source: "fake", Content: "背景", Role: "system", Tokens: 10,
		}}}},
		Decider: &fakeDecider{decision: sdk.Decision{Kind: sdk.DecisionReply, Reason: "test"}},
	})
	var toolRan bool
	if err := engine.RegisterTool(sdk.Tool{
		Name:    "echo",
		Summary: "echo",
		Schema:  []byte("{}"),
		Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
			toolRan = true
			return sdk.Text("ok"), nil
		},
	}); err != nil {
		t.Fatalf("RegisterTool: %v", err)
	}

	res, err := engine.Run(context.Background(), messageEvent())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !toolRan {
		t.Fatal("tool should run")
	}
	if res.Decision.Kind != sdk.DecisionReply || len(res.Actions) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(sink.Actions) != 1 || sink.Actions[0].Parts[0].Text != "在的" {
		t.Fatalf("unexpected sent actions: %+v", sink.Actions)
	}
	if res.Trace == nil || len(res.Trace.Blocks) != 1 || len(res.Trace.LLM) != 1 {
		t.Fatalf("unexpected trace: %+v", res.Trace)
	}
	if len(res.Trace.Tools) != 1 || res.Trace.Tools[0].Name != "echo" || !res.Trace.Tools[0].OK {
		t.Fatalf("unexpected tool trace: %+v", res.Trace.Tools)
	}
	if len(res.Trace.LLM[0].Tools) != 1 || res.Trace.LLM[0].Tools[0] != "echo" {
		t.Fatalf("unexpected llm tool trace: %+v", res.Trace.LLM[0])
	}
}

type tightProvider struct{}

func (tightProvider) ID() string    { return "tight" }
func (tightProvider) Priority() int { return 100 }
func (tightProvider) Budget() int   { return 5 }
func (tightProvider) Provide(ctx context.Context, t *sdk.Turn) ([]sdk.ContextBlock, error) {
	return []sdk.ContextBlock{{Source: "tight", Role: "system", Content: "too big", Tokens: 10}}, nil
}

type blockingOutput struct{}

func (blockingOutput) CheckOutput(ctx context.Context, t *sdk.Turn, reply string) error {
	return errors.New("blocked output")
}

func TestTurnEngineOutputPolicySilences(t *testing.T) {
	sink := &testkit.FakeActionSink{}
	engine := kernel.NewTurnEngine(kernel.TurnOptions{
		LLM:            &testkit.FakeLLM{Reply: "bad"},
		Actions:        sink,
		OutputPolicies: []sdk.OutputPolicy{blockingOutput{}},
		Decider:        &fakeDecider{decision: sdk.Decision{Kind: sdk.DecisionReply}},
	})
	res, err := engine.Run(context.Background(), messageEvent())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Decision.Kind != sdk.DecisionSilent || res.Decision.Reason != "output_policy" {
		t.Fatalf("unexpected decision: %+v", res.Decision)
	}
	if len(sink.Actions) != 0 || len(res.Actions) != 0 {
		t.Fatalf("output policy should prevent actions: %+v", sink.Actions)
	}
}

func TestTurnEngineProviderBudget(t *testing.T) {
	engine := kernel.NewTurnEngine(kernel.TurnOptions{
		LLM:       &testkit.FakeLLM{Reply: "ok"},
		Providers: []sdk.ContextProvider{tightProvider{}},
		Decider:   &fakeDecider{decision: sdk.Decision{Kind: sdk.DecisionReply}},
	})
	res, err := engine.Run(context.Background(), messageEvent())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Trace == nil || len(res.Trace.Blocks) != 1 || !res.Trace.Blocks[0].Omitted {
		t.Fatalf("provider budget should omit block: %+v", res.Trace)
	}
}

func TestTurnEnginePolicySilences(t *testing.T) {
	llm := &testkit.FakeLLM{Reply: "不应该调用"}
	sink := &testkit.FakeActionSink{}
	called := false
	engine := kernel.NewTurnEngine(kernel.TurnOptions{
		LLM:      llm,
		Actions:  sink,
		Policies: []sdk.Policy{&fakePolicy{err: errors.New("blocked")}},
		Decider:  &fakeDecider{decision: sdk.Decision{Kind: sdk.DecisionReply}, err: nil},
	})
	_ = called
	res, err := engine.Run(context.Background(), messageEvent())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Decision.Kind != sdk.DecisionSilent {
		t.Fatalf("expected silent, got %+v", res.Decision)
	}
	if len(sink.Actions) != 0 || llm.Calls != 0 {
		t.Fatalf("policy should prevent LLM/actions, calls=%d actions=%d", llm.Calls, len(sink.Actions))
	}
}

func TestKernelSetPluginEnabled(t *testing.T) {
	var order []string
	k := kernel.New(kernel.Options{})
	if err := k.Register(sdk.PluginSpec{New: func() sdk.Plugin {
		return &fakePlugin{manifest: manifest("tool.x", []string{"x"}, nil), order: &order, name: "x"}
	}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = k.Stop(context.Background()) }()

	if err := k.SetPluginEnabled(context.Background(), "tool.x", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	statuses := k.PluginStatuses()
	if len(statuses) != 1 || statuses[0].Enabled {
		t.Fatalf("expected disabled status: %+v", statuses)
	}
	if err := k.SetPluginEnabled(context.Background(), "tool.x", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !k.PluginStatuses()[0].Enabled {
		t.Fatalf("expected enabled status: %+v", k.PluginStatuses())
	}
}

func TestSchedulerCloseUnblocksFullQueue(t *testing.T) {
	ctx := context.Background()
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	s := kernel.NewScheduler(ctx, 1, 1, func(ctx context.Context, ev sdk.Event) error {
		if string(ev.ID) == "block" {
			once.Do(func() { close(started) })
			<-release
		}
		return nil
	})

	conv := sdk.Conversation{ID: "qq:group:1"}
	if err := s.Submit(ctx, sdk.Event{ID: "block", Conversation: conv}); err != nil {
		t.Fatalf("submit block: %v", err)
	}
	<-started
	if err := s.Submit(ctx, sdk.Event{ID: "queued", Conversation: conv}); err != nil {
		t.Fatalf("submit queued: %v", err)
	}

	if err := s.Submit(ctx, sdk.Event{ID: "third", Conversation: conv}); !errors.Is(err, kernel.ErrBackpressure) {
		t.Fatalf("full queue should return ErrBackpressure, got %v", err)
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Close(closeCtx) }()

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after drain")
	}
}

type slowLLM struct{}

func (slowLLM) Chat(ctx context.Context, messages []sdk.Message) (string, error) {
	select {
	case <-time.After(2 * time.Second):
		return "late", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (slowLLM) ChatWithTools(ctx context.Context, messages []sdk.Message, tools []sdk.ToolDefinition, exec sdk.ToolExecutor) (string, error) {
	return slowLLM{}.Chat(ctx, messages)
}

func TestTurnEngineLLMTimeout(t *testing.T) {
	engine := kernel.NewTurnEngine(kernel.TurnOptions{
		LLM:     slowLLM{},
		Decider: &fakeDecider{decision: sdk.Decision{Kind: sdk.DecisionReply, Reason: "test"}},
		Timeout: 30 * time.Millisecond,
	})
	start := time.Now()
	_, err := engine.Run(context.Background(), messageEvent())
	if err == nil {
		t.Fatal("expected LLM timeout error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("LLM timeout took too long: %s", time.Since(start))
	}
}

func TestTurnEngineTraceDecisionOnDeciderError(t *testing.T) {
	engine := kernel.NewTurnEngine(kernel.TurnOptions{
		Decider: &fakeDecider{err: errors.New("boom")},
	})
	res, err := engine.Run(context.Background(), messageEvent())
	if err == nil {
		t.Fatal("expected decider error")
	}
	if res.Decision.Kind != sdk.DecisionSilent || res.Trace.Decision.Kind != sdk.DecisionSilent {
		t.Fatalf("unexpected decision/trace: %+v %+v", res.Decision, res.Trace)
	}
}

func TestSchedulerReclaimsIdleQueues(t *testing.T) {
	done := make(chan struct{})
	s := kernel.NewSchedulerWithIdle(context.Background(), 1, 4, 50*time.Millisecond, func(ctx context.Context, ev sdk.Event) error {
		close(done)
		return nil
	})
	if err := s.Submit(context.Background(), sdk.Event{ID: "e", Conversation: sdk.Conversation{ID: "qq:group:1"}}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-done
	deadline := time.Now().Add(2 * time.Second)
	for s.QueueCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := s.QueueCount(); got != 0 {
		t.Fatalf("idle queue not reclaimed, count=%d", got)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSchedulerSerializesSameConversation(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	var order []string
	s := kernel.NewScheduler(ctx, 2, 8, func(ctx context.Context, ev sdk.Event) error {
		mu.Lock()
		order = append(order, string(ev.ID))
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	for _, id := range []string{"1", "2", "3"} {
		if err := s.Submit(ctx, sdk.Event{ID: sdk.EventID(id), Conversation: sdk.Conversation{ID: "qq:group:1"}}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n == 3 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 || order[0] != "1" || order[1] != "2" || order[2] != "3" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestSchedulerRejectsAfterClose(t *testing.T) {
	s := kernel.NewScheduler(context.Background(), 1, 1, nil)
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Submit(context.Background(), sdk.Event{Conversation: sdk.Conversation{ID: "x"}}); !errors.Is(err, kernel.ErrSchedulerClosed) {
		t.Fatalf("expected ErrSchedulerClosed, got %v", err)
	}
}
