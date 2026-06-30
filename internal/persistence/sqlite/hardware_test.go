package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// newHardwareTestStore opens an in-memory sqlite store with the full
// V23 migration applied (so hardware_models / node_hardware_history
// exist and nodes.hardware_model_id is present while board_model is gone).
func newHardwareTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return s
}

// seedHardwareNode inserts one node row with a recent last_seen_any_event_at
// (so it passes the staleness filter used by the hardware stats queries) and,
// when model is non-empty, sets the node's hardware_model_id FK. Staleness is
// exercised explicitly by seedHardwareNodeAt / the dedicated _ExcludesStaleNodes
// tests.
func seedHardwareNode(t *testing.T, ctx context.Context, s *Store, nodeID string, model string) {
	t.Helper()
	now := time.Now().UTC()
	seedHardwareNodeAt(t, ctx, s, nodeID, model, &now)
}

// seedHardwareNodeAt is the same as seedHardwareNode but lets the caller pin
// last_seen_any_event_at — the column the hardware stats queries gate on (unlike
// firmware, which gates last_map_report_at). Use nil for the "never seen on any
// event" case: last_seen_any_event_at is NOT NULL on nodes, so there is no real
// NULL row to build; nil is represented by the zero time, which fails the
// `>= cutoff` comparison exactly as a NULL would (the queries' IS NOT NULL guard
// is defensive and never trips in practice, since every ingest path bumps it).
func seedHardwareNodeAt(t *testing.T, ctx context.Context, s *Store, nodeID string, model string, lastSeenAnyEventAt *time.Time) {
	t.Helper()
	if model != "" {
		if _, err := s.UpsertHardwareModel(ctx, model, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("upsert hardware %q: %v", model, err)
		}
	}
	updated := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	seen := updated
	if lastSeenAnyEventAt == nil {
		seen = time.Time{}
	} else {
		seen = *lastSeenAnyEventAt
	}
	node := domain.Node{
		NodeID:             nodeID,
		LongName:           nodeID,
		LastSeenAnyEventAt: seen,
		UpdatedAt:          updated,
	}
	if _, err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("upsert node %q: %v", nodeID, err)
	}
	if model != "" {
		modelID, err := s.UpsertHardwareModel(ctx, model, updated)
		if err != nil {
			t.Fatalf("resolve hardware %q: %v", model, err)
		}
		if err := s.UpdateNodeHardwareModelID(ctx, nodeID, modelID, updated); err != nil {
			t.Fatalf("set hardware_model_id on %q: %v", nodeID, err)
		}
	}
}

func TestHardwareModelSnapshot_OrdersByCountAndExcludesNULL(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	// Two nodes on heltec-v3, one on tbeam, one with no model (NULL FK).
	seedHardwareNode(t, ctx, s, "!alpha", "heltec-v3")
	seedHardwareNode(t, ctx, s, "!bravo", "heltec-v3")
	seedHardwareNode(t, ctx, s, "!charlie", "tbeam")
	seedHardwareNode(t, ctx, s, "!delta", "")

	snap, err := s.HardwareModelSnapshot(ctx, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap) != 2 {
		t.Fatalf("expected 2 models (NULL FK excluded), got %d: %+v", len(snap), snap)
	}
	if snap[0].Model != "heltec-v3" || snap[0].Count != 2 {
		t.Fatalf("expected first row heltec-v3 count 2, got %+v", snap[0])
	}
	if snap[1].Model != "tbeam" || snap[1].Count != 1 {
		t.Fatalf("expected second row tbeam count 1, got %+v", snap[1])
	}
}

