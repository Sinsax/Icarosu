package qq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// MessageHandler 是收到 OneBot 事件后的回调。
type MessageHandler func(ctx context.Context, evt Event)

// Client 是 OneBot v11 WebSocket 客户端。
// 当前支持正向 WebSocket：机器人作为客户端连接 OneBot 的 WS 地址。
type Client struct {
	url     string
	token   string
	logger  *slog.Logger
	handler MessageHandler

	connMu sync.RWMutex
	conn   *websocket.Conn

	sendMu    sync.Mutex
	pending   map[string]chan []byte
	echoID    int64
	pendingMu sync.Mutex

	closeOnce sync.Once
	closed    atomic.Bool
}

func NewClient(url, token string, logger *slog.Logger) *Client {
	return &Client{
		url:     url,
		token:   token,
		logger:  logger,
		pending: make(map[string]chan []byte),
	}
}

// SetMessageHandler 设置事件回调。
func (c *Client) SetMessageHandler(h MessageHandler) {
	c.handler = h
}

// Connect 建立 WebSocket 连接并启动读取循环。
func (c *Client) Connect(ctx context.Context) error {
	c.closed.Store(false)

	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, header)
	if err != nil {
		return fmt.Errorf("dial onebot ws %q: %w", c.url, err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	c.logger.Info("onebot connected", "url", c.url)
	go c.readLoop(ctx)

	return nil
}

// Close 关闭连接。
func (c *Client) Close() error {
	c.closed.Store(true)
	var err error
	c.closeOnce.Do(func() {
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()
		if conn != nil {
			err = conn.Close()
		}
	})
	return err
}

// SendGroupMessage 发送群消息。
func (c *Client) SendGroupMessage(ctx context.Context, groupID int64, message string) error {
	_, err := c.sendRequest(ctx, "send_group_msg", map[string]any{
		"group_id": groupID,
		"message":  message,
	})
	return err
}

// SendPrivateMessage 发送私聊消息。
func (c *Client) SendPrivateMessage(ctx context.Context, userID int64, message string) error {
	_, err := c.sendRequest(ctx, "send_private_msg", map[string]any{
		"user_id": userID,
		"message": message,
	})
	return err
}

// SendGroupImage 发送群图片消息。
func (c *Client) SendGroupImage(ctx context.Context, groupID int64, file string) error {
	_, err := c.sendRequest(ctx, "send_group_msg", map[string]any{
		"group_id": groupID,
		"message": []map[string]any{
			{"type": "image", "data": map[string]any{"file": file}},
		},
	})
	return err
}

// SendPrivateImage 发送私聊图片消息。
func (c *Client) SendPrivateImage(ctx context.Context, userID int64, file string) error {
	_, err := c.sendRequest(ctx, "send_private_msg", map[string]any{
		"user_id": userID,
		"message": []map[string]any{
			{"type": "image", "data": map[string]any{"file": file}},
		},
	})
	return err
}

func (c *Client) readLoop(ctx context.Context) {
	for {
		if c.closed.Load() {
			return
		}

		_, data, err := c.readMessage()
		if err != nil {
			if c.closed.Load() {
				return
			}
			c.logger.Error("onebot read error", "error", err)
			if !c.reconnect(ctx) {
				return
			}
			continue
		}

		var envelope struct {
			Echo string `json:"echo"`
		}
		if err := json.Unmarshal(data, &envelope); err == nil && envelope.Echo != "" {
			c.resolvePending(envelope.Echo, data)
			continue
		}

		var evt Event
		if err := json.Unmarshal(data, &evt); err != nil {
			c.logger.Warn("failed to parse onebot event", "error", err, "raw", string(data))
			continue
		}
		if evt.PostType == "" {
			continue
		}

		if c.handler != nil {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						c.logger.Error("handler panic", "panic", r)
					}
				}()
				c.handler(context.Background(), evt)
			}()
		}
	}
}

// reconnect 自动重连，带指数退避。
func (c *Client) reconnect(ctx context.Context) bool {
	backoff := time.Second
	for {
		if c.closed.Load() {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		header := http.Header{}
		if c.token != "" {
			header.Set("Authorization", "Bearer "+c.token)
		}
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, header)
		if err == nil {
			c.connMu.Lock()
			c.conn = conn
			c.connMu.Unlock()
			c.logger.Info("onebot reconnected", "url", c.url)
			return true
		}

		c.logger.Warn("onebot reconnect failed", "error", err, "backoff", backoff.String())
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (c *Client) readMessage() (int, []byte, error) {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return 0, nil, fmt.Errorf("onebot connection is nil")
	}
	return conn.ReadMessage()
}

func (c *Client) resolvePending(echo string, data []byte) {
	c.pendingMu.Lock()
	ch, ok := c.pending[echo]
	if ok {
		delete(c.pending, echo)
	}
	c.pendingMu.Unlock()

	if ok {
		ch <- data
		close(ch)
	}
}

func (c *Client) sendRequest(ctx context.Context, action string, params map[string]any) ([]byte, error) {
	c.pendingMu.Lock()
	c.echoID++
	echo := fmt.Sprintf("%d", c.echoID)
	c.pendingMu.Unlock()

	req := map[string]any{
		"action": action,
		"params": params,
		"echo":   echo,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return nil, fmt.Errorf("onebot connection is nil")
	}

	c.sendMu.Lock()
	err = conn.WriteMessage(websocket.TextMessage, payload)
	c.sendMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write onebot request: %w", err)
	}

	// 当前发送采用 fire-and-forget，不等待 API 响应。
	// 避免 readLoop 在处理消息时因等待响应而阻塞，导致机器人假死。
	return nil, nil
}

// sendRequestWithResponse 发送 API 请求并等待响应。
// readLoop 已经异步处理事件，因此这里等待响应不会阻塞消息接收。
func (c *Client) sendRequestWithResponse(ctx context.Context, action string, params map[string]any) ([]byte, error) {
	c.pendingMu.Lock()
	c.echoID++
	echo := fmt.Sprintf("%d", c.echoID)
	ch := make(chan []byte, 1)
	c.pending[echo] = ch
	c.pendingMu.Unlock()

	cleanup := func() {
		c.pendingMu.Lock()
		if _, ok := c.pending[echo]; ok {
			delete(c.pending, echo)
			close(ch)
		}
		c.pendingMu.Unlock()
	}

	req := map[string]any{
		"action": action,
		"params": params,
		"echo":   echo,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		cleanup()
		return nil, err
	}

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		cleanup()
		return nil, fmt.Errorf("onebot connection is nil")
	}

	c.sendMu.Lock()
	err = conn.WriteMessage(websocket.TextMessage, payload)
	c.sendMu.Unlock()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("write onebot request: %w", err)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	}
}

// GetGroupMemberList 获取群成员列表。
func (c *Client) GetGroupMemberList(ctx context.Context, groupID int64) ([]GroupMember, error) {
	data, err := c.sendRequestWithResponse(ctx, "get_group_member_list", map[string]any{
		"group_id": groupID,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []GroupMember `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse group member list: %w", err)
	}
	return resp.Data, nil
}
