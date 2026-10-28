package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"icarosu/internal/sdk"
)

// TurnOptions 是回合引擎的依赖。
type TurnOptions struct {
	Logger           *slog.Logger
	Clock            sdk.Clock
	LLM              sdk.LLM
	Actions          sdk.ActionSink
	Tools            *toolRegistry
	Persona          sdk.Persona
	Providers        []sdk.ContextProvider
	Hooks            []sdk.TurnHook
	Policies         []sdk.Policy
	OutputPolicies   []sdk.OutputPolicy
	Decider          sdk.Decider
	Renderer         sdk.Renderer
	MaxContextTokens int
	Timeout          time.Duration
}

// TurnEngine 执行一次“观察 → 上下文 → 决策 → 生成 → 发送”的回合。
type TurnEngine struct {
	opts  TurnOptions
	tools *toolRegistry
	seq   atomic.Uint64
}

// NewTurnEngine 创建回合引擎。
func NewTurnEngine(opts TurnOptions) *TurnEngine {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Clock == nil {
		opts.Clock = realClock{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.MaxContextTokens <= 0 {
		opts.MaxContextTokens = 3000
	}
	tools := opts.Tools
	if tools == nil {
		tools = newToolRegistry(opts.Logger, nil)
	}
	return &TurnEngine{opts: opts, tools: tools}
}

// RegisterTool 注册一个工具。
func (e *TurnEngine) RegisterTool(t sdk.Tool) error { return e.tools.Register(t) }

func (e *TurnEngine) nextTurnID() sdk.TurnID {
	return sdk.TurnID(fmt.Sprintf("t_%d_%d", time.Now().UnixNano(), e.seq.Add(1)))
}

// Run 执行一个回合。
func (e *TurnEngine) Run(ctx context.Context, ev sdk.Event) (sdk.TurnResult, error) {
	started := time.Now()
	turn := &sdk.Turn{
		ID:      e.nextTurnID(),
		Event:   ev,
		Persona: e.opts.Persona,
	}
	trace := &sdk.Trace{
		TurnID:         turn.ID,
		EventID:        ev.ID,
		ConversationID: ev.Conversation.ID,
		ActorID:        ev.Actor.ID,
		StartedAt:      started,
	}
	turn.Trace = trace
	defer func() {
		trace.LatencyMS = time.Since(started).Milliseconds()
	}()

	for _, hook := range e.opts.Hooks {
		if err := e.callHook(ctx, hook, true, turn); err != nil {
			trace.Errors = append(trace.Errors, sdk.TraceError{Stage: "hook.before", Code: string(sdk.ErrPluginPanic), Message: err.Error()})
			decision := sdk.Decision{Kind: sdk.DecisionSilent, Reason: "hook_error"}
			turn.Decision = decision
			trace.Decision = decision
			return e.finish(ctx, turn, decision, err)
		}
	}

	providers := append([]sdk.ContextProvider(nil), e.opts.Providers...)
	sort.SliceStable(providers, func(i, j int) bool { return providers[i].Priority() > providers[j].Priority() })
	remaining := e.opts.MaxContextTokens
	for _, p := range providers {
		blocks, err := e.callProvider(ctx, p, turn)
		if err != nil {
			trace.Errors = append(trace.Errors, sdk.TraceError{Stage: "context." + p.ID(), Code: "context_error", Message: err.Error()})
			continue
		}
		providerRemaining := remaining
		if budget := p.Budget(); budget > 0 && budget < providerRemaining {
			providerRemaining = budget
		}
		for _, b := range blocks {
			if b.Role == "" {
				b.Role = "system"
			}
			if b.Tokens <= 0 {
				b.Tokens = len([]rune(b.Content))/3 + 1
			}
			if b.Tokens > providerRemaining {
				trace.Blocks = append(trace.Blocks, sdk.TraceBlock{Source: b.Source, Tokens: b.Tokens, Priority: b.Priority, Omitted: true})
				continue
			}
			providerRemaining -= b.Tokens
			remaining -= b.Tokens
			turn.Blocks = append(turn.Blocks, b)
			trace.Blocks = append(trace.Blocks, sdk.TraceBlock{Source: b.Source, Tokens: b.Tokens, Priority: b.Priority})
		}
	}

	for _, policy := range e.opts.Policies {
		if err := e.callPolicy(ctx, policy, turn); err != nil {
			decision := sdk.Decision{Kind: sdk.DecisionSilent, Reason: "policy:" + err.Error()}
			turn.Decision = decision
			trace.Decision = decision
			return e.finish(ctx, turn, decision, nil)
		}
	}

	decision, err := e.decide(ctx, turn)
	if err != nil {
		decision = sdk.Decision{Kind: sdk.DecisionSilent, Reason: "decider_error"}
		turn.Decision = decision
		trace.Decision = decision
		trace.Errors = append(trace.Errors, sdk.TraceError{Stage: "decider", Code: string(sdk.ErrPluginPanic), Message: err.Error()})
		return e.finish(ctx, turn, decision, err)
	}
	turn.Decision = decision
	trace.Decision = decision
	if decision.Kind != sdk.DecisionReply {
		return e.finish(ctx, turn, decision, nil)
	}

	reply, err := e.generate(ctx, turn, trace)
	if err != nil {
		return e.finish(ctx, turn, decision, err)
	}
	if err := e.checkOutput(ctx, turn, reply); err != nil {
		silent := sdk.Decision{Kind: sdk.DecisionSilent, Reason: "output_policy"}
		turn.Decision = silent
		trace.Decision = silent
		trace.Errors = append(trace.Errors, sdk.TraceError{Stage: "output_policy", Code: string(sdk.ErrPermission), Message: err.Error()})
		return e.finish(ctx, turn, silent, nil)
	}
	actions, err := e.render(ctx, turn, reply)
	if err != nil {
		return e.finish(ctx, turn, decision, err)
	}
	for i := range actions {
		if actions[i].ID == "" {
			actions[i].ID = fmt.Sprintf("%s:%d", turn.ID, i)
		}
		if actions[i].TurnID == "" {
			actions[i].TurnID = turn.ID
		}
		if actions[i].Conversation.ID == "" {
			actions[i].Conversation = ev.Conversation
		}
		if actions[i].Actor == "" {
			actions[i].Actor = ev.Self
		}
		if actions[i].IdempotencyKey == "" {
			actions[i].IdempotencyKey = actions[i].ID
		}
		turn.Actions = append(turn.Actions, actions[i])
		if e.opts.Actions != nil {
			ok := true
			var sendErr string
			sendCtx, cancel := context.WithTimeout(ctx, e.opts.Timeout)
			err := e.opts.Actions.Send(sendCtx, actions[i])
			cancel()
			if err != nil {
				ok = false
				sendErr = err.Error()
			}
			trace.Actions = append(trace.Actions, sdk.TraceAction{
				Adapter: string(actions[i].Conversation.AdapterID),
				Parts:   len(actions[i].Parts),
				OK:      ok,
				Error:   sendErr,
			})
		}
	}

	return e.finish(ctx, turn, decision, nil)
}

func (e *TurnEngine) finish(ctx context.Context, turn *sdk.Turn, decision sdk.Decision, err error) (sdk.TurnResult, error) {
	for _, hook := range e.opts.Hooks {
		if hookErr := e.callHook(ctx, hook, false, turn); hookErr != nil {
			turn.Trace.Errors = append(turn.Trace.Errors, sdk.TraceError{Stage: "hook.after", Code: string(sdk.ErrPluginPanic), Message: hookErr.Error()})
		}
	}
	return sdk.TurnResult{TurnID: turn.ID, Decision: decision, Actions: turn.Actions, Trace: turn.Trace}, err
}

func (e *TurnEngine) decide(ctx context.Context, turn *sdk.Turn) (sdk.Decision, error) {
	if e.opts.Decider == nil {
		return defaultDecision(turn.Event), nil
	}
	c, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	var decision sdk.Decision
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("decider panic: %v", r)
			}
		}()
		decision, err = e.opts.Decider.Decide(c, turn)
	}()
	if err != nil {
		return sdk.Decision{}, err
	}
	if decision.Kind == "" {
		decision.Kind = sdk.DecisionReply
	}
	return decision, nil
}

