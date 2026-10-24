package plugin

import (
	"context"
	"sync"
)

// Bot 是插件可用的机器人能力接口。
// 后续需要更多能力（发图片、撤回、管理群等）时，扩展这个接口并在 qq.Client 中实现。
type Bot interface {
	SendGroupMessage(ctx context.Context, groupID int64, message string) error
	SendPrivateMessage(ctx context.Context, userID int64, message string) error
	SendGroupImage(ctx context.Context, groupID int64, file string) error
	SendPrivateImage(ctx context.Context, userID int64, file string) error
}

// EventType 表示 QQ 事件类型。
type EventType string

const (
	EventGroupMessage   EventType = "group_message"
	EventPrivateMessage EventType = "private_message"
	EventGroupNotice    EventType = "group_notice"
	EventGroupRecall    EventType = "group_recall"
	EventMeta           EventType = "meta_event"
)

// Stage 表示插件在事件处理流水线中所处的阶段。
type Stage int

const (
	// StagePreprocessor 在响应器之前运行，可记录、过滤、增强或决定是否放行事件。
	StagePreprocessor Stage = iota
	// StageResponder 是真正可能产生回复的插件。
	StageResponder
	// StagePostprocessor 在响应器之后运行，可对回复进行后处理。
	StagePostprocessor
)

func (s Stage) String() string {
	switch s {
	case StagePreprocessor:
		return "preprocessor"
	case StageResponder:
		return "responder"
	case StagePostprocessor:
		return "postprocessor"
	default:
		return "unknown"
	}
}

// Image 是消息中的图片信息。
type Image struct {
	URL  string
	File string
}

// Event 是经过归一化后交给插件的事件对象。
type Event struct {
	Type             EventType
	GroupID          int64
	UserID           int64
	MessageID        int64
	RawMessage       string
	SenderName       string
	AtBot            bool
	Images           []Image
	ReplyToMessageID int64
	ReplyToUserID    int64
	OperatorID       int64
	ImageContext     string
	Raw              []byte
}

// Context 是插件流水线上下文，用于在插件之间传递中间产物。
// Event 保持向后兼容；Data 用于存放下一个阶段需要读取的临时数据。
// Data 使用读写锁保护，可在插件 goroutine 中安全读写。
type Context struct {
	mu    sync.RWMutex
	Event *Event
	Data  map[string]any
}

// NewContext 创建带事件和空 Data 的流水线上下文。
func NewContext(e *Event) *Context {
	if e == nil {
		e = &Event{}
	}
	return &Context{Event: e, Data: make(map[string]any)}
}

// Set 写入一个中间产物。
func (c *Context) Set(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Data == nil {
		c.Data = make(map[string]any)
	}
	c.Data[key] = v
}

// Get 读取一个中间产物，不存在时返回 nil。
func (c *Context) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Data == nil {
		return nil
	}
	return c.Data[key]
}

// Has 判断某个中间产物是否存在。
func (c *Context) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Data == nil {
		return false
	}
	_, ok := c.Data[key]
	return ok
}

// Plugin 是机器人插件的最底层抽象。
//
// 约定：
//   - Handle 返回 true 表示该事件已被处理，Manager 将停止继续分发；
//   - 返回 false 表示插件只是观察/未处理，后续插件仍可处理。
type Plugin interface {
	// Name 返回插件唯一名称。
	Name() string
	// Version 返回插件版本。
	Version() string
	// Description 返回插件说明。
	Description() string
	// Priority 返回同一阶段内的优先级，数值越大越先执行。
	Priority() int
	// Stage 返回插件所属的流水线阶段。
	Stage() Stage
	// Init 在插件管理器启动时调用，用于初始化资源。
	Init(ctx context.Context, bot Bot) error
	// Handle 处理一个归一化事件。
	Handle(ctx context.Context, bot Bot, e *Event) bool
	// Stop 在程序退出时调用，用于释放资源。
	Stop(ctx context.Context) error
}

// ContextualPlugin 是可选接口：实现后 Manager 会优先调用 HandleContext，
// 使插件可以读写流水线中间产物 Context.Data。
type ContextualPlugin interface {
	Plugin
	HandleContext(ctx context.Context, bot Bot, pc *Context) bool
}

// BasePlugin 提供默认实现，插件可内嵌它以减少样板代码。
type BasePlugin struct{}

func (BasePlugin) Version() string                          { return "0.1.0" }
func (BasePlugin) Description() string                      { return "" }
func (BasePlugin) Priority() int                            { return 0 }
func (BasePlugin) Stage() Stage                             { return StageResponder }
func (BasePlugin) Init(context.Context, Bot) error          { return nil }
func (BasePlugin) Handle(context.Context, Bot, *Event) bool { return false }
func (BasePlugin) Stop(context.Context) error               { return nil }
