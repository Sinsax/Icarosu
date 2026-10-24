package store

import (
	"fmt"
	"time"
)

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