func (e *TurnEngine) generate(ctx context.Context, turn *sdk.Turn, trace *sdk.Trace) (string, error) {
	if e.opts.LLM == nil {
		return "", fmt.Errorf("llm is not configured")
	}
	turn.Tools = e.tools.List()
	messages := make([]sdk.Message, 0, len(turn.Blocks)+2)
	if turn.Persona.SystemPrompt != "" {
		messages = append(messages, sdk.Message{Role: "system", Content: turn.Persona.SystemPrompt})
	}
	for _, b := range turn.Blocks {
		messages = append(messages, sdk.Message{Role: b.Role, Content: b.Content})
	}
	messages = append(messages, sdk.Message{Role: "user", Content: eventText(turn.Event)})

	started := time.Now()
	llmCtx, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	var reply string
	var err error
	var calledTools []string
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("llm panic: %v", r)
			}
		}()
		reply, err = e.opts.LLM.ChatWithTools(llmCtx, messages, e.tools.Definitions(), func(ctx context.Context, name, args string) (string, error) {
			toolStarted := time.Now()
			res, toolErr := e.tools.Execute(ctx, sdk.ToolCall{Name: name, Arguments: []byte(args), TurnID: turn.ID, Conversation: turn.Event.Conversation, Actor: turn.Event.Actor})
			trace.Tools = append(trace.Tools, sdk.TraceTool{
				Name: name, LatencyMS: time.Since(toolStarted).Milliseconds(),
				OK: toolErr == nil, Error: errString(toolErr),
			})
			calledTools = append(calledTools, name)
			if toolErr != nil {
				return "", toolErr
			}
			return ResultText(res), nil
		})
	}()
	trace.LLM = append(trace.LLM, sdk.TraceLLM{Round: 1, Tools: calledTools, LatencyMS: time.Since(started).Milliseconds(), Error: errString(err)})
	if err != nil {
		return "", err
	}
	return reply, nil
}

