package store

import (
	"fmt"
	"time"
)

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

// CountMembers 返回已记录的成员总数。
func (s *Store) CountMembers() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM members`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}