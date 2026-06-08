package httpapi

import (
	"context"
	"sort"
	"strings"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// topologyEdgesCache holds the most recent /api/v1/topology/edges response
// for a single cache key. The key is computed from the normalised query and
// the server-side relevance cut-off, so two requests that differ only in
// unset fields share a cache slot.
type topologyEdgesCache struct {
	key       string
	expiresAt time.Time
	payload   topologyEdgesPayload
}

type topologyEdgesPayload struct {
	Items     []domain.TopologyEdge `json:"items"`
	Truncated bool                  `json:"truncated"`
}

// topologyCacheKey builds a stable cache key from the parsed query + the
// server-side relevance cut-off. nil/empty SourceKinds collapse to the empty
// list so equivalent queries hit the same slot.
func topologyCacheKey(query repo.TopologyEdgeQuery, updatedSince time.Time) string {
	parts := []string{
		"node=" + strings.TrimSpace(query.NodeID),
		"channel=" + strings.ToLower(strings.TrimSpace(query.Channel)),
		"updated=" + updatedSince.UTC().Format(time.RFC3339Nano),
	}
	kinds := make([]string, len(query.SourceKinds))
	for i, k := range query.SourceKinds {
		kinds[i] = string(k)
	}
	sort.Strings(kinds)
	parts = append(parts, "kinds="+strings.Join(kinds, ","))

	return strings.Join(parts, "|")
}

func (s *Server) loadTopologyEdges(ctx context.Context, query repo.TopologyEdgeQuery, now time.Time) (topologyEdgesPayload, error) {
	ttl := s.cfg.Web.Map.TopologyCacheTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	cacheKey := topologyCacheKey(query, query.UpdatedSince)

	s.topologyMu.Lock()
	if cached, ok := s.cachedTopologyEdges(cacheKey, now); ok {
		s.topologyMu.Unlock()

		return cached, nil
	}
	s.topologyMu.Unlock()

	maxEdges := s.cfg.Web.Map.TopologyMaxEdges
	if maxEdges <= 0 {
		maxEdges = 2000
	}
	if query.Limit <= 0 {
		query.Limit = maxEdges
	}

	items, err := s.store.ListTopologyEdges(ctx, query)
	if err != nil {
		return topologyEdgesPayload{}, err
	}
	truncated := len(items) >= query.Limit
	payload := topologyEdgesPayload{Items: items, Truncated: truncated}

	s.topologyMu.Lock()
	defer s.topologyMu.Unlock()

	// Re-check after taking the write lock: a concurrent request may have
	// populated the slot for the same key while we were querying the store.
	if existing, ok := s.cachedTopologyEdges(cacheKey, now); ok {
		return existing, nil
	}
	s.topologyCache = topologyEdgesCache{
		key:       cacheKey,
		expiresAt: now.Add(ttl),
		payload:   topologyEdgesPayload{Items: cloneTopologyEdges(payload.Items), Truncated: payload.Truncated},
	}

	return payload, nil
}

// cachedTopologyEdges returns a defensive copy of the cached payload when the
// stored key matches and the entry is still fresh. Caller must hold s.topologyMu.
func (s *Server) cachedTopologyEdges(requestKey string, now time.Time) (topologyEdgesPayload, bool) {
	if s.topologyCache.key != requestKey {
		return topologyEdgesPayload{}, false
	}
	if !now.Before(s.topologyCache.expiresAt) {
		return topologyEdgesPayload{}, false
	}

	return topologyEdgesPayload{Items: cloneTopologyEdges(s.topologyCache.payload.Items), Truncated: s.topologyCache.payload.Truncated}, true
}

func cloneTopologyEdges(in []domain.TopologyEdge) []domain.TopologyEdge {
	if in == nil {
		return nil
	}
	out := make([]domain.TopologyEdge, len(in))
	copy(out, in)

	return out
}
