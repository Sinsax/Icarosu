package style_test

import (
	"testing"

	"icarosu/internal/style"
)

func TestRandomByTags(t *testing.T) {
	lib := style.NewLibrary()
	got := lib.RandomByTags([]string{"lazy", "night"})
	if got == "" {
		t.Fatal("expected a style expression")
	}
}

func TestApplyAppendsExpression(t *testing.T) {
	lib := style.NewLibrary()
	reply := lib.Apply("还没呢", []string{"food"})
	if reply == "" {
		t.Fatal("expected non-empty reply")
	}
}
