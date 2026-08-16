package qq

import "encoding/json"

// Event 是 OneBot v11 的通用事件结构。
// 只解析当前阶段需要的字段，后续可按需扩展。
type Event struct {
	PostType    string          `json:"post_type"`
	MessageType string          `json:"message_type,omitempty"`
	NoticeType  string          `json:"notice_type,omitempty"`
	SelfID      int64           `json:"self_id,omitempty"`
	UserID      int64           `json:"user_id,omitempty"`
	GroupID     int64           `json:"group_id,omitempty"`
	MessageID   int64           `json:"message_id,omitempty"`
	OperatorID  int64           `json:"operator_id,omitempty"`
	RawMessage  string          `json:"raw_message,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	SubType     string          `json:"sub_type,omitempty"`
	Sender      Sender          `json:"sender,omitempty"`
}

// Sender 是消息发送者信息。
type Sender struct {
	UserID   int64  `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
}

// Segment 是 OneBot 消息段。
type Segment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// GroupMember 是群成员信息。
type GroupMember struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card"`
	Role     string `json:"role"`
}
