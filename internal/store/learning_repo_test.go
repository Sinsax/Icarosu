package store

import (
	"os"
	"path/filepath"
	"testing"

	"icarosu/internal/config"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir, err := os.MkdirTemp(".", ".learning-store-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	st, err := Open(config.StoreConfig{Driver: "sqlite", DSN: filepath.Join(dir, "bot.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestLearningRepos(t *testing.T) {
	st := openTestStore(t)

	if err := st.SaveStyleExpression(101, []string{"funny"}, "群A梗"); err != nil {
		t.Fatalf("save style: %v", err)
	}
	exprs, err := st.LoadStyleExpressions()
	if err != nil {
		t.Fatalf("load styles: %v", err)
	}
	if len(exprs) != 1 || exprs[0].GroupID != 101 || exprs[0].Expression != "群A梗" {
		t.Fatalf("unexpected styles: %+v", exprs)
	}

	if err := st.SaveJargon(101, "yyds", "永远的神", 1); err != nil {
		t.Fatalf("save jargon: %v", err)
	}
	jargons, err := st.LoadJargonsByGroup(101, 10)
	if err != nil {
		t.Fatalf("load jargons: %v", err)
	}
	if len(jargons) != 1 || jargons[0].Term != "yyds" {
		t.Fatalf("unexpected jargons: %+v", jargons)
	}

	if err := st.UpsertUserProfile(101, 1, "小明"); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}
	if err := st.UpsertUserProfile(101, 1, "小明"); err != nil {
		t.Fatalf("upsert profile again: %v", err)
	}
	profiles, err := st.LoadUserProfiles(101, 10)
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0].MessageCount != 2 {
		t.Fatalf("unexpected profiles: %+v", profiles)
	}
}