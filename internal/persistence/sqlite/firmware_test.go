package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// newFirmwareTestStore opens an in-memory sqlite store with the full
// V21 migration applied (so firmware_versions / node_firmware_history
// exist).
func newFirmwareTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return s
}

// seedNode inserts one node row at the given times. firmwareVersion is
// the version string used to create a firmware_versions row and set the
// node's firmware_version_id FK.
func seedNode(t *testing.T, ctx context.Context, s *Store, nodeID string, firmwareVersion string) {
	t.Helper()
	if firmwareVersion != "" {
		if _, err := s.UpsertFirmwareVersion(ctx, firmwareVersion, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("upsert firmware %q: %v", firmwareVersion, err)
		}
	}
	updated := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	node := domain.Node{
		NodeID:             nodeID,
		LongName:           nodeID,
		LastSeenAnyEventAt: updated,
		UpdatedAt:          updated,
	}
	if _, err := s.UpsertNode(ctx, node); err != nil {
		t.Fatalf("upsert node %q: %v", nodeID, err)
	}
	if firmwareVersion != "" {
		versionID, err := s.UpsertFirmwareVersion(ctx, firmwareVersion, updated)
		if err != nil {
			t.Fatalf("resolve firmware %q: %v", firmwareVersion, err)
		}
		if err := s.UpdateNodeFirmwareVersion(ctx, nodeID, versionID, updated); err != nil {
			t.Fatalf("set firmware_version_id on %q: %v", nodeID, err)
		}
	}
}

func TestFirmwareVersionSnapshot_OrdersByCountAndExcludesNULL(t *testing.T) {
	ctx := context.Background()
	s := newFirmwareTestStore(t)

	// Two nodes on 2.6.5, one on 2.7.10, one with no version (NULL FK).
	seedNode(t, ctx, s, "!alpha", "2.6.5")
	seedNode(t, ctx, s, "!bravo", "2.6.5")
	seedNode(t, ctx, s, "!charlie", "2.7.10")
	seedNode(t, ctx, s, "!delta", "")

	snap, err := s.FirmwareVersionSnapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap) != 2 {
		t.Fatalf("expected 2 versions (NULL FK excluded), got %d: %+v", len(snap), snap)
	}
	if snap[0].Version != "2.6.5" || snap[0].Count != 2 {
		t.Fatalf("expected first row 2.6.5 count 2, got %+v", snap[0])
	}
	if snap[1].Version != "2.7.10" || snap[1].Count != 1 {
		t.Fatalf("expected second row 2.7.10 count 1, got %+v", snap[1])
	}
}

func TestFirmwareVersionHistory_ZeroPadsMissingWeeksAndKeepsOther(t *testing.T) {
	ctx := context.Background()
	s := newFirmwareTestStore(t)

	// Three versions, three nodes on 2.6.5, two on 2.7.10, one on 2.7.15.
	for _, n := range []string{"!a1", "!a2", "!a3"} {
		seedNode(t, ctx, s, n, "2.6.5")
	}
	for _, n := range []string{"!b1", "!b2"} {
		seedNode(t, ctx, s, n, "2.7.10")
	}
	seedNode(t, ctx, s, "!c1", "2.7.15")

	// Two non-contiguous weeks: index 0 and index 3. Indexes 1, 2 are
	// missing and must be padded with zeros. Top-2 + "other" → 3 columns.
	week0 := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) // Monday
	week3 := week0.AddDate(0, 0, 21)
	if _, err := s.RecordFirmwareHistoryWeek(ctx, week0, week0); err != nil {
		t.Fatalf("record week0: %v", err)
	}
	if _, err := s.RecordFirmwareHistoryWeek(ctx, week3, week3); err != nil {
		t.Fatalf("record week3: %v", err)
	}

	since := week0
	hist, err := s.FirmwareVersionHistory(ctx, since, 2, 4)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if hist.Weeks != 4 || hist.TopN != 2 {
		t.Fatalf("unexpected result metadata: %+v", hist)
	}
	if len(hist.Versions) != 3 {
		t.Fatalf("expected 3 version labels (top-2 + other), got %d: %+v", len(hist.Versions), hist.Versions)
	}
	if hist.Versions[len(hist.Versions)-1] != "(other)" {
		t.Fatalf("expected last label (other), got %q", hist.Versions[len(hist.Versions)-1])
	}

	if len(hist.VersionsByWeek) != 3 {
		t.Fatalf("expected 3 series, got %d", len(hist.VersionsByWeek))
	}
	for i, series := range hist.VersionsByWeek {
		if len(series) != 4 {
			t.Fatalf("series %d expected 4 weekly columns, got %d", i, len(series))
		}
	}

	// Week 0: 3 on 2.6.5, 2 on 2.7.10, 1 on 2.7.15. top-2 picks 2.6.5
	// (3) and 2.7.10 (2); 2.7.15 (1) goes into "(other)".
	if hist.VersionsByWeek[0][0] != 3 {
		t.Errorf("week 0 / 2.6.5: expected 3, got %d", hist.VersionsByWeek[0][0])
	}
	if hist.VersionsByWeek[1][0] != 2 {
		t.Errorf("week 0 / 2.7.10: expected 2, got %d", hist.VersionsByWeek[1][0])
	}
	if hist.VersionsByWeek[2][0] != 1 {
		t.Errorf("week 0 / (other): expected 1, got %d", hist.VersionsByWeek[2][0])
	}

	// Sparse weeks 1, 2: all zeros on every series.
	for weekIdx := 1; weekIdx <= 2; weekIdx++ {
		for sIdx, series := range hist.VersionsByWeek {
			if series[weekIdx] != 0 {
				t.Errorf("sparse week %d / series %d: expected 0, got %d", weekIdx, sIdx, series[weekIdx])
			}
		}
	}

	// Week 3: same shape as week 0.
	if hist.VersionsByWeek[0][3] != 3 {
		t.Errorf("week 3 / 2.6.5: expected 3, got %d", hist.VersionsByWeek[0][3])
	}
	if hist.VersionsByWeek[1][3] != 2 {
		t.Errorf("week 3 / 2.7.10: expected 2, got %d", hist.VersionsByWeek[1][3])
	}
	if hist.VersionsByWeek[2][3] != 1 {
		t.Errorf("week 3 / (other): expected 1, got %d", hist.VersionsByWeek[2][3])
	}
}

