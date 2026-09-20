package conversation

import (
	"reflect"
	"testing"
)

func TestVisibleOutputDistinctTexts(t *testing.T) {
	texts := DistinctVisibleTexts("hello", "world", "hello", "  world  ", "new")
	expected := []string{"hello", "world", "new"}
	if !reflect.DeepEqual(texts, expected) {
		t.Fatalf("expected %v, got %v", expected, texts)
	}
}

func TestStableDigest(t *testing.T) {
	d1 := StableDigest("hello")
	d2 := StableDigest("hello")
	d3 := StableDigest("world")
	if d1 != d2 {
		t.Fatalf("digests of identical string mismatch")
	}
	if d1 == d3 {
		t.Fatalf("digests of different strings matched")
	}
}
