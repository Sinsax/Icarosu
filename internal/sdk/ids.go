package sdk

// ActorID 是平台内唯一的 actor 标识，例如 "qq:user:123"。
type ActorID string

// ConversationID 是平台内唯一的会话标识，例如 "qq:group:123"。
type ConversationID string

// AdapterID 是平台适配器标识，例如 "onebot"。
type AdapterID string

// EventID 是平台事件标识（用于去重）。
type EventID string

// TurnID 是一次回合的标识。
type TurnID string

// ToolCallID 是 LLM 工具调用标识。
type ToolCallID string
