package dedup

import (
	"container/list"
	"sync"
	"time"

	"meshmap-lite/internal/config"
)

type entry struct {
	key string
	ts  time.Time
}

// Store tracks recently seen keys in a bounded TTL window.
type Store struct {
	mu    sync.Mutex
	size  int
	ttl   time.Duration
	items map[string]*list.Element
	order *list.List
}

// New creates a dedup store with maximum key count and entry TTL.
func New(cfg config.KVConfig) *Store {
	size := cfg.Size
	ttl := cfg.TTL
	if size < 1 {
		size = 1
	}

	return &Store{size: size, ttl: ttl, items: make(map[string]*list.Element, size), order: list.New()}
}

// Seen reports whether key has been observed in the active dedup window.
func (s *Store) Seen(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[key]; ok {
		e := el.Value.(entry)
		if s.ttl <= 0 || now.Sub(e.ts) <= s.ttl {
			s.order.MoveToFront(el)

			return true
		}
		s.order.Remove(el)
		delete(s.items, key)
	}
	el := s.order.PushFront(entry{key: key, ts: now})
	s.items[key] = el
	for s.order.Len() > s.size {
		last := s.order.Back()
		if last == nil {
			break
		}
		old := last.Value.(entry)
		delete(s.items, old.key)
		s.order.Remove(last)
	}

	return false
}
