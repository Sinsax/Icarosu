package store

import (
	"context"
	"path/filepath"
	"testing"

	"icarosu/internal/config"
)

func TestSQLitePragmasApplyToEveryConnection(t *testing.T) {
	st, err := Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "bot.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		conn, err := st.db.Conn(ctx)
		if err != nil {
			t.Fatalf("conn %d: %v", i, err)
		}
		defer conn.Close()
		var busy, foreign int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatalf("busy_timeout: %v", err)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreign); err != nil {
			t.Fatalf("foreign_keys: %v", err)
		}
		if busy != 5000 || foreign != 1 {
			t.Fatalf("connection %d has busy_timeout=%d foreign_keys=%d", i, busy, foreign)
		}
	}
}
