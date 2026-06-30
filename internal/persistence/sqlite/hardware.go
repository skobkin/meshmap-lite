package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"meshmap-lite/internal/repo"
)

// HardwareHistoryRow is one pivot row from the SQL aggregation before padding.
// Counts[i] corresponds to topIDs[i] (in order); Other is the catch-all bucket
// for models outside the top-N.
type HardwareHistoryRow struct {
	WeekStart string // YYYY-MM-DD
	Counts    []int
	Other     int
}

// UpsertHardwareModel finds-or-creates a row in hardware_models and returns its
// id. last_seen_at is bumped on conflict; first_seen_at and id are preserved.
func (s *Store) UpsertHardwareModel(ctx context.Context, model string, observedAt time.Time) (int64, error) {
	if model == "" {
		return 0, fmt.Errorf("empty hardware model")
	}
	obs := observedAt.UTC().Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO hardware_models (model_string, first_seen_at, last_seen_at, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(model_string) DO UPDATE SET last_seen_at = MAX(last_seen_at, excluded.last_seen_at)
RETURNING id
`, model, obs, obs, now).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// UpdateNodeHardwareModelID sets the node's current hardware_model_id and bumps
// updated_at. The current model is the denormalized cache used for the snapshot
// endpoint; the source of truth for trends is node_hardware_history (written by
// the scheduled job).
func (s *Store) UpdateNodeHardwareModelID(ctx context.Context, nodeID string, modelID int64, observedAt time.Time) error {
	if nodeID == "" || modelID == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET hardware_model_id = ?, updated_at = ? WHERE node_id = ?`,
		modelID, observedAt.UTC().Format(time.RFC3339Nano), nodeID)

	return err
}

// RecordHardwareHistoryWeek performs the weekly snapshot for one week.
//
// Uses INSERT OR IGNORE so re-running for the same week is a no-op
// (first-writer-wins; preserves the Monday state). If a node's
// hardware_model_id changed mid-week between two runs of the same week, the
// second run does NOT overwrite the row — the Monday state is canonical,
// mid-week changes are captured by the next Monday's snapshot.
//
// maxAge is the staleness window applied to nodes.last_seen_any_event_at:
// nodes that haven't been seen on any event in this duration are excluded
// from the snapshot. Unlike firmware (which gates last_map_report_at because
// firmware only arrives via MapReport), hardware arrives with NodeInfo, so
// the broadest liveness column covers nearly every known node. This is a
// write-time gate — once a row is in node_hardware_history, it stays; the
// history read path does not re-filter at query time.
//
// Returns the number of rows newly inserted.
func (s *Store) RecordHardwareHistoryWeek(ctx context.Context, weekStart time.Time, observedAt time.Time, maxAge time.Duration) (int64, error) {
	weekStartText := weekStart.UTC().Format("2006-01-02")
	obsText := observedAt.UTC().Format(time.RFC3339Nano)
	cutoff := observedAt.UTC().Add(-maxAge).Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO node_hardware_history (node_id, hardware_model_id, week_start, observed_at)
SELECT n.node_id, n.hardware_model_id, ?, ?
FROM nodes n
WHERE n.hardware_model_id IS NOT NULL
  AND n.last_seen_any_event_at IS NOT NULL
  AND n.last_seen_any_event_at >= ?
`, weekStartText, obsText, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()

	return n, nil
}

// LastHardwareHistoryWeek returns the max week_start in node_hardware_history.
// Returns the zero time.Time if the table is empty. Used by the scheduled job
// to detect missed weeks on startup.
func (s *Store) LastHardwareHistoryWeek(ctx context.Context) (time.Time, error) {
	var out sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT MAX(week_start) FROM node_hardware_history`).Scan(&out)
	if err != nil {
		return time.Time{}, err
	}
	if !out.Valid || out.String == "" {
		return time.Time{}, nil
	}
	t, parseErr := time.Parse("2006-01-02", out.String)
	if parseErr != nil {
		// Malformed week_start (shouldn't happen — the weekly snapshot job is
		// the only writer and uses startOfWeek's format). Return the zero time
		// and the parse error so the scheduled job treats this the same as "no
		// valid history yet" via the err return.
		return time.Time{}, parseErr
	}

	return t, nil
}

// HardwareModelSnapshot returns the current fleet distribution (today's counts
// per hardware model). Driven by the nodes JOIN hardware_models denormalization.
//
// maxAge is the staleness window applied to nodes.last_seen_any_event_at: nodes
// that haven't been seen on any event in this duration are excluded. See
// web.stats.hardware.max_age.
func (s *Store) HardwareModelSnapshot(ctx context.Context, maxAge time.Duration) ([]repo.HardwareModelCount, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `
SELECT hm.model_string, COUNT(*) AS c, MAX(n.last_seen_any_event_at) AS last_seen
FROM nodes n
JOIN hardware_models hm ON hm.id = n.hardware_model_id
WHERE n.last_seen_any_event_at IS NOT NULL
  AND n.last_seen_any_event_at >= ?
GROUP BY hm.id, hm.model_string
ORDER BY c DESC, hm.model_string ASC
`, cutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]repo.HardwareModelCount, 0)
	for rows.Next() {
		var hm repo.HardwareModelCount
		var lastSeen sql.NullString
		if err := rows.Scan(&hm.Model, &hm.Count, &lastSeen); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			hm.LastSeenAt = mustTime(lastSeen)
		}
		out = append(out, hm)
	}

	return out, rows.Err()
}

