package store

import (
	"fmt"
	"time"
)

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