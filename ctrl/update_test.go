package ctrl

import "testing"

func TestSameStringSet(t *testing.T) {
	if !sameStringSet([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("order should not matter")
	}
	if sameStringSet([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("length should matter")
	}
	if sameStringSet(nil, []string{"a"}) {
		t.Fatal("nil vs value")
	}
	if !sameStringSet(nil, nil) {
		t.Fatal("empty sets should match")
	}
}
