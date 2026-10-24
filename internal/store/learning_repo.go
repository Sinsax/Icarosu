package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// StyleExpressionRecord 是持久化的群表达方式。
type StyleExpressionRecord struct {
	ID         int64
	GroupID    int64
	Tags       []string
	Expression string
	CreatedAt  time.Time
}

// JargonRecord 是群内黑话/梗。
type JargonRecord struct {
	ID           int64
	GroupID      int64
	Term         string
	Meaning      string
	SourceUserID int64
	CreatedAt    time.Time
}

// UserProfileRecord 是群内用户画像。
type UserProfileRecord struct {
	GroupID      int64
	UserID       int64
	Nickname     string
	Interests    string
	MessageCount int
	LastSeenAt   time.Time
}

// SaveStyleExpression 保存一条群表达方式。
func (s *Store) SaveStyleExpression(groupID int64, tags []string, expression string) error {
	data, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("encode style tags: %w", err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO style_expressions (group_id, tags, expression, created_at)
		VALUES (?, ?, ?, ?)
	`, groupID, string(data), expression, time.Now()); err != nil {
		return fmt.Errorf("save style expression: %w", err)
	}
	return nil
}

// LoadStyleExpressions 加载全部表达方式。
func (s *Store) LoadStyleExpressions() ([]StyleExpressionRecord, error) {
	rows, err := s.db.Query(`SELECT id, group_id, tags, expression, created_at FROM style_expressions ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query style expressions: %w", err)
	}
	defer rows.Close()

	var records []StyleExpressionRecord
	for rows.Next() {
		var r StyleExpressionRecord
		var tagsJSON string
		if err := rows.Scan(&r.ID, &r.GroupID, &tagsJSON, &r.Expression, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan style expression: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &r.Tags); err != nil {
			return nil, fmt.Errorf("decode style tags: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// SaveJargon 保存一条群内黑话。
func (s *Store) SaveJargon(groupID int64, term, meaning string, sourceUserID int64) error {
	if _, err := s.db.Exec(`
		INSERT INTO jargons (group_id, term, meaning, source_user_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, groupID, term, meaning, sourceUserID, time.Now()); err != nil {
		return fmt.Errorf("save jargon: %w", err)
	}
	return nil
}

// LoadJargonsByGroup 加载某个群的黑话。
func (s *Store) LoadJargonsByGroup(groupID int64, limit int) ([]JargonRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, group_id, term, meaning, source_user_id, created_at
		FROM jargons WHERE group_id = ? ORDER BY id DESC LIMIT ?
	`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("query jargons: %w", err)
	}
	defer rows.Close()

	var records []JargonRecord
	for rows.Next() {
		var r JargonRecord
		if err := rows.Scan(&r.ID, &r.GroupID, &r.Term, &r.Meaning, &r.SourceUserID, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan jargon: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// UpsertUserProfile 更新群用户画像。
func (s *Store) UpsertUserProfile(groupID, userID int64, nickname string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_profiles (group_id, user_id, nickname, interests, message_count, last_seen_at)
		VALUES (?, ?, ?, '', 1, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET
			nickname = excluded.nickname,
			message_count = message_count + 1,
			last_seen_at = excluded.last_seen_at
	`, groupID, userID, nickname, time.Now())
	if err != nil {
		return fmt.Errorf("upsert user profile: %w", err)
	}
	return nil
}

// LoadUserProfiles 加载某个群的用户画像。
func (s *Store) LoadUserProfiles(groupID int64, limit int) ([]UserProfileRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT group_id, user_id, nickname, interests, message_count, last_seen_at
		FROM user_profiles WHERE group_id = ? ORDER BY message_count DESC LIMIT ?
	`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("query user profiles: %w", err)
	}
	defer rows.Close()

	var records []UserProfileRecord
	for rows.Next() {
		var r UserProfileRecord
		if err := rows.Scan(&r.GroupID, &r.UserID, &r.Nickname, &r.Interests, &r.MessageCount, &r.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan user profile: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}