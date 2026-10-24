package llm

import "context"

// Provider 是上层业务依赖的 LLM 能力接口。
// 目前由 *Client 实现；测试或未来接入其他模型服务时可以提供替代实现。
type Provider interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool, executor ToolExecutor) (string, error)
}

var _ Provider = (*Client)(nil)