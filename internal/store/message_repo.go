package store

import (
	"fmt"
	"time"
)

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

// CountMessages 返回消息总数。
func (s *Store) CountMessages() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return n, nil
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