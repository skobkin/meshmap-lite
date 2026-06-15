package updatecheck

import "sync"

// Cache holds the latest successful UpdateSnapshot per source. It is owned
// by the Manager; HTTP handlers and other readers only call Get. The cache
// is in-memory only — there is no restart durability, by design.
type Cache struct {
	mu    sync.RWMutex
	items map[string]UpdateSnapshot
	known map[string]bool
}

// NewCache returns an empty cache ready for use.
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]UpdateSnapshot),
		known: make(map[string]bool),
	}
}

// Get returns the cached snapshot for the named source. The bool indicates
// whether a snapshot exists — distinct from an empty UpdateSnapshot.
func (c *Cache) Get(name string) (UpdateSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snap, ok := c.items[name]
	if !ok {
		return UpdateSnapshot{}, false
	}
	known := c.known[name]
	if !known {
		return UpdateSnapshot{}, false
	}

	return snap, true
}

// Set overwrites the cached snapshot for the named source. The Manager is
// the only caller.
func (c *Cache) Set(name string, snap UpdateSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[name] = snap
	c.known[name] = true
}
