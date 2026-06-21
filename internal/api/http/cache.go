package httpapi

import (
	"sync"
	"time"
)

// ttlCache is a small, thread-safe, time-bounded cache for HTTP response
// payloads. The activity period cache and firmware snapshot/history caches
// both use this shape; responses are recomputed lazily on the first read
// after expiry and replaced with the fresh value.
//
// ttlCache is not generic-by-design: storing `any` would force type
// assertions at every call site. Each owner keeps its own typed wrapper.
type ttlCache struct {
	mu sync.Mutex
	// key -> entry. The empty TTL means "always recompute".
	entries map[string]ttlEntry
	now     func() time.Time
}

type ttlEntry struct {
	value     any
	expiresAt time.Time
}

// newTTLCache constructs a ttlCache. now is optional; it defaults to
// time.Now. Tests inject a deterministic clock.
func newTTLCache(now func() time.Time) *ttlCache {
	if now == nil {
		now = time.Now
	}

	return &ttlCache{
		entries: make(map[string]ttlEntry),
		now:     now,
	}
}

// Get returns the cached value for key and reports whether it is still
// fresh. A zero TTL disables caching and Get always returns ok=false.
func (c *ttlCache) Get(key string, ttl time.Duration) (any, bool) {
	if ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(entry.expiresAt) {
		// Drop stale entry eagerly so the next writer replaces it.
		delete(c.entries, key)

		return nil, false
	}

	return entry.value, true
}

// Set stores value for key with the given TTL. The expiry is computed as
// now+ttl, so a zero TTL is silently ignored (matches Get's semantics).
func (c *ttlCache) Set(key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = ttlEntry{
		value:     value,
		expiresAt: c.now().Add(ttl),
	}
}

// Invalidate clears all cached entries. Called when an upstream write
// (e.g. the weekly firmware snapshot) renders every cached response
// stale.
func (c *ttlCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]ttlEntry)
}
