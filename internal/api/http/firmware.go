package httpapi

import (
	"net/http"
	"strconv"
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
	cacheKey := "snapshot"

	if cached, ok := s.firmwareCache.Get(cacheKey, ttl); ok {
		writeJSON(w, http.StatusOK, cached)

		return
	}

	counts, err := s.store.FirmwareVersionSnapshot(r.Context())
	if err != nil {
		s.log.Error("firmware snapshot query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "firmware snapshot unavailable")

		return
	}

	payload := buildFirmwareSnapshotPayload(s.now(), counts)
	s.firmwareCache.Set(cacheKey, payload, ttl)
	writeJSON(w, http.StatusOK, payload)
}

// firmwareHistory serves GET /api/v1/stats/firmware/history. Returns the
// dense per-week pivot for the top-N versions plus an "(other)" bucket,
// padded out to StatsConfig.Software.HistoryWeeks columns.
//
// Query params (all optional):
//
//	weeks=N - render N weeks ending at the current week (default 54)
//	top=N   - top-N versions in the window (default 15)
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
	if v, err := strconv.Atoi(r.URL.Query().Get("weeks")); err == nil && v > 0 {
		historyWeeks = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("top")); err == nil && v > 0 {
		topN = v
	}

	cacheKey := "history:w=" + strconv.Itoa(historyWeeks) + ":t=" + strconv.Itoa(topN)

	if cached, ok := s.firmwareCache.Get(cacheKey, ttl); ok {
		writeJSON(w, http.StatusOK, cached)

		return
	}

	now := s.now().UTC()
	since := now.AddDate(0, 0, -7*historyWeeks)
	result, err := s.store.FirmwareVersionHistory(r.Context(), since, topN, historyWeeks)
	if err != nil {
		s.log.Error("firmware history query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "firmware history unavailable")

		return
	}

	payload := firmwareHistoryPayload{
		GeneratedAt:    now,
		Weeks:          result.Weeks,
		Top:            result.TopN,
		Versions:       result.Versions,
		VersionsByWeek: result.VersionsByWeek,
	}
	s.firmwareCache.Set(cacheKey, payload, ttl)
	writeJSON(w, http.StatusOK, payload)
}

func buildFirmwareSnapshotPayload(generatedAt time.Time, counts []repo.FirmwareVersionCount) firmwareSnapshotPayload {
	out := firmwareSnapshotPayload{
		GeneratedAt: generatedAt,
		Versions:    make([]firmwareVersionPayload, 0, len(counts)),
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
