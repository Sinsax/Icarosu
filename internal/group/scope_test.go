package group

import (
	"reflect"
	"testing"

	"icarosu/internal/config"
)

func TestResolverDefaultIsolation(t *testing.T) {
	r := NewResolver(nil, nil)
	got := r.MemoryGroupIDs(100)
	if !reflect.DeepEqual(got, []int64{100}) {
		t.Fatalf("expected isolated memory group, got %v", got)
	}
	got = r.ExpressionGroupIDs(100)
	if !reflect.DeepEqual(got, []int64{100}) {
		t.Fatalf("expected isolated expression group, got %v", got)
	}
}

func TestResolverMemoryShareGroup(t *testing.T) {
	r := NewResolver([]config.ShareGroup{
		{Name: "share", GroupIDs: []int64{1, 2, 3}},
	}, nil)
	got := r.MemoryGroupIDs(2)
	want := []int64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("unexpected memory group ids: %v", got)
	}
	seen := make(map[int64]bool)
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("missing %d in %v", id, got)
		}
	}
	// 表达方式仍应隔离
	expr := r.ExpressionGroupIDs(2)
	if !reflect.DeepEqual(expr, []int64{2}) {
		t.Fatalf("expression should be isolated, got %v", expr)
	}
}

func TestResolverExpressionShareGroup(t *testing.T) {
	r := NewResolver(nil, []config.ShareGroup{
		{Name: "expr", GroupIDs: []int64{10, 11}},
	})
	got := r.ExpressionGroupIDs(10)
	seen := make(map[int64]bool)
	for _, id := range got {
		seen[id] = true
	}
	if !seen[10] || !seen[11] || len(got) != 2 {
		t.Fatalf("unexpected expression group ids: %v", got)
	}
}
