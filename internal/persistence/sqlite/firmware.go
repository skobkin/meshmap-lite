package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"meshmap-lite/internal/repo"
)

// FirmwareHistoryRow is one pivot row from the SQL aggregation before padding.
// Counts[i] corresponds to topIDs[i] (in order); Other is the catch-all bucket
// for versions outside the top-N.
type FirmwareHistoryRow struct {
	WeekStart string // YYYY-MM-DD
	Counts    []int
	Other     int
}

// UpsertFirmwareVersion finds-or-creates a row in firmware_versions and
// returns its id. last_seen_at is bumped on conflict; first_seen_at and id
// are preserved.
func (s *Store) UpsertFirmwareVersion(ctx context.Context, version string, observedAt time.Time) (int64, error) {
	if version == "" {
		return 0, fmt.Errorf("empty firmware version")
	}
	obs := observedAt.UTC().Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO firmware_versions (version_string, first_seen_at, last_seen_at, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(version_string) DO UPDATE SET last_seen_at = MAX(last_seen_at, excluded.last_seen_at)
RETURNING id
`, version, obs, obs, now).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// UpdateNodeFirmwareVersion sets the node's current firmware_version_id
// and bumps updated_at. The current version is the denormalized cache used
// for the snapshot endpoint; the source of truth for trends is
// node_firmware_history (written by the scheduled job).
func (s *Store) UpdateNodeFirmwareVersion(ctx context.Context, nodeID string, versionID int64, observedAt time.Time) error {
	if nodeID == "" || versionID == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET firmware_version_id = ?, updated_at = ? WHERE node_id = ?`,
		versionID, observedAt.UTC().Format(time.RFC3339Nano), nodeID)

	return err
}

// RecordFirmwareHistoryWeek performs the weekly snapshot for one week.
//
// Uses INSERT OR IGNORE so re-running for the same week is a no-op
// (first-writer-wins; preserves the Monday state). If a node's
// firmware_version_id changed mid-week between two runs of the same
// week, the second run does NOT overwrite the row — the Monday state
// is canonical, mid-week changes are captured by the next Monday's
// snapshot.
//
// maxAge is the staleness window applied to nodes.last_map_report_at:
// nodes that haven't sent a MapReport in this duration are excluded
// from the snapshot. This is a write-time gate — once a row is in
// node_firmware_history, it stays; the history read path does not
// re-filter at query time (see internal/api/http/firmware.go).
//
// Returns the number of rows newly inserted.
func (s *Store) RecordFirmwareHistoryWeek(ctx context.Context, weekStart time.Time, observedAt time.Time, maxAge time.Duration) (int64, error) {
	weekStartText := weekStart.UTC().Format("2006-01-02")
	obsText := observedAt.UTC().Format(time.RFC3339Nano)
	cutoff := observedAt.UTC().Add(-maxAge).Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO node_firmware_history (node_id, firmware_version_id, week_start, observed_at)
SELECT n.node_id, n.firmware_version_id, ?, ?
FROM nodes n
WHERE n.firmware_version_id IS NOT NULL
  AND n.last_map_report_at IS NOT NULL
  AND n.last_map_report_at >= ?
`, weekStartText, obsText, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()

	return n, nil
}

// LastFirmwareHistoryWeek returns the max week_start in node_firmware_history.
// Returns the zero time.Time if the table is empty. Used by the scheduled
// job to detect missed weeks on startup.
func (s *Store) LastFirmwareHistoryWeek(ctx context.Context) (time.Time, error) {
	var out sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT MAX(week_start) FROM node_firmware_history`).Scan(&out)
	if err != nil {
		return time.Time{}, err
	}
	if !out.Valid || out.String == "" {
		return time.Time{}, nil
	}
	t, parseErr := time.Parse("2006-01-02", out.String)
	if parseErr != nil {
		// Malformed week_start (shouldn't happen — the weekly snapshot
		// job is the only writer and uses startOfWeek's format). Return
		// the zero time and the parse error so the scheduled job treats
		// this the same as "no valid history yet" via the err return.
		return time.Time{}, parseErr
	}

	return t, nil
}

// FirmwareVersionSnapshot returns the current fleet distribution (today's
// counts per firmware version). Driven by the nodes JOIN firmware_versions
// denormalization; cheap because the data fits in a small working set.
//
// maxAge is the staleness window applied to nodes.last_map_report_at:
// nodes that haven't sent a MapReport in this duration are excluded
// from the snapshot. See web.stats.software.map_report_max_age.
func (s *Store) FirmwareVersionSnapshot(ctx context.Context, maxAge time.Duration) ([]repo.FirmwareVersionCount, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `
SELECT fv.version_string, COUNT(*) AS c, MAX(n.last_seen_any_event_at) AS last_seen
FROM nodes n
JOIN firmware_versions fv ON fv.id = n.firmware_version_id
WHERE n.last_map_report_at IS NOT NULL
  AND n.last_map_report_at >= ?
GROUP BY fv.id, fv.version_string
ORDER BY c DESC, fv.version_string ASC
`, cutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]repo.FirmwareVersionCount, 0)
	for rows.Next() {
		var fv repo.FirmwareVersionCount
		var lastSeen sql.NullString
		if err := rows.Scan(&fv.Version, &fv.Count, &lastSeen); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			fv.LastSeenAt = mustTime(lastSeen)
		}
		out = append(out, fv)
	}

	return out, rows.Err()
}

