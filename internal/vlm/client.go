package vlm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Config 是 VLM 图片识别客户端配置。
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Prompt  string
	Timeout time.Duration
}

// Result 是 VLM 对一张图片的判断结果。
type Result struct {
	Description string   `json:"description"`
	IsMeme      bool     `json:"is_meme"`
	FitsPersona bool     `json:"fits_persona"`
	Reason      string   `json:"reason"`
	Tags        []string `json:"tags"`
	Name        string   `json:"name"`
}

type contentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

type requestMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type request struct {
	Model       string           `json:"model"`
	Messages    []requestMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
}

type response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client 是轻量 OpenAI 兼容 VLM 客户端。
type Client struct {
	config Config
	http   *http.Client
	logger *slog.Logger
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

// Ping 用一个极小的文本请求检查 VLM API 是否可用。
func (c *Client) Ping(ctx context.Context) error {
	if c.config.BaseURL == "" || c.config.Model == "" {
		return fmt.Errorf("vlm base_url/model is empty")
	}

	body, err := json.Marshal(request{
		Model: c.config.Model,
		Messages: []requestMessage{
			{Role: "user", Content: []contentPart{{Type: "text", Text: "ping"}}},
		},
		MaxTokens:   1,
		Temperature: 0,
	})
	if err != nil {
		return fmt.Errorf("marshal vlm ping: %w", err)
	}

	url := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create vlm ping: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("vlm ping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vlm ping returned %d", resp.StatusCode)
	}
	return nil
}

// Analyze 识别一张图片，返回是否适合收藏为表情包。
func (c *Client) Analyze(ctx context.Context, imageURL, persona string) (*Result, error) {
	if c.config.BaseURL == "" || c.config.Model == "" {
		return nil, fmt.Errorf("vlm base_url/model is empty")
	}

	prompt := c.config.Prompt
	if persona != "" {
		prompt += "\n机器人人设：" + persona
	}

	body, err := json.Marshal(request{
		Model: c.config.Model,
		Messages: []requestMessage{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "text", Text: prompt},
					{
						Type: "image_url",
						ImageURL: &struct {
							URL string `json:"url"`
						}{URL: imageURL},
					},
				},
			},
		},
		MaxTokens:   300,
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal vlm request: %w", err)
	}

	url := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create vlm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call vlm api: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read vlm response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vlm api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var result response
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse vlm response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("vlm api error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("vlm response has no choices")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var out Result
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("parse vlm result: %w, content=%s", err, content)
	}
	return &out, nil
}
