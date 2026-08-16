package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"icarosu/internal/platform"
	"icarosu/internal/qq"
)

// Client 是一个极简 Telegram Bot API 适配器，使用 long polling 接收消息。
type Client struct {
	token   string
	baseURL string
	http    *http.Client
	logger  *slog.Logger

	handler platform.MessageHandler

	mu     sync.Mutex
	offset int64

	closed atomic.Bool
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClient(token string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		token:   token,
		baseURL: "https://api.telegram.org",
		http:    &http.Client{Timeout: 60 * time.Second},
		logger:  logger,
	}
}

func (c *Client) SetMessageHandler(h platform.MessageHandler) {
	c.handler = h
}

func (c *Client) Connect(ctx context.Context) error {
	if c.token == "" {
		return fmt.Errorf("telegram token is empty")
	}
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.closed.Store(false)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.pollLoop(ctx)
	}()
	c.logger.Info("telegram adapter started")
	return nil
}

func (c *Client) Close() error {
	c.closed.Store(true)
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *Client) SendGroupMessage(ctx context.Context, groupID int64, message string) error {
	return c.sendMessage(ctx, groupID, message)
}

func (c *Client) SendPrivateMessage(ctx context.Context, userID int64, message string) error {
	return c.sendMessage(ctx, userID, message)
}

func (c *Client) SendGroupImage(ctx context.Context, groupID int64, file string) error {
	return c.sendPhoto(ctx, groupID, file)
}

func (c *Client) SendPrivateImage(ctx context.Context, userID int64, file string) error {
	return c.sendPhoto(ctx, userID, file)
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID int    `json:"message_id"`
	From      *user  `json:"from"`
	Chat      *chat  `json:"chat"`
	Text      string `json:"text"`
}

type user struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type apiResponse struct {
	OK          bool     `json:"ok"`
	Result      []update `json:"result"`
	Description string   `json:"description"`
}

func (c *Client) pollLoop(ctx context.Context) {
	for {
		if c.closed.Load() {
			return
		}
		updates, err := c.getUpdates(ctx, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Warn("telegram getUpdates failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			if u.Message == nil {
				continue
			}
			c.handleMessage(ctx, *u.Message)
		}
	}
}

func (c *Client) getUpdates(ctx context.Context, timeout int) ([]update, error) {
	c.mu.Lock()
	offset := c.offset
	c.mu.Unlock()

	u := url.Values{}
	u.Set("timeout", strconv.Itoa(timeout))
	u.Set("offset", strconv.FormatInt(offset, 10))
	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?%s", c.baseURL, c.token, u.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("telegram api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var ar apiResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, err
	}
	if !ar.OK {
		return nil, fmt.Errorf("telegram api error: %s", ar.Description)
	}

	if len(ar.Result) > 0 {
		c.mu.Lock()
		if last := ar.Result[len(ar.Result)-1].UpdateID; last >= c.offset {
			c.offset = last + 1
		}
		c.mu.Unlock()
	}
	return ar.Result, nil
}

func (c *Client) handleMessage(ctx context.Context, msg message) {
	if msg.From == nil || msg.Chat == nil {
		return
	}
	evt := qq.Event{
		PostType:    "message",
		MessageType: "private",
		UserID:      msg.From.ID,
		MessageID:   int64(msg.MessageID),
		RawMessage:  msg.Text,
		Sender: qq.Sender{
			UserID:   msg.From.ID,
			Nickname: strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName),
		},
	}
	if msg.Chat.Type == "group" || msg.Chat.Type == "supergroup" {
		evt.MessageType = "group"
		evt.GroupID = msg.Chat.ID
	}

	if c.handler != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Error("telegram handler panic", "panic", r)
				}
			}()
			c.handler(ctx, evt)
		}()
	}
}

func (c *Client) sendMessage(ctx context.Context, chatID int64, text string) error {
	form := url.Values{}
	form.Set("chat_id", strconv.FormatInt(chatID, 10))
	form.Set("text", text)
	return c.postForm(ctx, "sendMessage", form)
}

func (c *Client) sendPhoto(ctx context.Context, chatID int64, file string) error {
	form := url.Values{}
	form.Set("chat_id", strconv.FormatInt(chatID, 10))
	form.Set("photo", file)
	return c.postForm(ctx, "sendPhoto", form)
}

func (c *Client) postForm(ctx context.Context, method string, form url.Values) error {
	endpoint := fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram %s returned %d: %s", method, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

var _ platform.Adapter = (*Client)(nil)
