package session_test

import (
	"testing"
	"time"

	"icarosu/internal/session"
)

func TestSessionKeysAreNamespaced(t *testing.T) {
	m := session.New(time.Hour)
	group := m.Get("group:123")
	user := m.Get("user:123")
	if group == user {
		t.Fatal("group and private session must not share the same lock")
	}
	if group.ID != "group:123" || user.ID != "user:123" {
		t.Fatalf("unexpected ids: %q %q", group.ID, user.ID)
	}
}

func TestAcquirePreventsCleanupTOCTOU(t *testing.T) {
	m := session.New(time.Millisecond)
	s := m.Acquire("group:1")
	time.Sleep(5 * time.Millisecond)
	m.Cleanup()
	if got := m.Get("group:1"); got != s {
		t.Fatal("Acquire must pin the session until Release")
	}
	s.Release()
	time.Sleep(5 * time.Millisecond)
	m.Cleanup()
	if got := m.Get("group:1"); got == s {
		t.Fatal("released session should be evictable after TTL")
	}
}

func TestCleanupSkipsInUseSession(t *testing.T) {
	m := session.New(time.Millisecond)
	s := m.Get("group:1")
	s.Lock()

	time.Sleep(5 * time.Millisecond)
	m.Cleanup()
	if got := m.Get("group:1"); got != s {
		t.Fatal("in-use session should not be cleaned up")
	}

	s.Unlock()
	time.Sleep(5 * time.Millisecond)
	m.Cleanup()
	if got := m.Get("group:1"); got == s {
		t.Fatal("idle session should be cleaned up")
	}
}