func TestHardwareModelHistory_ZeroPadsMissingWeeksAndKeepsOther(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	// Three models, three nodes on heltec-v3, two on tbeam, one on rak4631.
	for _, n := range []string{"!a1", "!a2", "!a3"} {
		seedHardwareNode(t, ctx, s, n, "heltec-v3")
	}
	for _, n := range []string{"!b1", "!b2"} {
		seedHardwareNode(t, ctx, s, n, "tbeam")
	}
	seedHardwareNode(t, ctx, s, "!c1", "rak4631")

	// Two non-contiguous weeks: index 0 and index 3. Indexes 1, 2 are
	// missing and must be padded with zeros. Top-2 + "other" → 3 columns.
	week0 := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) // Monday
	week3 := week0.AddDate(0, 0, 21)
	if _, err := s.RecordHardwareHistoryWeek(ctx, week0, week0, 14*24*time.Hour); err != nil {
		t.Fatalf("record week0: %v", err)
	}
	if _, err := s.RecordHardwareHistoryWeek(ctx, week3, week3, 14*24*time.Hour); err != nil {
		t.Fatalf("record week3: %v", err)
	}

	since := week0
	hist, err := s.HardwareModelHistory(ctx, since, 2, 4)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if hist.Weeks != 4 || hist.TopN != 2 {
		t.Fatalf("unexpected result metadata: %+v", hist)
	}
	if len(hist.Models) != 3 {
		t.Fatalf("expected 3 model labels (top-2 + other), got %d: %+v", len(hist.Models), hist.Models)
	}
	if hist.Models[len(hist.Models)-1] != "(other)" {
		t.Fatalf("expected last label (other), got %q", hist.Models[len(hist.Models)-1])
	}

	if len(hist.ModelsByWeek) != 3 {
		t.Fatalf("expected 3 series, got %d", len(hist.ModelsByWeek))
	}
	for i, series := range hist.ModelsByWeek {
		if len(series) != 4 {
			t.Fatalf("series %d expected 4 weekly columns, got %d", i, len(series))
		}
	}

	// Week 0: 3 on heltec-v3, 2 on tbeam, 1 on rak4631. top-2 picks heltec-v3
	// (3) and tbeam (2); rak4631 (1) goes into "(other)".
	if hist.ModelsByWeek[0][0] != 3 {
		t.Errorf("week 0 / heltec-v3: expected 3, got %d", hist.ModelsByWeek[0][0])
	}
	if hist.ModelsByWeek[1][0] != 2 {
		t.Errorf("week 0 / tbeam: expected 2, got %d", hist.ModelsByWeek[1][0])
	}
	if hist.ModelsByWeek[2][0] != 1 {
		t.Errorf("week 0 / (other): expected 1, got %d", hist.ModelsByWeek[2][0])
	}

	// Sparse weeks 1, 2: all zeros on every series.
	for weekIdx := 1; weekIdx <= 2; weekIdx++ {
		for sIdx, series := range hist.ModelsByWeek {
			if series[weekIdx] != 0 {
				t.Errorf("sparse week %d / series %d: expected 0, got %d", weekIdx, sIdx, series[weekIdx])
			}
		}
	}

	// Week 3: same shape as week 0.
	if hist.ModelsByWeek[0][3] != 3 {
		t.Errorf("week 3 / heltec-v3: expected 3, got %d", hist.ModelsByWeek[0][3])
	}
	if hist.ModelsByWeek[1][3] != 2 {
		t.Errorf("week 3 / tbeam: expected 2, got %d", hist.ModelsByWeek[1][3])
	}
	if hist.ModelsByWeek[2][3] != 1 {
		t.Errorf("week 3 / (other): expected 1, got %d", hist.ModelsByWeek[2][3])
	}
}

func TestRecordHardwareHistoryWeek_FirstWriterWins(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	seedHardwareNode(t, ctx, s, "!alpha", "heltec-v3")
	week := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)

	// First write establishes Monday's state.
	firstObserved := week.Add(2 * time.Hour)
	inserted, err := s.RecordHardwareHistoryWeek(ctx, week, firstObserved, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 row inserted on first snapshot, got %d", inserted)
	}

	// Operator swaps the node's board mid-week. The next snapshot for the same
	// week must NOT overwrite the row.
	seedHardwareNode(t, ctx, s, "!alpha", "tbeam")
	secondObserved := week.AddDate(0, 0, 2) // Wednesday of the same week
	inserted, err = s.RecordHardwareHistoryWeek(ctx, week, secondObserved, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if inserted != 0 {
		t.Fatalf("expected 0 rows inserted on re-snapshot of same week, got %d", inserted)
	}

	// Read back: row must reflect Monday's state.
	var observedAt, modelString string
	if err := s.db.QueryRowContext(ctx, `
SELECT h.observed_at, hm.model_string
FROM node_hardware_history h
JOIN hardware_models hm ON hm.id = h.hardware_model_id
WHERE h.node_id = ? AND h.week_start = ?`,
		"!alpha", "2026-05-04").Scan(&observedAt, &modelString); err != nil {
		t.Fatalf("read history row: %v", err)
	}
	if modelString != "heltec-v3" {
		t.Errorf("expected Monday's heltec-v3 to win, got %q", modelString)
	}
	if observedAt != firstObserved.UTC().Format(time.RFC3339Nano) {
		t.Errorf("expected Monday's observed_at %q to win, got %q",
			firstObserved.UTC().Format(time.RFC3339Nano), observedAt)
	}
}

