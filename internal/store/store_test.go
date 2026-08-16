package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"icarosu/internal/config"
	"icarosu/internal/store"
)

func TestStoreCRUD(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".store-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	dsn := filepath.Join(dir, "bot.db")
	s, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.UpsertMember(100, 200, "小明", "群名片"); err != nil {
		t.Fatalf("upsert member: %v", err)
	}
	members, err := s.GetMembers(100)
	if err != nil {
		t.Fatalf("get members: %v", err)
	}
	if len(members) != 1 || members[0].Nickname != "小明" {
		t.Fatalf("unexpected members: %+v", members)
	}

	if err := s.SaveMessage(100, 200, "小明", "大家好", "user"); err != nil {
		t.Fatalf("save message: %v", err)
	}
	if err := s.SaveMessage(100, 0, "", "大家好呀", "assistant"); err != nil {
		t.Fatalf("save assistant message: %v", err)
	}
	msgs, err := s.RecentMessages(100, 10)
	if err != nil {
		t.Fatalf("recent messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if err := s.SaveMemory(100, 200, "小明喜欢看番"); err != nil {
		t.Fatalf("save memory: %v", err)
	}
	memories, err := s.GetMemories(100, 10)
	if err != nil {
		t.Fatalf("get memories: %v", err)
	}
	if len(memories) != 1 || memories[0].Content != "小明喜欢看番" {
		t.Fatalf("unexpected memories: %+v", memories)
	}
}

func TestStoreRelationData(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".store-relation-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	s, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.SaveReply(100, 200, 111, 110); err != nil {
		t.Fatalf("save reply: %v", err)
	}
	if err := s.SaveRecall(100, 111, 200, 300); err != nil {
		t.Fatalf("save recall: %v", err)
	}
	now := time.Now()
	if err := s.RecordActivity(100, 200, now); err != nil {
		t.Fatalf("record activity: %v", err)
	}
	if err := s.RecordInteraction(100, 200, 300); err != nil {
		t.Fatalf("record interaction: %v", err)
	}

	hours, err := s.GetUserActiveHours(100, 200, 5)
	if err != nil || len(hours) != 1 {
		t.Fatalf("active hours: %v %v", hours, err)
	}
	interactions, err := s.GetTopInteractions(100, 10)
	if err != nil || len(interactions) != 1 {
		t.Fatalf("interactions: %v %v", interactions, err)
	}
}

func TestStoreEvents(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".store-event-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	s, err := store.Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	eventID, err := s.SaveEvent(100, "一起看番", "大家约好周末一起看新番", []int64{200, 201})
	if err != nil {
		t.Fatalf("save event: %v", err)
	}
	if eventID <= 0 {
		t.Fatalf("expected event id > 0, got %d", eventID)
	}

	events, err := s.GetEventsByUser(100, 200, 10)
	if err != nil {
		t.Fatalf("get events by user: %v", err)
	}
	if len(events) != 1 || events[0].Title != "一起看番" {
		t.Fatalf("unexpected events: %+v", events)
	}
}