// HardwareModelHistory returns the dense per-week pivot for the top-N models
// plus an "(other)" bucket. since is the oldest week_start to include
// (inclusive); totalWeeks is the number of columns to render (missing weeks are
// zero-filled).
func (s *Store) HardwareModelHistory(ctx context.Context, since time.Time, topN int, totalWeeks int) (repo.HardwareHistoryResult, error) {
	if topN <= 0 {
		topN = 15
	}
	if totalWeeks <= 0 {
		totalWeeks = 54
	}
	sinceWeek := startOfWeek(since)
	sinceText := sinceWeek.Format("2006-01-02")

	// Step 1: pick top-N models by row count in window.
	topIDs, err := s.hardwareTopModels(ctx, sinceText, topN)
	if err != nil {
		return repo.HardwareHistoryResult{}, err
	}

	result := repo.HardwareHistoryResult{
		Weeks: totalWeeks,
		TopN:  topN,
	}
	// Resolve the column week starts up front so callers (the HTTP handler,
	// the front-end chart) have a single source of truth for display math.
	// startOfWeek normalizes the caller's `since` to the enclosing Monday, so
	// this slice always aligns with the inner ModelsByWeek axis even when the
	// caller passed a mid-week day.
	result.WeekStarts = make([]time.Time, totalWeeks)
	for i := 0; i < totalWeeks; i++ {
		result.WeekStarts[i] = sinceWeek.AddDate(0, 0, 7*i)
	}

	if len(topIDs) == 0 {
		return result, nil
	}

	// Step 2: pivot per-week counts for top-N + other.
	pivotRows, err := s.hardwarePivotRows(ctx, topIDs, sinceText)
	if err != nil {
		return repo.HardwareHistoryResult{}, err
	}

	// Step 3: resolve model strings for top IDs.
	modelStrings, err := s.modelStringsByID(ctx, topIDs)
	if err != nil {
		return repo.HardwareHistoryResult{}, err
	}

	// Detect whether the "other" bucket has any data.
	hasOther := false
	for _, r := range pivotRows {
		if r.Other > 0 {
			hasOther = true

			break
		}
	}

	result.Models = make([]string, 0, len(modelStrings)+1)
	result.Models = append(result.Models, modelStrings...)
	if hasOther {
		result.Models = append(result.Models, "(other)")
	}

	result.ModelsByWeek = make([][]int, len(result.Models))
	for i := range result.ModelsByWeek {
		result.ModelsByWeek[i] = make([]int, totalWeeks)
	}

	for _, r := range pivotRows {
		ws, parseErr := time.Parse("2006-01-02", r.WeekStart)
		if parseErr != nil {
			continue
		}
		idx := int(ws.Sub(sinceWeek).Hours() / 24 / 7)
		if idx < 0 || idx >= totalWeeks {
			continue
		}
		for i, c := range r.Counts {
			result.ModelsByWeek[i][idx] = c
		}
		if hasOther {
			result.ModelsByWeek[len(result.Models)-1][idx] = r.Other
		}
	}

	return result, nil
}

func (s *Store) hardwareTopModels(ctx context.Context, sinceText string, topN int) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT hardware_model_id
FROM node_hardware_history
WHERE week_start >= ?
GROUP BY hardware_model_id
ORDER BY COUNT(*) DESC
LIMIT ?
`, sinceText, topN)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0, topN)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}

	return out, rows.Err()
}

func (s *Store) hardwarePivotRows(ctx context.Context, topIDs []int64, sinceText string) ([]HardwareHistoryRow, error) {
	pivotExprs := make([]string, len(topIDs))
	for i := range pivotExprs {
		pivotExprs[i] = "SUM(CASE WHEN hardware_model_id = ? THEN 1 ELSE 0 END)"
	}
	pivotSelect := strings.Join(pivotExprs, ", ")
	otherPlaceholders := strings.Repeat(",?", len(topIDs))[1:]

	//nolint:gosec // Safe: pivotSelect and otherPlaceholders are built from
	// strings.Repeat(",?", n)[1:] — only "?" characters, not user input.
	sqlStr := fmt.Sprintf(`
SELECT week_start, %s,
       SUM(CASE WHEN hardware_model_id NOT IN (%s) THEN 1 ELSE 0 END) AS other
FROM node_hardware_history
WHERE week_start >= ?
GROUP BY week_start
ORDER BY week_start
`, pivotSelect, otherPlaceholders)

	args := make([]interface{}, 0, len(topIDs)*2+1)
	// First N args feed the SUM(CASE WHEN hardware_model_id = ?) pivots.
	for _, id := range topIDs {
		args = append(args, id)
	}
	// Next N args feed the NOT IN(...) exclusion list (same IDs).
	for _, id := range topIDs {
		args = append(args, id)
	}
	// Final arg feeds WHERE week_start >= ?.
	args = append(args, sinceText)

	rows, err := s.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]HardwareHistoryRow, 0)
	for rows.Next() {
		var weekStart string
		var other int
		counts := make([]int, len(topIDs))
		dests := make([]interface{}, 0, len(topIDs)+2)
		dests = append(dests, &weekStart)
		for i := range counts {
			dests = append(dests, &counts[i])
		}
		dests = append(dests, &other)
		if err := rows.Scan(dests...); err != nil {
			return nil, err
		}
		out = append(out, HardwareHistoryRow{WeekStart: weekStart, Counts: counts, Other: other})
	}

	return out, rows.Err()
}

func (s *Store) modelStringsByID(ctx context.Context, ids []int64) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat(",?", len(ids))[1:]
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	//nolint:gosec // Safe: placeholders is strings.Repeat(",?", n)[1:] — only "?" characters.
	sqlStr := fmt.Sprintf(`SELECT id, model_string FROM hardware_models WHERE id IN (%s)`, placeholders)
	rows, err := s.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	byID := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var v string
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		byID[id] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		if v, ok := byID[id]; ok {
			out[i] = v
		}
	}

	return out, nil
}