// FirmwareVersionHistory returns the dense per-week pivot for the top-N
// versions plus an "(other)" bucket. since is the oldest week_start to
// include (inclusive); totalWeeks is the number of columns to render
// (missing weeks are zero-filled).
func (s *Store) FirmwareVersionHistory(ctx context.Context, since time.Time, topN int, totalWeeks int) (repo.FirmwareHistoryResult, error) {
	if topN <= 0 {
		topN = 15
	}
	if totalWeeks <= 0 {
		totalWeeks = 54
	}
	sinceWeek := startOfWeek(since)
	sinceText := sinceWeek.Format("2006-01-02")

	// Step 1: pick top-N versions by row count in window.
	topIDs, err := s.firmwareTopVersions(ctx, sinceText, topN)
	if err != nil {
		return repo.FirmwareHistoryResult{}, err
	}

	result := repo.FirmwareHistoryResult{
		Weeks: totalWeeks,
		TopN:  topN,
	}
	// Resolve the column week starts up front so callers (the HTTP
	// handler, the front-end chart) have a single source of truth for
	// display math. startOfWeek normalizes the caller's `since` to the
	// enclosing Monday, so this slice always aligns with the inner
	// VersionsByWeek axis even when the caller passed a mid-week day.
	result.WeekStarts = make([]time.Time, totalWeeks)
	for i := 0; i < totalWeeks; i++ {
		result.WeekStarts[i] = sinceWeek.AddDate(0, 0, 7*i)
	}

	if len(topIDs) == 0 {
		return result, nil
	}

	// Step 2: pivot per-week counts for top-N + other.
	pivotRows, err := s.firmwarePivotRows(ctx, topIDs, sinceText)
	if err != nil {
		return repo.FirmwareHistoryResult{}, err
	}

	// Step 3: resolve version strings for top IDs.
	versionStrings, err := s.versionStringsByID(ctx, topIDs)
	if err != nil {
		return repo.FirmwareHistoryResult{}, err
	}

	// Detect whether the "other" bucket has any data.
	hasOther := false
	for _, r := range pivotRows {
		if r.Other > 0 {
			hasOther = true

			break
		}
	}

	result.Versions = make([]string, 0, len(versionStrings)+1)
	result.Versions = append(result.Versions, versionStrings...)
	if hasOther {
		result.Versions = append(result.Versions, "(other)")
	}

	result.VersionsByWeek = make([][]int, len(result.Versions))
	for i := range result.VersionsByWeek {
		result.VersionsByWeek[i] = make([]int, totalWeeks)
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
			result.VersionsByWeek[i][idx] = c
		}
		if hasOther {
			result.VersionsByWeek[len(result.Versions)-1][idx] = r.Other
		}
	}

	return result, nil
}

func (s *Store) firmwareTopVersions(ctx context.Context, sinceText string, topN int) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT firmware_version_id
FROM node_firmware_history
WHERE week_start >= ?
GROUP BY firmware_version_id
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

func (s *Store) firmwarePivotRows(ctx context.Context, topIDs []int64, sinceText string) ([]FirmwareHistoryRow, error) {
	pivotExprs := make([]string, len(topIDs))
	for i := range pivotExprs {
		pivotExprs[i] = "SUM(CASE WHEN firmware_version_id = ? THEN 1 ELSE 0 END)"
	}
	pivotSelect := strings.Join(pivotExprs, ", ")
	otherPlaceholders := strings.Repeat(",?", len(topIDs))[1:]

	//nolint:gosec // Safe: pivotSelect and otherPlaceholders are built from
	// strings.Repeat(",?", n)[1:] — only "?" characters, not user input.
	sqlStr := fmt.Sprintf(`
SELECT week_start, %s,
       SUM(CASE WHEN firmware_version_id NOT IN (%s) THEN 1 ELSE 0 END) AS other
FROM node_firmware_history
WHERE week_start >= ?
GROUP BY week_start
ORDER BY week_start
`, pivotSelect, otherPlaceholders)

	args := make([]interface{}, 0, len(topIDs)*2+1)
	// First N args feed the SUM(CASE WHEN firmware_version_id = ?) pivots.
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

	out := make([]FirmwareHistoryRow, 0)
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
		out = append(out, FirmwareHistoryRow{WeekStart: weekStart, Counts: counts, Other: other})
	}

	return out, rows.Err()
}

func (s *Store) versionStringsByID(ctx context.Context, ids []int64) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat(",?", len(ids))[1:]
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	//nolint:gosec // Safe: placeholders is strings.Repeat(",?", n)[1:] — only "?" characters.
	sqlStr := fmt.Sprintf(`SELECT id, version_string FROM firmware_versions WHERE id IN (%s)`, placeholders)
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

// startOfWeek returns the Monday 00:00 UTC of the week containing t.
func startOfWeek(t time.Time) time.Time {
	t = t.UTC()
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// offset is the number of days to subtract to reach Monday.
	offset := (int(t.Weekday()) + 6) % 7

	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.UTC)
}
