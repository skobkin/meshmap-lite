package updatecheck

import (
	"sync"
	"testing"
	"time"
)

func TestCacheGetReturnsFalseForUnknownSource(t *testing.T) {
	c := NewCache()

	if _, ok := c.Get("missing"); ok {
		t.Fatalf("expected Get to report missing entry")
	}
}

func TestCacheSetThenGetRoundtripsSnapshot(t *testing.T) {
	c := NewCache()
	snap := UpdateSnapshot{
		Source:     "meshmap-lite",
		CheckedAt:  time.Now().UTC(),
		SourceHash: "abc",
	}

	c.Set("meshmap-lite", snap)

	got, ok := c.Get("meshmap-lite")
	if !ok {
		t.Fatalf("expected Get to report known entry")
	}
	if got.Source != "meshmap-lite" {
		t.Fatalf("unexpected source: %q", got.Source)
	}
	if got.SourceHash != "abc" {
		t.Fatalf("unexpected source hash: %q", got.SourceHash)
	}
}

func TestCacheSetOverwritesPreviousSnapshot(t *testing.T) {
	c := NewCache()
	c.Set("meshmap-lite", UpdateSnapshot{SourceHash: "v1"})
	c.Set("meshmap-lite", UpdateSnapshot{SourceHash: "v2"})

	got, _ := c.Get("meshmap-lite")
	if got.SourceHash != "v2" {
		t.Fatalf("expected overwrite, got %q", got.SourceHash)
	}
}

func TestCacheIsConcurrencySafe(t *testing.T) {
	c := NewCache()
	const writers = 8
	const iters = 64

	var wg sync.WaitGroup
	wg.Add(writers)
	for range writers {
		go func() {
			defer wg.Done()
			for range iters {
				c.Set("meshmap-lite", UpdateSnapshot{SourceHash: "h"})
				_, _ = c.Get("meshmap-lite")
			}
		}()
	}
	wg.Wait()

	got, ok := c.Get("meshmap-lite")
	if !ok {
		t.Fatalf("expected entry after concurrent writes")
	}
	if got.SourceHash != "h" {
		t.Fatalf("unexpected final hash: %q", got.SourceHash)
	}
}
