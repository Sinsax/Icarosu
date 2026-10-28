package sdk

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// LLMToolCall 是模型返回的工具调用请求。
type LLMToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Message 是平台中立的 LLM 对话消息。
type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []LLMToolCall
}

// ToolDefinition 是 LLM function-calling 定义。
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ToolExecutor 执行一次 LLM 工具调用。
type ToolExecutor func(ctx context.Context, name, arguments string) (string, error)

// LLM 是 Host 暴露的模型能力。
type LLM interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []ToolDefinition, exec ToolExecutor) (string, error)
}

// MemoryItem 是一条记忆。
type MemoryItem struct {
	ID        string
	Content   string
	Scope     string
	Kind      string
	Score     float64
	CreatedAt time.Time
}

// Memory 是 Host 暴露的记忆能力。
type Memory interface {
	Recall(ctx context.Context, query string, limit int) ([]MemoryItem, error)
	Remember(ctx context.Context, item MemoryItem) error
}

// ActionSink 是 Host 暴露的动作出口。
type ActionSink interface {
	Send(ctx context.Context, a Action) error
}

// Storage 是 Host 暴露的命名空间存储能力。
type Storage interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// Bus 是 Host 暴露的事件总线。
type Bus interface {
	Publish(ctx context.Context, ev Event) error
	Subscribe(kind string, fn Observer) (unsubscribe func(), err error)
}

// Clock 是可注入时钟。
type Clock interface {
	Now() time.Time
}

// Host 是插件可用的能力入口。插件只能通过 Host 产生副作用。
type Host interface {
	ID() string
	Has(capability string) bool
	Bus() Bus
	LLM() LLM
	Memory() Memory
	Tools() ToolRegistry
	Actions() ActionSink
	Storage(namespace string) (Storage, error)
	Config(section string, dst any) error
	Logger() *slog.Logger
	Clock() Clock
}
