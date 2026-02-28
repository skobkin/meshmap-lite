package dedup

import (
	"container/list"
	"sync"
	"time"
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
func New(opts Options) *Store {
	opts = opts.normalized()

	return &Store{
		size:  opts.Size,
		ttl:   opts.TTL,
		items: make(map[string]*list.Element, opts.Size),
		order: list.New(),
	}
}

// CheckAndMark reports whether key has already been observed in the active dedup window.
func (s *Store) CheckAndMark(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.items[key]; ok {
		e := el.Value.(entry)
		if s.ttl <= 0 || now.Sub(e.ts) <= s.ttl {
			s.order.MoveToFront(el)
			el.Value = entry{key: key, ts: now}

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
