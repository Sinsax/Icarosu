package store

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// CountMemories 返回长期记忆总数。
func (s *Store) CountMemories() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count memories: %w", err)
	}
	return n, nil
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

// SaveMemoryWithVector 保存一条长期记忆及其向量，返回记忆 ID。
func (s *Store) SaveMemoryWithVector(groupID, userID int64, content string, vector []float32) (int64, error) {
	now := time.Now()
	res, err := s.db.Exec(`
		INSERT INTO memories (group_id, user_id, content, created_at)
		VALUES (?, ?, ?, ?)
	`, groupID, userID, content, now)
	if err != nil {
		return 0, fmt.Errorf("save memory with vector: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("save memory with vector id: %w", err)
	}

	data, err := json.Marshal(vector)
	if err != nil {
		return 0, fmt.Errorf("encode memory vector: %w", err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO memory_embeddings (memory_id, group_id, vector, model, created_at)
		VALUES (?, ?, ?, 'local', ?)
	`, id, groupID, string(data), now); err != nil {
		return 0, fmt.Errorf("save memory embedding: %w", err)
	}
	return id, nil
}

// SearchMemories 在某个群内按向量余弦相似度检索长期记忆。
func (s *Store) SearchMemories(groupID int64, query []float32, limit int) ([]MemoryRecord, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(`
		SELECT m.id, m.group_id, m.user_id, m.content, m.created_at, e.vector
		FROM memories m
		JOIN memory_embeddings e ON e.memory_id = m.id
		WHERE m.group_id = ?
		ORDER BY m.id DESC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("query memory embeddings: %w", err)
	}
	defer rows.Close()

	var scored []MemoryRecord
	for rows.Next() {
		var m MemoryRecord
		var vectorJSON string
		if err := rows.Scan(&m.ID, &m.GroupID, &m.UserID, &m.Content, &m.CreatedAt, &vectorJSON); err != nil {
			return nil, fmt.Errorf("scan memory embedding: %w", err)
		}
		var vec []float32
		if err := json.Unmarshal([]byte(vectorJSON), &vec); err != nil {
			return nil, fmt.Errorf("decode memory vector: %w", err)
		}
		m.Score = cosineSimilarity(query, vec)
		scored = append(scored, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortMemoryByScore(scored)
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// ListMemories 返回所有长期记忆，支持分页。
func (s *Store) ListMemories(limit, offset int) ([]MemoryRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(`
		SELECT id, group_id, user_id, content, created_at
		FROM memories ORDER BY id DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
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

// SearchMemoriesText 按关键词模糊搜索长期记忆。
func (s *Store) SearchMemoriesText(query string, limit int) ([]MemoryRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, group_id, user_id, content, created_at
		FROM memories WHERE content LIKE ? ORDER BY id DESC LIMIT ?
	`, "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
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

// DeleteMemory 删除一条长期记忆及其向量。
func (s *Store) DeleteMemory(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM memory_embeddings WHERE memory_id = ?`, id); err != nil {
		return fmt.Errorf("delete memory embedding: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete memory: %w", err)
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

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func sortMemoryByScore(records []MemoryRecord) {
	for i := 1; i < len(records); i++ {
		for j := i; j > 0 && records[j].Score > records[j-1].Score; j-- {
			records[j], records[j-1] = records[j-1], records[j]
		}
	}
}