func TestLastHardwareHistoryWeek_ReturnsZeroWhenEmpty(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	got, err := s.LastHardwareHistoryWeek(ctx)
	if err != nil {
		t.Fatalf("last week: %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("expected zero time on empty history, got %s", got)
	}
}

func TestUpsertHardwareModel_BumpsLastSeenOnConflict(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	earlier := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	id1, err := s.UpsertHardwareModel(ctx, "heltec-v3", earlier)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if id1 == 0 {
		t.Fatalf("expected non-zero model id on first upsert")
	}

	id2, err := s.UpsertHardwareModel(ctx, "heltec-v3", later)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id on conflict, got %d vs %d", id2, id1)
	}

	var lastSeen string
	if err := s.db.QueryRowContext(ctx,
		`SELECT last_seen_at FROM hardware_models WHERE id = ?`, id1).Scan(&lastSeen); err != nil {
		t.Fatalf("read last_seen_at: %v", err)
	}
	if lastSeen != later.UTC().Format(time.RFC3339Nano) {
		t.Errorf("expected last_seen_at %q, got %q", later.UTC().Format(time.RFC3339Nano), lastSeen)
	}
}

// Ensure HardwareHistoryResult is the shape produced by the store — the
// API serializer relies on it being a stable type.
func TestHardwareHistoryResult_TypeStability(t *testing.T) {
	var _ = repo.HardwareHistoryResult{
		Weeks:        1,
		TopN:         1,
		Models:       []string{"x"},
		ModelsByWeek: [][]int{{1}},
		WeekStarts:   []time.Time{time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)},
	}
}

// TestHardwareModelSnapshot_ExcludesStaleNodes pins the read-side staleness
// filter for the snapshot bar chart: a node whose last_seen_any_event_at is
// older than maxAge (or never seen) is excluded from "today's distribution,"
// even though nodes.hardware_model_id is set. Note the gate column is
// last_seen_any_event_at (not last_map_report_at as for firmware) because
// hardware arrives on NodeInfo and covers nearly every node. "!never" is seeded
// with the zero time (see seedHardwareNodeAt) since last_seen_any_event_at is
// NOT NULL.
func TestHardwareModelSnapshot_ExcludesStaleNodes(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	now := time.Now().UTC()
	fresh := now.Add(-1 * time.Hour)       // well within 14d
	stale := now.Add(-30 * 24 * time.Hour) // outside 14d
	maxAge := 14 * 24 * time.Hour

	seedHardwareNodeAt(t, ctx, s, "!fresh", "heltec-v3", &fresh)
	seedHardwareNodeAt(t, ctx, s, "!stale", "heltec-v3", &stale)
	seedHardwareNodeAt(t, ctx, s, "!never", "tbeam", nil) // never seen on any event (zero time)

	snap, err := s.HardwareModelSnapshot(ctx, maxAge)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap) != 1 {
		t.Fatalf("expected only the fresh node to count, got %d rows: %+v", len(snap), snap)
	}
	if snap[0].Model != "heltec-v3" || snap[0].Count != 1 {
		t.Fatalf("expected single heltec-v3 count=1 (only !fresh), got %+v", snap[0])
	}
}

// TestRecordHardwareHistoryWeek_ExcludesStaleNodes pins the write-side
// staleness gate for the history area chart: at the moment the weekly snapshot
// job runs, a node whose last_seen_any_event_at is older than maxAge (or never
// seen) must NOT be inserted into node_hardware_history. The history read path
// itself does no filtering (see plan, section 4), so rows already in the table
// stay — only new writes are gated. The gate column is last_seen_any_event_at
// (not last_map_report_at as for firmware).
func TestRecordHardwareHistoryWeek_ExcludesStaleNodes(t *testing.T) {
	ctx := context.Background()
	s := newHardwareTestStore(t)

	maxAge := 14 * 24 * time.Hour
	observedAt := time.Now().UTC().Add(-30 * 24 * time.Hour)
	fresh := observedAt.Add(-1 * time.Hour)
	stale := observedAt.Add(-30 * 24 * time.Hour)

	seedHardwareNodeAt(t, ctx, s, "!fresh", "heltec-v3", &fresh)
	seedHardwareNodeAt(t, ctx, s, "!stale", "heltec-v3", &stale)
	seedHardwareNodeAt(t, ctx, s, "!never", "tbeam", nil)

	week := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) // Monday
	inserted, err := s.RecordHardwareHistoryWeek(ctx, week, observedAt, maxAge)
	if err != nil {
		t.Fatalf("record week: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected exactly 1 row inserted (only !fresh), got %d", inserted)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT node_id FROM node_hardware_history WHERE week_start = ?`, "2026-06-15")
	if err != nil {
		t.Fatalf("query history rows: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var seen []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen = append(seen, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	if len(seen) != 1 || seen[0] != "!fresh" {
		t.Fatalf("expected only !fresh row, got %v", seen)
	}
}
