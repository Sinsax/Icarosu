package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Config 是 OpenAI 兼容 API 客户端的配置。
type Config struct {
	BaseURL        string
	APIKey         string
	Model          string
	Temperature    float64
	MaxTokens      int
	Timeout        time.Duration
	CachePrefix    bool
	CacheMaxTokens int
	LogCache       bool
}

// Tool 是可供模型调用的工具定义。
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 是工具函数定义。
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall 是模型返回的工具调用请求。
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction 是工具调用的函数部分。
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message 是对话消息。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// Response 是一次模型返回的完整结果。
type Response struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
}

// ToolExecutor 执行一个工具调用，返回工具结果文本。
type ToolExecutor func(ctx context.Context, name, arguments string) (string, error)

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

type responseMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Reasoning        json.RawMessage `json:"reasoning,omitempty"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
}

type usage struct {
	PromptTokens          int `json:"prompt_tokens,omitempty"`
	CompletionTokens      int `json:"completion_tokens,omitempty"`
	TotalTokens           int `json:"total_tokens,omitempty"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message responseMessage `json:"message"`
	} `json:"choices"`
	Usage *usage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client 是一个轻量 OpenAI 兼容 Chat Completions 客户端。
type Client struct {
	config Config
	http   *http.Client
	logger *slog.Logger

	requests atomic.Int64
	tokens   atomic.Int64
	errors   atomic.Int64
}

// Stats 是 LLM 客户端的累计统计。
type Stats struct {
	Requests int64 `json:"requests"`
	Tokens   int64 `json:"tokens"`
	Errors   int64 `json:"errors"`
}

// Stats 返回当前累计统计。
func (c *Client) Stats() Stats {
	return Stats{
		Requests: c.requests.Load(),
		Tokens:   c.tokens.Load(),
		Errors:   c.errors.Load(),
	}
}

func NewClient(cfg Config, logger *slog.Logger) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: timeout},
		logger: logger,
	}
}

// UpdateConfig 热更新客户端配置（超时、模型、密钥等）。
func (c *Client) UpdateConfig(cfg Config) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	c.config = cfg
	c.http.Timeout = timeout
}

// Chat 发送一轮对话，返回模型回复文本（不启用工具调用）。
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	return c.ChatWithTools(ctx, messages, nil, nil)
}

// ChatWithTools 发送对话，并在模型请求工具时自动循环执行工具调用，直到模型给出最终文本。
func (c *Client) ChatWithTools(ctx context.Context, messages []Message, tools []Tool, executor ToolExecutor) (string, error) {
	msgs := append([]Message(nil), messages...)

	const maxToolRounds = 5
	for round := 0; round < maxToolRounds; round++ {
		resp, err := c.completeWithRetry(ctx, msgs, tools)
		if err != nil {
			return "", err
		}

		if resp.Reasoning != "" {
			c.logger.Info("llm reasoning", "reasoning", resp.Reasoning)
		}

		if len(resp.ToolCalls) == 0 {
			if resp.Content == "" {
				return "", fmt.Errorf("llm returned empty reply")
			}
			c.logger.Info("llm reply", "reply", resp.Content)
			return resp.Content, nil
		}

		if executor == nil {
			return "", fmt.Errorf("llm requested tools but no executor provided")
		}

		c.logger.Info("tool calls", "count", len(resp.ToolCalls))
		msgs = append(msgs, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			c.logger.Info("tool call", "name", tc.Function.Name, "arguments", tc.Function.Arguments)

			result, err := executor(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("tool error: %v", err)
				c.logger.Warn("tool call failed", "name", tc.Function.Name, "error", err)
			}

			c.logger.Info("tool result", "name", tc.Function.Name, "result", result)
			msgs = append(msgs, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	return "", fmt.Errorf("llm tool calling exceeded max rounds")
}

// completeWithRetry 调用 Complete，并对可重试错误做简单退避重试。
func (c *Client) completeWithRetry(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.Complete(ctx, messages, tools)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !shouldRetryLLM(err) || attempt == maxRetries {
			break
		}
		wait := time.Duration(attempt+1) * 500 * time.Millisecond
		c.logger.Warn("llm request failed, retrying", "attempt", attempt+1, "error", err, "wait", wait.String())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, lastErr
}

func shouldRetryLLM(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// 鉴权/参数类错误不重试
	if strings.Contains(s, "401") || strings.Contains(s, "403") || strings.Contains(s, "400") {
		return false
	}
	return true
}

// Complete 发送一次请求，返回包含思考过程、文本和工具调用的完整响应。
func (c *Client) Complete(ctx context.Context, messages []Message, tools []Tool) (resp *Response, err error) {
	c.requests.Add(1)
	defer func() {
		if err != nil {
			c.errors.Add(1)
		}
	}()

	if c.config.BaseURL == "" {
		return nil, fmt.Errorf("llm base_url is empty")
	}
	if c.config.Model == "" {
		return nil, fmt.Errorf("llm model is empty")
	}

	url := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"

	body, err := json.Marshal(chatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Stream:      false,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal llm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	c.logger.Debug("llm request", "model", c.config.Model, "messages", len(messages), "tools", len(tools))

	httpResp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call llm api: %w", err)
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm api returned %d: %s", httpResp.StatusCode, strings.TrimSpace(string(data)))
	}

	var result chatResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("llm api error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("llm response has no choices")
	}

	msg := result.Choices[0].Message

	if result.Usage != nil {
		c.tokens.Add(int64(result.Usage.TotalTokens))
	}

	if c.config.LogCache && result.Usage != nil {
		hit := result.Usage.PromptCacheHitTokens
		miss := result.Usage.PromptCacheMissTokens
		if hit > 0 || miss > 0 {
			ratio := 0.0
			if total := hit + miss; total > 0 {
				ratio = float64(hit) / float64(total) * 100
			}
			c.logger.Info("llm cache",
				"hit_tokens", hit,
				"miss_tokens", miss,
				"hit_ratio", fmt.Sprintf("%.1f%%", ratio),
				"prompt_tokens", result.Usage.PromptTokens,
				"completion_tokens", result.Usage.CompletionTokens,
				"total_tokens", result.Usage.TotalTokens,
			)
		}
	}

	return &Response{
		Content:   strings.TrimSpace(msg.Content),
		Reasoning: extractReasoning(msg),
		ToolCalls: msg.ToolCalls,
	}, nil
}

func extractReasoning(msg responseMessage) string {
	if s := strings.TrimSpace(msg.ReasoningContent); s != "" {
		return s
	}
	if len(msg.Reasoning) > 0 && string(msg.Reasoning) != "null" {
		return string(msg.Reasoning)
	}
	return ""
}
