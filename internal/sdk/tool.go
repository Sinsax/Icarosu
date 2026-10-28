package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ToolSpec 是 manifest 里的工具声明。
type ToolSpec struct {
	Name        string   `yaml:"name" json:"name"`
	Summary     string   `yaml:"summary" json:"summary"`
	Schema      string   `yaml:"schema" json:"schema"`
	Manual      string   `yaml:"manual" json:"manual"`
	Permissions []string `yaml:"permissions" json:"permissions"`
	TimeoutMS   int      `yaml:"timeout_ms" json:"timeout_ms"`
	SideEffect  string   `yaml:"side_effect" json:"side_effect"`
}

// Tool 是运行期可执行工具。
type Tool struct {
	Name        string
	Summary     string
	Schema      json.RawMessage
	Manual      string
	Permissions []string
	Timeout     time.Duration
	Execute     func(ctx context.Context, call ToolCall) (ToolResult, error)
}

// ToolCall 是一次工具调用。
type ToolCall struct {
	ID           ToolCallID
	Name         string
	Arguments    json.RawMessage
	Conversation Conversation
	Actor        Actor
	TurnID       TurnID
}

// ToolResult 是工具执行结果。
type ToolResult struct {
	Text  string
	Parts []Part
	Meta  map[string]string
}

// Text 构造纯文本工具结果。
func Text(s string) ToolResult { return ToolResult{Text: s} }

// ToolRegistry 是 Host 暴露给插件的工具注册能力。
type ToolRegistry interface {
	Register(t Tool) error
	List() []ToolSpec
}

// ValidateToolName 检查 LLM function name 兼容性。
func ValidateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("tool name is empty")
	}
	for i, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return fmt.Errorf("tool name %q has invalid character %q at %d", name, r, i)
	}
	if name[0] >= '0' && name[0] <= '9' {
		return fmt.Errorf("tool name %q must not start with a digit", name)
	}
	return nil
}
