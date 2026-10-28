package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"icarosu/internal/sdk"
)

// SDKClient 把 *Client 适配成 sdk.LLM。
type SDKClient struct {
	inner *Client
}

func NewSDKClient(c *Client) *SDKClient { return &SDKClient{inner: c} }

func (c *SDKClient) Chat(ctx context.Context, messages []sdk.Message) (string, error) {
	if c.inner == nil {
		return "", fmt.Errorf("llm client is nil")
	}
	return c.inner.Chat(ctx, toLLMMessages(messages))
}

func (c *SDKClient) ChatWithTools(ctx context.Context, messages []sdk.Message, tools []sdk.ToolDefinition, exec sdk.ToolExecutor) (string, error) {
	if c.inner == nil {
		return "", fmt.Errorf("llm client is nil")
	}
	var executor ToolExecutor
	if exec != nil {
		executor = func(ctx context.Context, name, arguments string) (string, error) {
			return exec(ctx, name, arguments)
		}
	}
	return c.inner.ChatWithTools(ctx, toLLMMessages(messages), toLLMTools(tools), executor)
}

func toLLMMessages(messages []sdk.Message) []Message {
	out := make([]Message, 0, len(messages))
	for _, m := range messages {
		msg := Message{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		out = append(out, msg)
	}
	return out
}

func toLLMTools(tools []sdk.ToolDefinition) []Tool {
	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		var params map[string]any
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &params)
		}
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

var _ sdk.LLM = (*SDKClient)(nil)
