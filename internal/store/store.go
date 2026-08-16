package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"icarosu/internal/config"
)

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

// Store 是 SQLite 长期存储。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库。
func Open(cfg config.StoreConfig) (*Store, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = "sqlite"
	}
	dsn := cfg.DSN
	if dsn == "" {
		dsn = "data/bot.db"
	}

	// SQLite 需要确保数据库文件所在目录存在
	if dir := filepath.Dir(dsn); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create store dir %q: %w", dir, err)
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping store: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS members (
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			nickname TEXT NOT NULL DEFAULT '',
			card TEXT NOT NULL DEFAULT '',
			relationship TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			message_count INTEGER NOT NULL DEFAULT 0,
			first_seen_at DATETIME NOT NULL,
			last_seen_at DATETIME NOT NULL,
			PRIMARY KEY (group_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL DEFAULT 0,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			sender_name TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_participants (
			event_id INTEGER NOT NULL,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'participant',
			PRIMARY KEY (event_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS message_replies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			message_id INTEGER NOT NULL,
			reply_to_message_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS recalls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			message_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			operator_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_activity (
			group_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			hour INTEGER NOT NULL,
			weekday INTEGER NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (group_id, user_id, hour, weekday)
		)`,
		`CREATE TABLE IF NOT EXISTS interactions (
			group_id INTEGER NOT NULL,
			from_user_id INTEGER NOT NULL,
			to_user_id INTEGER NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (group_id, from_user_id, to_user_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate store: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertMember 记录或更新群成员信息。
func (s *Store) UpsertMember(groupID, userID int64, nickname, card string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO members (group_id, user_id, nickname, card, relationship, note, message_count, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, '群友', '', 1, ?, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET
			nickname = excluded.nickname,
			card = excluded.card,
			message_count = message_count + 1,
			last_seen_at = excluded.last_seen_at
	`, groupID, userID, nickname, card, now, now)
	if err != nil {
		return fmt.Errorf("upsert member: %w", err)
	}
	return nil
}

// UpsertMemberInfo 同步群成员信息，不增加发言计数。
func (s *Store) UpsertMemberInfo(groupID, userID int64, nickname, card string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO members (group_id, user_id, nickname, card, relationship, note, message_count, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, '群友', '', 0, ?, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET
			nickname = excluded.nickname,
			card = excluded.card,
			last_seen_at = excluded.last_seen_at
	`, groupID, userID, nickname, card, now, now)
	if err != nil {
		return fmt.Errorf("upsert member info: %w", err)
	}
	return nil
}

// GetMembers 返回群成员列表。
func (s *Store) GetMembers(groupID int64) ([]Member, error) {
	rows, err := s.db.Query(`
		SELECT group_id, user_id, nickname, card, relationship, note, message_count, first_seen_at, last_seen_at
		FROM members WHERE group_id = ? ORDER BY message_count DESC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("query members: %w", err)
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.Nickname, &m.Card, &m.Relationship, &m.Note, &m.MessageCount, &m.FirstSeenAt, &m.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// SaveMessage 保存一条消息。
func (s *Store) SaveMessage(groupID, userID int64, senderName, content, role string) error {
	_, err := s.db.Exec(`
		INSERT INTO messages (group_id, user_id, sender_name, content, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, groupID, userID, senderName, content, role, time.Now())
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	return nil
}

// SaveReply 记录一条引用回复关系。
func (s *Store) SaveReply(groupID, userID, messageID, replyToID int64) error {
	_, err := s.db.Exec(`
		INSERT INTO message_replies (group_id, user_id, message_id, reply_to_message_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, groupID, userID, messageID, replyToID, time.Now())
	if err != nil {
		return fmt.Errorf("save reply: %w", err)
	}
	return nil
}

// SaveRecall 记录一条撤回消息。
func (s *Store) SaveRecall(groupID, messageID, userID, operatorID int64) error {
	_, err := s.db.Exec(`
		INSERT INTO recalls (group_id, message_id, user_id, operator_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, groupID, messageID, userID, operatorID, time.Now())
	if err != nil {
		return fmt.Errorf("save recall: %w", err)
	}
	return nil
}

// RecordActivity 记录用户活跃时间段。
func (s *Store) RecordActivity(groupID, userID int64, t time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO user_activity (group_id, user_id, hour, weekday, message_count)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(group_id, user_id, hour, weekday) DO UPDATE SET
			message_count = message_count + 1
	`, groupID, userID, t.Hour(), int(t.Weekday()))
	if err != nil {
		return fmt.Errorf("record activity: %w", err)
	}
	return nil
}

// RecordInteraction 记录一次“谁回复/提到了谁”的互动。
func (s *Store) RecordInteraction(groupID, fromID, toID int64) error {
	if fromID == 0 || toID == 0 || fromID == toID {
		return nil
	}
	_, err := s.db.Exec(`
		INSERT INTO interactions (group_id, from_user_id, to_user_id, count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(group_id, from_user_id, to_user_id) DO UPDATE SET
			count = count + 1
	`, groupID, fromID, toID)
	if err != nil {
		return fmt.Errorf("record interaction: %w", err)
	}
	return nil
}

// GetUserActiveHours 返回用户最活跃的时间段。
func (s *Store) GetUserActiveHours(groupID, userID int64, limit int) ([]HourStat, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(`
		SELECT hour, weekday, message_count
		FROM user_activity
		WHERE group_id = ? AND user_id = ?
		ORDER BY message_count DESC LIMIT ?
	`, groupID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query user active hours: %w", err)
	}
	defer rows.Close()

	var stats []HourStat
	for rows.Next() {
		var h HourStat
		if err := rows.Scan(&h.Hour, &h.Weekday, &h.Count); err != nil {
			return nil, fmt.Errorf("scan hour stat: %w", err)
		}
		stats = append(stats, h)
	}
	return stats, rows.Err()
}

// GetTopInteractions 返回群内互动最多的关系对。
func (s *Store) GetTopInteractions(groupID int64, limit int) ([]InteractionStat, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT from_user_id, to_user_id, count
		FROM interactions
		WHERE group_id = ?
		ORDER BY count DESC LIMIT ?
	`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("query interactions: %w", err)
	}
	defer rows.Close()

	var stats []InteractionStat
	for rows.Next() {
		var it InteractionStat
		if err := rows.Scan(&it.FromUserID, &it.ToUserID, &it.Count); err != nil {
			return nil, fmt.Errorf("scan interaction: %w", err)
		}
		stats = append(stats, it)
	}
	return stats, rows.Err()
}

// RecentMessages 返回某个群最近的群聊记录。
func (s *Store) RecentMessages(groupID int64, limit int) ([]MessageRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, group_id, user_id, sender_name, content, role, created_at
		FROM messages WHERE group_id = ? ORDER BY id DESC LIMIT ?
	`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent messages: %w", err)
	}
	defer rows.Close()

	var records []MessageRecord
	for rows.Next() {
		var r MessageRecord
		if err := rows.Scan(&r.ID, &r.GroupID, &r.UserID, &r.SenderName, &r.Content, &r.Role, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 倒序转正序
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	return records, nil
}

// GroupIDs 返回数据库中出现过的所有群号。
func (s *Store) GroupIDs() ([]int64, error) {
	rows, err := s.db.Query(`SELECT DISTINCT group_id FROM messages ORDER BY group_id`)
	if err != nil {
		return nil, fmt.Errorf("query group ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan group id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SaveEvent 保存一个事件，并关联参与人。
func (s *Store) SaveEvent(groupID int64, title, content string, participantIDs []int64) (int64, error) {
	now := time.Now()
	res, err := s.db.Exec(`
		INSERT INTO events (group_id, title, content, created_at)
		VALUES (?, ?, ?, ?)
	`, groupID, title, content, now)
	if err != nil {
		return 0, fmt.Errorf("save event: %w", err)
	}
	eventID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("event id: %w", err)
	}
	for _, uid := range participantIDs {
		if _, err := s.db.Exec(`
			INSERT INTO event_participants (event_id, group_id, user_id, role)
			VALUES (?, ?, ?, 'participant')
		`, eventID, groupID, uid); err != nil {
			return 0, fmt.Errorf("save event participant: %w", err)
		}
	}
	return eventID, nil
}

// GetEventsByGroup 返回某个群的事件。
func (s *Store) GetEventsByGroup(groupID int64, limit int) ([]EventRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, group_id, title, content, created_at
		FROM events WHERE group_id = ? ORDER BY id DESC LIMIT ?
	`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("query events by group: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		if err := rows.Scan(&e.ID, &e.GroupID, &e.Title, &e.Content, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetEventsByUser 返回某个用户参与过的事件（关系图查询）。
func (s *Store) GetEventsByUser(groupID, userID int64, limit int) ([]EventRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT e.id, e.group_id, e.title, e.content, e.created_at
		FROM events e
		JOIN event_participants p ON p.event_id = e.id
		WHERE e.group_id = ? AND p.user_id = ?
		ORDER BY e.id DESC LIMIT ?
	`, groupID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query events by user: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		if err := rows.Scan(&e.ID, &e.GroupID, &e.Title, &e.Content, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// SaveMemory 保存一条长期记忆。
func (s *Store) SaveMemory(groupID, userID int64, content string) error {
	_, err := s.db.Exec(`
		INSERT INTO memories (group_id, user_id, content, created_at)
		VALUES (?, ?, ?, ?)
	`, groupID, userID, content, time.Now())
	if err != nil {
		return fmt.Errorf("save memory: %w", err)
	}
	return nil
}

// GetMemories 返回某个群的长期记忆。
func (s *Store) GetMemories(groupID int64, limit int) ([]MemoryRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, group_id, user_id, content, created_at
		FROM memories WHERE group_id = ? ORDER BY id DESC LIMIT ?
	`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var memories []MemoryRecord
	for rows.Next() {
		var m MemoryRecord
		if err := rows.Scan(&m.ID, &m.GroupID, &m.UserID, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}
