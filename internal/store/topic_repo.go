package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// SaveTopicState 保存某个群的话题状态。
func (s *Store) SaveTopicState(groupID int64, topicText, lastUserMsg, lastReply string, stability int, vector []float32, updatedAt time.Time) error {
	data, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("encode topic vector: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO topic_states (group_id, topic, last_user_msg, last_reply, stability, vector, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id) DO UPDATE SET
			topic = excluded.topic,
			last_user_msg = excluded.last_user_msg,
			last_reply = excluded.last_reply,
			stability = excluded.stability,
			vector = excluded.vector,
			updated_at = excluded.updated_at
	`, groupID, topicText, lastUserMsg, lastReply, stability, string(data), updatedAt)
	if err != nil {
		return fmt.Errorf("save topic state: %w", err)
	}
	return nil
}

// LoadTopicStates 加载所有已保存的话题状态。
func (s *Store) LoadTopicStates() ([]TopicStateRecord, error) {
	rows, err := s.db.Query(`SELECT group_id, topic, last_user_msg, last_reply, stability, vector, updated_at FROM topic_states`)
	if err != nil {
		return nil, fmt.Errorf("query topic states: %w", err)
	}
	defer rows.Close()

	var records []TopicStateRecord
	for rows.Next() {
		var r TopicStateRecord
		var vectorJSON string
		if err := rows.Scan(&r.GroupID, &r.Topic, &r.LastUserMsg, &r.LastReply, &r.Stability, &vectorJSON, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan topic state: %w", err)
		}
		if err := json.Unmarshal([]byte(vectorJSON), &r.Vector); err != nil {
			return nil, fmt.Errorf("decode topic vector: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}