func (e *TurnEngine) render(ctx context.Context, turn *sdk.Turn, reply string) ([]sdk.Action, error) {
	if e.opts.Renderer == nil {
		return []sdk.Action{{
			Conversation: turn.Event.Conversation,
			Actor:        turn.Event.Self,
			Parts:        []sdk.Part{{Type: sdk.PartText, Text: reply}},
		}}, nil
	}
	c, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	var actions []sdk.Action
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("renderer panic: %v", r)
			}
		}()
		actions, err = e.opts.Renderer.Render(c, turn, reply)
	}()
	return actions, err
}

func (e *TurnEngine) callProvider(ctx context.Context, p sdk.ContextProvider, turn *sdk.Turn) (blocks []sdk.ContextBlock, err error) {
	c, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("context provider panic: %v", r)
		}
	}()
	return p.Provide(c, turn)
}

func (e *TurnEngine) callHook(ctx context.Context, h sdk.TurnHook, before bool, turn *sdk.Turn) (err error) {
	c, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("turn hook panic: %v", r)
		}
	}()
	if before {
		return h.BeforeTurn(c, turn)
	}
	return h.AfterTurn(c, turn, sdk.TurnResult{TurnID: turn.ID, Decision: turn.Decision, Actions: turn.Actions, Trace: turn.Trace})
}

func (e *TurnEngine) checkOutput(ctx context.Context, turn *sdk.Turn, reply string) error {
	for _, policy := range e.opts.OutputPolicies {
		c, cancel := context.WithTimeout(ctx, e.opts.Timeout)
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("output policy panic: %v", r)
				}
			}()
			return policy.CheckOutput(c, turn, reply)
		}()
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *TurnEngine) callPolicy(ctx context.Context, p sdk.Policy, turn *sdk.Turn) (err error) {
	c, cancel := context.WithTimeout(ctx, e.opts.Timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("policy panic: %v", r)
		}
	}()
	return p.Check(c, turn)
}

func defaultDecision(ev sdk.Event) sdk.Decision {
	if ev.Kind != "message.created" {
		return sdk.Decision{Kind: sdk.DecisionObserve, Reason: "non_message"}
	}
	if ev.Conversation.Kind == "private" {
		return sdk.Decision{Kind: sdk.DecisionReply, Reason: "private"}
	}
	for _, m := range ev.Mentions {
		if m == ev.Self {
			return sdk.Decision{Kind: sdk.DecisionReply, Reason: "at_bot"}
		}
	}
	return sdk.Decision{Kind: sdk.DecisionObserve, Reason: "not_addressed"}
}

func eventText(ev sdk.Event) string {
	var b strings.Builder
	for _, p := range ev.Parts {
		switch p.Type {
		case sdk.PartText:
			b.WriteString(p.Text)
		case sdk.PartImage:
			b.WriteString("[图片]")
		case sdk.PartAudio:
			b.WriteString("[语音]")
		case sdk.PartFile:
			b.WriteString("[文件]")
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "（对方发来非文本消息）"
	}
	return text
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
