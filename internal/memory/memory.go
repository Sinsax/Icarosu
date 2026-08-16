package memory

import (
	"sync"
	"time"

	"icarosu/internal/plugin"
	"icarosu/internal/store"
)

// Entry 是短期上下文里的一条消息。
type Entry struct {
	Role       string
	SenderName string
	UserID     int64
	Content    string
	Time       time.Time
}

// Memory 提供短期上下文和长期记忆读写。
type Memory struct {
	mu     sync.Mutex
	short  map[int64][]Entry
	size   int
	maxAge time.Duration
	store  *store.Store
}

func New(size int, st *store.Store) *Memory {
	if size <= 0 {
		size = 20
	}
	return &Memory{
		short: make(map[int64][]Entry),
		size:  size,
		store: st,
	}
}

// SetContextMaxAge 设置上下文消息的最大有效时间。
// 超过该时间的旧消息不会被作为上下文使用，避免冷场很久后突然被唤醒时接话尴尬。
func (m *Memory) SetContextMaxAge(minutes int) {
	if minutes <= 0 {
		m.maxAge = 0
		return
	}
	m.maxAge = time.Duration(minutes) * time.Minute
}

// RecordIncoming 记录一条收到的消息，并更新成员/长期存储。
func (m *Memory) RecordIncoming(e *plugin.Event) {
	content := e.RawMessage
	m.add(chatKey(e), Entry{
		Role:       "user",
		SenderName: e.SenderName,
		UserID:     e.UserID,
		Content:    content,
		Time:       time.Now(),
	})

	if m.store == nil {
		return
	}
	if e.Type == plugin.EventGroupMessage {
		_ = m.store.UpsertMember(e.GroupID, e.UserID, e.SenderName, e.SenderName)
		_ = m.store.SaveMessage(e.GroupID, e.UserID, e.SenderName, content, "user")
	}
}

// PreloadFromStore 启动时从数据库读取最近消息补充短期上下文，不触发任何发送。
func (m *Memory) PreloadFromStore(limit int) {
	if m.store == nil {
		return
	}
	ids, err := m.store.GroupIDs()
	if err != nil {
		return
	}
	for _, gid := range ids {
		records, err := m.store.RecentMessages(gid, limit)
		if err != nil {
			continue
		}
		for _, r := range records {
			if m.maxAge > 0 && time.Since(r.CreatedAt) > m.maxAge {
				continue
			}
			role := "user"
			if r.Role == "assistant" {
				role = "assistant"
			}
			m.add(gid, Entry{
				Role:       role,
				SenderName: r.SenderName,
				UserID:     r.UserID,
				Content:    r.Content,
				Time:       r.CreatedAt,
			})
		}
	}
}

// RecordImageContext 把图片识别结果作为一条上下文消息记录。
func (m *Memory) RecordImageContext(e *plugin.Event, description string) {
	m.add(chatKey(e), Entry{
		Role:       "user",
		SenderName: e.SenderName,
		UserID:     e.UserID,
		Content:    "[图片识别] " + description,
		Time:       time.Now(),
	})
}

// RecordReply 记录机器人的回复。
func (m *Memory) RecordReply(e *plugin.Event, reply string) {
	m.add(chatKey(e), Entry{
		Role:    "assistant",
		Content: reply,
		Time:    time.Now(),
	})

	if m.store != nil && e.Type == plugin.EventGroupMessage {
		_ = m.store.SaveMessage(e.GroupID, 0, "", reply, "assistant")
	}
}

// Recent 返回某个群/私聊的最近上下文。
func (m *Memory) Recent(e *plugin.Event, limit int) []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := chatKey(e)
	entries := m.short[key]

	// 过滤掉超过上下文有效期的旧消息
	if m.maxAge > 0 {
		now := time.Now()
		filtered := make([]Entry, 0, len(entries))
		for _, entry := range entries {
			if entry.Time.IsZero() || now.Sub(entry.Time) <= m.maxAge {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	if limit > len(entries) {
		limit = len(entries)
	}
	out := make([]Entry, limit)
	copy(out, entries[len(entries)-limit:])
	return out
}

// Clear 清空某个群/私聊的短期上下文。
func (m *Memory) Clear(e *plugin.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.short, chatKey(e))
}

// LongTermMemories 返回某个群的长期记忆文本。
func (m *Memory) LongTermMemories(groupID int64, limit int) []string {
	if m.store == nil {
		return nil
	}
	records, err := m.store.GetMemories(groupID, limit)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, r.Content)
	}
	return out
}

func (m *Memory) add(key int64, entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.short[key] = append(m.short[key], entry)
	if len(m.short[key]) > m.size {
		m.short[key] = m.short[key][len(m.short[key])-m.size:]
	}
}

func chatKey(e *plugin.Event) int64 {
	if e.Type == plugin.EventGroupMessage {
		return e.GroupID
	}
	return e.UserID
}
