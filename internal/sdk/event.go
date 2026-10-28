package sdk

import (
	"encoding/json"
	"time"
)

// SourceRef 描述事件来自哪个 adapter/平台。
type SourceRef struct {
	AdapterID AdapterID
	Platform  string
}

// Conversation 是平台中立的会话。
type Conversation struct {
	ID        ConversationID
	AdapterID AdapterID
	Kind      string // group / private / channel / thread
	Title     string
	Metadata  map[string]string
}

// Actor 是平台中立的参与者。
type Actor struct {
	ID       ActorID
	Name     string
	Card     string
	IsSelf   bool
	IsAdmin  bool
	Metadata map[string]string
}

// PartType 是消息片段的类型。
type PartType string

const (
	PartText    PartType = "text"
	PartImage   PartType = "image"
	PartAudio   PartType = "audio"
	PartVideo   PartType = "video"
	PartFile    PartType = "file"
	PartReply   PartType = "reply"
	PartMention PartType = "mention"
)

// Part 是平台中立的消息片段。
type Part struct {
	Type PartType
	Text string
	URL  string
	MIME string
	Data []byte
	Meta map[string]string
}

// Event 是 adapter 归一化后交给内核的事件。
type Event struct {
	ID           EventID
	Kind         string // message.created / notice.recalled / timer.fired ...
	OccurredAt   time.Time
	Source       SourceRef
	Conversation Conversation
	Actor        Actor
	Self         ActorID
	Parts        []Part
	ReplyTo      string
	Mentions     []ActorID
	Meta         map[string]string // adapter 原生 ID/调试信息
	Raw          json.RawMessage   // 仅 adapter/调试用，内核不解析
}
