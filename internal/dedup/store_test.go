package dedup

import (
	"testing"
	"time"
)

func TestStoreSeen(t *testing.T) {
	s := New(2, time.Hour)
	now := time.Now()
	if s.Seen("a", now) {
		t.Fatal("first seen should be false")
	}
	if !s.Seen("a", now.Add(time.Minute)) {
		t.Fatal("second seen should be true")
	}
	s.Seen("b", now)
	s.Seen("c", now)
	if s.Seen("a", now.Add(2*time.Minute)) {
		t.Fatal("a should have been evicted")
	}
}
