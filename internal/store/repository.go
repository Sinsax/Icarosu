package store

import "time"

// Repository 接口按领域拆分，供上层业务依赖，避免直接依赖 *Store 具体实现。
// *Store 实现所有这些接口；未来可以替换为其他存储适配器。

type MemberRepository interface {
	UpsertMember(groupID, userID int64, nickname, card string) error
	UpsertMemberInfo(groupID, userID int64, nickname, card string) error
	GetMembers(groupID int64) ([]Member, error)
	CountMembers() (int, error)
}

type MessageRepository interface {
	SaveMessage(groupID, userID int64, senderName, content, role string) error
	RecentMessages(groupID int64, limit int) ([]MessageRecord, error)
	GroupIDs() ([]int64, error)
	CountMessages() (int, error)
}

type MemoryRepository interface {
	SaveMemory(groupID, userID int64, content string) error
	SaveMemoryWithVector(groupID, userID int64, content string, vector []float32) (int64, error)
	SearchMemories(groupID int64, query []float32, limit int) ([]MemoryRecord, error)
	GetMemories(groupID int64, limit int) ([]MemoryRecord, error)
	ListMemories(limit, offset int) ([]MemoryRecord, error)
	SearchMemoriesText(query string, limit int) ([]MemoryRecord, error)
	DeleteMemory(id int64) error
	CountMemories() (int, error)
}

type EventRepository interface {
	SaveEvent(groupID int64, title, content string, participantIDs []int64) (int64, error)
	GetEventsByGroup(groupID int64, limit int) ([]EventRecord, error)
	GetEventsByUser(groupID, userID int64, limit int) ([]EventRecord, error)
}

type ActivityRepository interface {
	RecordActivity(groupID, userID int64, t time.Time) error
	GetUserActiveHours(groupID, userID int64, limit int) ([]HourStat, error)
	GetTopInteractions(groupID int64, limit int) ([]InteractionStat, error)
}

type TopicRepository interface {
	SaveTopicState(groupID int64, topicText, lastUserMsg, lastReply string, stability int, vector []float32, updatedAt time.Time) error
	LoadTopicStates() ([]TopicStateRecord, error)
}

type RecallRepository interface {
	SaveReply(groupID, userID, messageID, replyToID int64) error
	SaveRecall(groupID, messageID, userID, operatorID int64) error
}

// ConversationStore 是 memory 等模块需要的最小存储能力组合。
type ConversationStore interface {
	MemberRepository
	MessageRepository
	MemoryRepository
}

// ChatContextStore 是 chat 插件读取群数据上下文时需要的最小存储能力组合。
type ChatContextStore interface {
	ActivityRepository
	EventRepository
}

// SummaryStore 是 summarizer 插件需要的最小存储能力组合。
type SummaryStore interface {
	MessageRepository
	MemoryRepository
}

// RecorderStore 是 recorder 插件需要的最小存储能力组合。
type RecorderStore interface {
	RecallRepository
	ActivityRepository
	RecordInteraction(groupID, fromID, toID int64) error
	UpsertUserProfile(groupID, userID int64, nickname string) error
}

// AdminStore 是 admin 插件需要的最小存储能力组合。
type AdminStore interface {
	MemberRepository
	MemoryRepository
}

// GroupDataStore 是 WebUI 展示群维度学习数据需要的能力。
type GroupDataStore interface {
	LoadStyleExpressions() ([]StyleExpressionRecord, error)
	LoadJargonsByGroup(groupID int64, limit int) ([]JargonRecord, error)
	LoadUserProfiles(groupID int64, limit int) ([]UserProfileRecord, error)
}

// FullStore 是完整存储能力，供组装层和 WebUI 等需要全量能力的模块使用。
type FullStore interface {
	MemberRepository
	MessageRepository
	MemoryRepository
	EventRepository
	TopicRepository
	RecorderStore
	GroupDataStore
}

var (
	_ MemberRepository   = (*Store)(nil)
	_ MessageRepository  = (*Store)(nil)
	_ MemoryRepository   = (*Store)(nil)
	_ EventRepository    = (*Store)(nil)
	_ ActivityRepository = (*Store)(nil)
	_ TopicRepository    = (*Store)(nil)
	_ RecallRepository   = (*Store)(nil)
	_ ConversationStore  = (*Store)(nil)
	_ ChatContextStore   = (*Store)(nil)
	_ SummaryStore       = (*Store)(nil)
	_ RecorderStore      = (*Store)(nil)
	_ AdminStore         = (*Store)(nil)
	_ FullStore          = (*Store)(nil)
)