func TestRecordFirmwareHistoryWeek_FirstWriterWins(t *testing.T) {
	ctx := context.Background()
	s := newFirmwareTestStore(t)

	seedNode(t, ctx, s, "!alpha", "2.6.5")
	week := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)

	// First write establishes Monday's state.
	firstObserved := week.Add(2 * time.Hour)
	inserted, err := s.RecordFirmwareHistoryWeek(ctx, week, firstObserved)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 row inserted on first snapshot, got %d", inserted)
	}

	// Operator changes the node's version mid-week. The next snapshot
	// for the same week must NOT overwrite the row.
	seedNode(t, ctx, s, "!alpha", "2.7.10")
	secondObserved := week.AddDate(0, 0, 2) // Wednesday of the same week
	inserted, err = s.RecordFirmwareHistoryWeek(ctx, week, secondObserved)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if inserted != 0 {
		t.Fatalf("expected 0 rows inserted on re-snapshot of same week, got %d", inserted)
	}

	// Read back: row must reflect Monday's state.
	var observedAt, versionString string
	if err := s.db.QueryRowContext(ctx, `
SELECT h.observed_at, fv.version_string
FROM node_firmware_history h
JOIN firmware_versions fv ON fv.id = h.firmware_version_id
WHERE h.node_id = ? AND h.week_start = ?`,
		"!alpha", "2026-05-04").Scan(&observedAt, &versionString); err != nil {
		t.Fatalf("read history row: %v", err)
	}
	if versionString != "2.6.5" {
		t.Errorf("expected Monday's 2.6.5 to win, got %q", versionString)
	}
	if observedAt != firstObserved.UTC().Format(time.RFC3339Nano) {
		t.Errorf("expected Monday's observed_at %q to win, got %q",
			firstObserved.UTC().Format(time.RFC3339Nano), observedAt)
	}
}

func TestLastFirmwareHistoryWeek_ReturnsZeroWhenEmpty(t *testing.T) {
	ctx := context.Background()
	s := newFirmwareTestStore(t)

	got, err := s.LastFirmwareHistoryWeek(ctx)
	if err != nil {
		t.Fatalf("last week: %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("expected zero time on empty history, got %s", got)
	}
}

func TestUpsertFirmwareVersion_BumpsLastSeenOnConflict(t *testing.T) {
	ctx := context.Background()
	s := newFirmwareTestStore(t)

	earlier := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	id1, err := s.UpsertFirmwareVersion(ctx, "2.6.5", earlier)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if id1 == 0 {
		t.Fatalf("expected non-zero version id on first upsert")
	}

	id2, err := s.UpsertFirmwareVersion(ctx, "2.6.5", later)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same id on conflict, got %d vs %d", id2, id1)
	}

	var lastSeen string
	if err := s.db.QueryRowContext(ctx,
		`SELECT last_seen_at FROM firmware_versions WHERE id = ?`, id1).Scan(&lastSeen); err != nil {
		t.Fatalf("read last_seen_at: %v", err)
	}
	if lastSeen != later.UTC().Format(time.RFC3339Nano) {
		t.Errorf("expected last_seen_at %q, got %q", later.UTC().Format(time.RFC3339Nano), lastSeen)
	}
}

// Ensure FirmwareHistoryResult is the shape produced by the store — the
// API serializer relies on it being a stable type.
func TestFirmwareHistoryResult_TypeStability(t *testing.T) {
	var _ = repo.FirmwareHistoryResult{
		Weeks:          1,
		TopN:           1,
		Versions:       []string{"x"},
		VersionsByWeek: [][]int{{1}},
	}
}
