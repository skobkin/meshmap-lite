package httpapi

import (
	"net/http"
	"time"

	"meshmap-lite/internal/repo"
)

// firmwareSnapshot serves GET /api/v1/stats/firmware. Returns the current
// fleet distribution (one row per firmware version). Cached for the
// StatsConfig.Software.SnapshotCacheTTL (default 1 h) — the fleet version
// mix changes on the order of days, not minutes, so sub-hour freshness
// buys nothing.
func (s *Server) firmwareSnapshot(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Web.Stats.Software
	ttl := cfg.SnapshotCacheTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	// MapReportMaxAge is the staleness window applied to
	// nodes.last_map_report_at: a node that hasn't sent a MapReport in
	// this duration is excluded from "today's distribution." Default 14d
	// (mirrors stats.defaultFirmwareMaxAge so the snapshot endpoint and
	// the weekly job agree; see NewFirmwareSnapshotJob).
	maxAge := cfg.MapReportMaxAge
	if maxAge <= 0 {
		maxAge = 14 * 24 * time.Hour
	}
	cacheKey := "snapshot"

	if cached, ok := s.firmwareCache.Get(cacheKey, ttl); ok {
		writeJSON(w, http.StatusOK, cached)

		return
	}

	counts, err := s.store.FirmwareVersionSnapshot(r.Context(), maxAge)
	if err != nil {
		s.log.Error("firmware snapshot query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "firmware snapshot unavailable")

		return
	}

	payload := buildFirmwareSnapshotPayload(s.now(), counts, ttl)
	s.firmwareCache.Set(cacheKey, payload, ttl)
	writeJSON(w, http.StatusOK, payload)
}

// firmwareHistory serves GET /api/v1/stats/firmware/history. Returns the
// dense per-week pivot for the top-N versions plus an "(other)" bucket,
// padded out to StatsConfig.Software.HistoryWeeks columns. The window
// shape is fully config-driven; the endpoint does not accept query
// parameters so that a malicious client cannot force oversized
// allocations and so the response stays cacheable behind a single key.
//
// Cached for StatsConfig.Software.HistoryCacheTTL (default 24 h) — the
// underlying weekly snapshot writes are idempotent and infrequent, so
// re-querying more often than the writer's cadence is pure waste.
func (s *Server) firmwareHistory(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Web.Stats.Software
	ttl := cfg.HistoryCacheTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	historyWeeks := cfg.HistoryWeeks
	if historyWeeks <= 0 {
		historyWeeks = 54
	}
	topN := cfg.TopVersions
	if topN <= 0 {
		topN = 15
	}

	cacheKey := "history"

	if cached, ok := s.firmwareCache.Get(cacheKey, ttl); ok {
		writeJSON(w, http.StatusOK, cached)

		return
	}

	// The store allocates exactly `historyWeeks` columns starting at
	// startOfWeek(since). To include the current week as the last column
	// instead of dropping it off the end, compute since as the Monday of
	// the current week minus (weeks-1) weeks — that gives exactly
	// `weeks` columns ending at the current week. The previous
	// formulation (since = now - 7*weeks) anchored to a mid-week day,
	// which pushed the current week past the end of the window and
	// pulled in an extra older week.
	now := s.now().UTC()
	currentWeek := mondayOfWeek(now)
	since := currentWeek.AddDate(0, 0, -7*(historyWeeks-1))
	result, err := s.store.FirmwareVersionHistory(r.Context(), since, topN, historyWeeks)
	if err != nil {
		s.log.Error("firmware history query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "firmware history unavailable")

		return
	}

	// Defensive: if the store didn't populate WeekStarts (shouldn't —
	// the SQL store has done so since PR #111), reconstruct it here so
	// the chart label math is correct even with a rolled-back or
	// third-party ReadStore implementation. Matches the store's own
	// startOfWeek math byte-for-byte.
	weekStarts := result.WeekStarts
	if len(weekStarts) == 0 {
		weekStarts = make([]time.Time, result.Weeks)
		for i := 0; i < result.Weeks; i++ {
			weekStarts[i] = since.AddDate(0, 0, 7*i)
		}
	}

	payload := firmwareHistoryPayload{
		GeneratedAt:     now,
		CacheTtlSeconds: int(ttl / time.Second),
		Weeks:           result.Weeks,
		Top:             result.TopN,
		Versions:        result.Versions,
		VersionsByWeek:  result.VersionsByWeek,
		WeekStarts:      weekStarts,
	}
	s.firmwareCache.Set(cacheKey, payload, ttl)
	writeJSON(w, http.StatusOK, payload)
}

// mondayOfWeek returns the Monday 00:00 UTC of the week containing t.
// Duplicates the sqlite store's internal helper because the handler
// package can't import unexported symbols from internal/persistence/sqlite;
// the two implementations are pinned to the same behaviour by
// TestFirmwareHistoryHandler_IncludesCurrentWeek.
func mondayOfWeek(t time.Time) time.Time {
	t = t.UTC()
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// offset is the number of days to subtract to reach Monday.
	offset := (int(t.Weekday()) + 6) % 7

	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.UTC)
}

func buildFirmwareSnapshotPayload(generatedAt time.Time, counts []repo.FirmwareVersionCount, ttl time.Duration) firmwareSnapshotPayload {
	out := firmwareSnapshotPayload{
		GeneratedAt:     generatedAt,
		CacheTtlSeconds: int(ttl / time.Second),
		Versions:        make([]firmwareVersionPayload, 0, len(counts)),
	}
	for _, c := range counts {
		out.TotalNodesWithVersion += c.Count
		out.Versions = append(out.Versions, firmwareVersionPayload{
			Version:    c.Version,
			Count:      c.Count,
			LastSeenAt: c.LastSeenAt,
		})
	}

	return out
}
