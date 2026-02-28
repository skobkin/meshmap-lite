package dedup

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStoreCheckAndMark(t *testing.T) {
	t.Parallel()

	s := New(Options{Size: 2, TTL: time.Hour})
	now := time.Now()
	if s.CheckAndMark("a", now) {
		t.Fatal("first seen should be false")
	}
	if !s.CheckAndMark("a", now.Add(time.Minute)) {
		t.Fatal("second seen should be true")
	}
	s.CheckAndMark("b", now)
	s.CheckAndMark("c", now)
	if s.CheckAndMark("a", now.Add(2*time.Minute)) {
		t.Fatal("a should have been evicted")
	}
}

func TestStoreCheckAndMarkTTLExpiry(t *testing.T) {
	t.Parallel()

	s := New(Options{Size: 4, TTL: time.Minute})
	now := time.Now()
	if s.CheckAndMark("a", now) {
		t.Fatal("first seen should be false")
	}
	if s.CheckAndMark("a", now.Add(time.Minute+time.Nanosecond)) {
		t.Fatal("expired entry should be treated as new")
	}
}

func TestStoreCheckAndMarkNonPositiveTTLNeverExpires(t *testing.T) {
	t.Parallel()

	for _, ttl := range []time.Duration{0, -time.Second} {
		s := New(Options{Size: 2, TTL: ttl})
		now := time.Now()
		if s.CheckAndMark("a", now) {
			t.Fatalf("first seen should be false for ttl=%s", ttl)
		}
		if !s.CheckAndMark("a", now.Add(24*time.Hour)) {
			t.Fatalf("duplicate should remain seen for ttl=%s", ttl)
		}
	}
}

func TestStoreNormalizesSizeBelowOne(t *testing.T) {
	t.Parallel()

	s := New(Options{Size: 0, TTL: time.Hour})
	now := time.Now()
	s.CheckAndMark("a", now)
	s.CheckAndMark("b", now.Add(time.Second))
	if s.CheckAndMark("a", now.Add(2*time.Second)) {
		t.Fatal("size normalization should keep only one entry")
	}
}

func TestStoreDuplicateAccessRefreshesRecency(t *testing.T) {
	t.Parallel()

	s := New(Options{Size: 2, TTL: time.Hour})
	now := time.Now()
	s.CheckAndMark("a", now)
	s.CheckAndMark("b", now.Add(time.Second))
	if !s.CheckAndMark("a", now.Add(2*time.Second)) {
		t.Fatal("expected duplicate to be seen")
	}
	s.CheckAndMark("c", now.Add(3*time.Second))
	if !s.CheckAndMark("a", now.Add(4*time.Second)) {
		t.Fatal("a should still be present after recency refresh")
	}
	if s.CheckAndMark("b", now.Add(5*time.Second)) {
		t.Fatal("b should have been evicted after a recency refresh")
	}
}

func TestStoreConcurrentCallers(t *testing.T) {
	t.Parallel()

	s := New(Options{Size: 64, TTL: time.Hour})
	now := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 16; j++ {
				key := fmt.Sprintf("key-%d", j%4)
				_ = s.CheckAndMark(key, now.Add(time.Duration(i+j)*time.Millisecond))
			}
		}()
	}

	wg.Wait()

	for i := 0; i < 4; i++ {
		if !s.CheckAndMark(fmt.Sprintf("key-%d", i), now.Add(30*time.Minute)) {
			t.Fatalf("expected key-%d to remain present after concurrent writes", i)
		}
	}
}
