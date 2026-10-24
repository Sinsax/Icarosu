package style

import (
	"testing"
)

func TestAddForGroupIsolation(t *testing.T) {
	lib := NewLibrary()
	lib.AddForGroup(101, []string{"funny"}, "群A专属梗")
	lib.AddForGroup(102, []string{"funny"}, "群B专属梗")

	contains := func(exprs []string, target string) bool {
		for _, e := range exprs {
			if e == target {
				return true
			}
		}
		return false
	}

	if !contains(lib.SampleByTagsForGroup(101, []string{"funny"}, 50), "群A专属梗") {
		t.Fatal("group A should contain its own learned expression")
	}
	if contains(lib.SampleByTagsForGroup(102, []string{"funny"}, 50), "群A专属梗") {
		t.Fatal("group B should not contain group A expression")
	}
	if !contains(lib.SampleByTagsForGroup(102, []string{"funny"}, 50), "群B专属梗") {
		t.Fatal("group B should contain its own learned expression")
	}
}

func TestAddForGroupsSharesExpression(t *testing.T) {
	lib := NewLibrary()
	lib.AddForGroups([]int64{201, 202}, []string{"food"}, "共享干饭梗")

	for _, gid := range []int64{201, 202} {
		found := false
		for _, s := range lib.SampleByTagsForGroup(gid, []string{"food"}, 50) {
			if s == "共享干饭梗" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("group %d should contain shared expression", gid)
		}
	}
}
