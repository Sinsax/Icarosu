package sdk

import "context"

// ContextBlock 是插件提供给回合的一段上下文。
type ContextBlock struct {
	Source     string
	Role       string // system / user / assistant
	Content    string
	Tokens     int
	Priority   int
	Cacheable  bool
	Provenance []string
}

// Persona 是当前实例的人格快照。
type Persona struct {
	ID           string
	Name         string
	SystemPrompt string
	StyleRules   []string
	Boundaries   []string
	Examples     []string
}

// Turn 是一次回合的上下文。
type Turn struct {
	ID       TurnID
	Event    Event
	Persona  Persona
	Blocks   []ContextBlock
	Tools    []ToolSpec
	Decision Decision
	Actions  []Action
	Trace    *Trace
}

// PersonaProvider 提供当前实例的人格。
type PersonaProvider interface {
	Persona(ctx context.Context) (Persona, error)
}

// AdapterProvider 让插件暴露一个平台 adapter。
type AdapterProvider interface {
	Adapter() Adapter
}

// ContextProvider 提供上下文块。
type ContextProvider interface {
	ID() string
	Priority() int
	Budget() int
	Provide(ctx context.Context, t *Turn) ([]ContextBlock, error)
}

// TurnHook 在回合前后执行同步钩子。
type TurnHook interface {
	BeforeTurn(ctx context.Context, t *Turn) error
	AfterTurn(ctx context.Context, t *Turn, res TurnResult) error
}

// Decider 决定是否回复/静默/观察/延迟。
type Decider interface {
	Decide(ctx context.Context, t *Turn) (Decision, error)
}

// Renderer 把 LLM 回复渲染成 0..N 个 Action。
type Renderer interface {
	Render(ctx context.Context, t *Turn, reply string) ([]Action, error)
}

// Observer 是异步观察者。
type Observer interface {
	Observe(ctx context.Context, ev Event)
}

// Policy 是治理策略，可拒绝/降级。
type Policy interface {
	Check(ctx context.Context, t *Turn) error
}

// OutputPolicy 在 LLM 生成后、发送前检查回复文本。
type OutputPolicy interface {
	CheckOutput(ctx context.Context, t *Turn, reply string) error
}

// EmitFunc 是 adapter 向内核提交事件的回调。
type EmitFunc func(ctx context.Context, ev Event)

// Adapter 是平台适配器能力。
type Adapter interface {
	ID() string
	Start(ctx context.Context, emit EmitFunc) error
	Send(ctx context.Context, a Action) error
	Close() error
	Capabilities() []string
}
