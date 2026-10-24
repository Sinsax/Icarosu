package store

import "time"

// Member 是群成员记忆。
type Member struct {
	GroupID      int64
	UserID       int64
	Nickname     string
	Card         string
	Relationship string
	Note         string
	MessageCount int
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
}

// MessageRecord 是存储的消息记录。
type MessageRecord struct {
	ID         int64
	GroupID    int64
	UserID     int64
	SenderName string
	Content    string
	Role       string
	CreatedAt  time.Time
}

// MemoryRecord 是长期记忆。
type MemoryRecord struct {
	ID        int64
	GroupID   int64
	UserID    int64
	Content   string
	CreatedAt time.Time
	Score     float64
}

// TopicStateRecord 是持久化的话题状态。
type TopicStateRecord struct {
	GroupID     int64
	Topic       string
	LastUserMsg string
	LastReply   string
	Stability   int
	Vector      []float32
	UpdatedAt   time.Time
}

// EventRecord 是事件记录，用于把人-事件关系图化。
type EventRecord struct {
	ID        int64
	GroupID   int64
	Title     string
	Content   string
	CreatedAt time.Time
}

// HourStat 是用户活跃时间段统计。
type HourStat struct {
	Hour    int
	Weekday int
	Count   int
}

// InteractionStat 是互动关系统计。
type InteractionStat struct {
	FromUserID int64
	ToUserID   int64
	Count      int
}
