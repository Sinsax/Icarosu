package store

import (
	"fmt"
	"time"
)

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
