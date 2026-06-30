package httpapi

import (
	"net/http"
	"time"

	"meshmap-lite/internal/repo"
)

// hardwareSnapshot serves GET /api/v1/stats/hardware. Returns the current
// fleet distribution (one row per hardware model). Cached for the
// StatsConfig.Hardware.SnapshotCacheTTL (default 1 h) — the fleet model mix
// changes on the order of weeks, not minutes, so sub-hour freshness buys
// nothing.
func (s *Server) hardwareSnapshot(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Web.Stats.Hardware
	ttl := cfg.SnapshotCacheTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	// MaxAge is the staleness window applied to nodes.last_seen_any_event_at:
	// a node that hasn't been seen on any event in this duration is excluded
	// from "today's distribution." Default 14d (mirrors
	// stats.defaultHardwareMaxAge so the snapshot endpoint and the weekly job
	// agree; see NewHardwareSnapshotJob). Hardware arrives with NodeInfo, so
	// the broadest liveness column covers nearly every known node.
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 14 * 24 * time.Hour
	}
	cacheKey := "snapshot"

	if cached, ok := s.hardwareCache.Get(cacheKey, ttl); ok {
		writeJSON(w, http.StatusOK, cached)

		return
	}

	counts, err := s.store.HardwareModelSnapshot(r.Context(), maxAge)
	if err != nil {
		s.log.Error("hardware snapshot query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "hardware snapshot unavailable")

		return
	}

	payload := buildHardwareSnapshotPayload(s.now(), counts, ttl)
	s.hardwareCache.Set(cacheKey, payload, ttl)
	writeJSON(w, http.StatusOK, payload)
}

// hardwareHistory serves GET /api/v1/stats/hardware/history. Returns the
// dense per-week pivot for the top-N models plus an "(other)" bucket, padded
// out to StatsConfig.Hardware.HistoryWeeks columns. The window shape is fully
// config-driven; the endpoint does not accept query parameters so that a
// malicious client cannot force oversized allocations and so the response
// stays cacheable behind a single key.
//
// Cached for StatsConfig.Hardware.HistoryCacheTTL (default 24 h) — the
// underlying weekly snapshot writes are idempotent and infrequent, so
// re-querying more often than the writer's cadence is pure waste.
func (s *Server) hardwareHistory(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Web.Stats.Hardware
	ttl := cfg.HistoryCacheTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	historyWeeks := cfg.HistoryWeeks
	if historyWeeks <= 0 {
		historyWeeks = 54
	}
	topN := cfg.TopModels
	if topN <= 0 {
		topN = 15
	}

	cacheKey := "history"

	if cached, ok := s.hardwareCache.Get(cacheKey, ttl); ok {
		writeJSON(w, http.StatusOK, cached)

		return
	}

	// The store allocates exactly `historyWeeks` columns starting at
	// startOfWeek(since). To include the current week as the last column
	// instead of dropping it off the end, compute since as the Monday of the
	// current week minus (weeks-1) weeks — that gives exactly `weeks` columns
	// ending at the current week. Matches the firmware history handler's math.
	now := s.now().UTC()
	currentWeek := mondayOfWeek(now)
	since := currentWeek.AddDate(0, 0, -7*(historyWeeks-1))
	result, err := s.store.HardwareModelHistory(r.Context(), since, topN, historyWeeks)
	if err != nil {
		s.log.Error("hardware history query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "hardware history unavailable")

		return
	}

	// Defensive: if the store didn't populate WeekStarts (shouldn't — the SQL
	// store has done so since it mirrors the firmware store), reconstruct it
	// here so the chart label math is correct even with a rolled-back or
	// third-party ReadStore implementation. Matches the store's own
	// startOfWeek math byte-for-byte.
	weekStarts := result.WeekStarts
	if len(weekStarts) == 0 {
		weekStarts = make([]time.Time, result.Weeks)
		for i := 0; i < result.Weeks; i++ {
			weekStarts[i] = since.AddDate(0, 0, 7*i)
		}
	}

	payload := hardwareHistoryPayload{
		GeneratedAt:     now,
		CacheTtlSeconds: int(ttl / time.Second),
		Weeks:           result.Weeks,
		Top:             result.TopN,
		Models:          result.Models,
		ModelsByWeek:    result.ModelsByWeek,
		WeekStarts:      weekStarts,
	}
	s.hardwareCache.Set(cacheKey, payload, ttl)
	writeJSON(w, http.StatusOK, payload)
}

func buildHardwareSnapshotPayload(generatedAt time.Time, counts []repo.HardwareModelCount, ttl time.Duration) hardwareSnapshotPayload {
	out := hardwareSnapshotPayload{
		GeneratedAt:     generatedAt,
		CacheTtlSeconds: int(ttl / time.Second),
		Models:          make([]hardwareModelPayload, 0, len(counts)),
	}
	for _, c := range counts {
		out.TotalNodesWithModel += c.Count
		out.Models = append(out.Models, hardwareModelPayload{
			Model:      c.Model,
			Count:      c.Count,
			LastSeenAt: c.LastSeenAt,
		})
	}

	return out
}
