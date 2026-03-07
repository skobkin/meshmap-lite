package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
)

func TestUpsertNode_CreatedFlagOnFirstInsertOnly(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	firstSeen := time.Now().UTC().Truncate(time.Microsecond)
	created, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		LongName:           "Alpha",
		FirstSeenAt:        firstSeen,
		LastSeenAnyEventAt: firstSeen,
		UpdatedAt:          firstSeen,
	})
	if err != nil {
		t.Fatalf("first upsert node: %v", err)
	}
	if !created {
		t.Fatalf("expected first upsert to report created")
	}

	secondSeen := firstSeen.Add(10 * time.Second)
	created, err = s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		ShortName:          "A",
		FirstSeenAt:        secondSeen,
		LastSeenAnyEventAt: secondSeen,
		UpdatedAt:          secondSeen,
	})
	if err != nil {
		t.Fatalf("second upsert node: %v", err)
	}
	if created {
		t.Fatalf("expected second upsert to report existing row")
	}

	var storedFirstSeen string
	if err := s.db.QueryRowContext(ctx, `SELECT first_seen_at FROM nodes WHERE node_id = ?`, "!aaaa1111").Scan(&storedFirstSeen); err != nil {
		t.Fatalf("query first_seen_at: %v", err)
	}
	if storedFirstSeen != firstSeen.Format(time.RFC3339Nano) {
		t.Fatalf("expected first_seen_at %q, got %q", firstSeen.Format(time.RFC3339Nano), storedFirstSeen)
	}
}

func TestUpsertPosition_UpdatesNodeLastSeenPositionAt(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!bbbb2222",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	observedAt := now.Add(30 * time.Second)
	if err := s.UpsertPosition(ctx, domain.NodePosition{
		NodeID:        "!bbbb2222",
		Latitude:      10.1,
		Longitude:     20.2,
		ObservedAt:    observedAt,
		UpdatedAt:     observedAt,
		SourceKind:    domain.PositionSourceChannel,
		SourceChannel: "LongFast",
	}); err != nil {
		t.Fatalf("upsert position: %v", err)
	}

	var lastSeenPositionAt, updatedAt string
	if err := s.db.QueryRowContext(ctx, `SELECT last_seen_position_at, updated_at FROM nodes WHERE node_id = ?`, "!bbbb2222").Scan(&lastSeenPositionAt, &updatedAt); err != nil {
		t.Fatalf("query node timestamps: %v", err)
	}
	wantTS := observedAt.Format(time.RFC3339Nano)
	if lastSeenPositionAt != wantTS {
		t.Fatalf("expected last_seen_position_at %q, got %q", wantTS, lastSeenPositionAt)
	}
	if updatedAt != wantTS {
		t.Fatalf("expected updated_at %q, got %q", wantTS, updatedAt)
	}
}

func TestInsertLogEvent_CachesChannelIDs(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
			ObservedAt: now.Add(time.Duration(i) * time.Second),
			NodeID:     "!cccc3333",
			EventKind:  domain.LogEventKindTelemetryValue,
			Channel:    "LongFast",
		}); err != nil {
			t.Fatalf("insert log event #%d: %v", i+1, err)
		}
	}

	var channels int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM log_channels`).Scan(&channels); err != nil {
		t.Fatalf("count log channels: %v", err)
	}
	if channels != 1 {
		t.Fatalf("expected exactly one log channel row, got %d", channels)
	}
	if len(s.logChannelIDs) != 1 {
		t.Fatalf("expected one cached log channel id, got %d", len(s.logChannelIDs))
	}
}

func TestGetNodeDetails_WithTelemetryOnSingleConnection(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!dddd4444",
		LongName:           "Delta",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := s.MergeTelemetry(ctx, domain.NodeTelemetrySnapshot{
		NodeID:     "!dddd4444",
		ObservedAt: now,
		UpdatedAt:  now,
		Power: domain.TelemetrySectionPower{
			Voltage: ptrFloat64(4.1),
		},
	}); err != nil {
		t.Fatalf("merge telemetry: %v", err)
	}

	detailsCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	details, err := s.GetNodeDetails(detailsCtx, "!dddd4444")
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if details.Node.NodeID != "!dddd4444" {
		t.Fatalf("expected node id !dddd4444, got %q", details.Node.NodeID)
	}
	if details.Telemetry == nil {
		t.Fatalf("expected telemetry to be loaded")
	}
	if details.Telemetry.Power.Voltage == nil || *details.Telemetry.Power.Voltage != 4.1 {
		t.Fatalf("expected voltage 4.1, got %#v", details.Telemetry.Power.Voltage)
	}
}

func TestGetMapNodes_HidesStaleAndMissingPositions(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	recentObservedAt := now.Add(-24 * time.Hour)
	staleObservedAt := now.Add(-(15 * 24 * time.Hour))

	for _, tc := range []struct {
		nodeID     string
		observedAt *time.Time
	}{
		{nodeID: "!recent111", observedAt: &recentObservedAt},
		{nodeID: "!stale222", observedAt: &staleObservedAt},
		{nodeID: "!missing333", observedAt: nil},
	} {
		updatedAt := now
		if tc.observedAt != nil {
			updatedAt = *tc.observedAt
		}
		if _, err := s.UpsertNode(ctx, domain.Node{
			NodeID:             tc.nodeID,
			LongName:           tc.nodeID,
			FirstSeenAt:        now,
			LastSeenAnyEventAt: updatedAt,
			UpdatedAt:          updatedAt,
		}); err != nil {
			t.Fatalf("upsert node %s: %v", tc.nodeID, err)
		}
		if tc.observedAt == nil {
			continue
		}
		if err := s.UpsertPosition(ctx, domain.NodePosition{
			NodeID:     tc.nodeID,
			Latitude:   10.1,
			Longitude:  20.2,
			ObservedAt: *tc.observedAt,
			UpdatedAt:  *tc.observedAt,
			SourceKind: domain.PositionSourceChannel,
		}); err != nil {
			t.Fatalf("upsert position %s: %v", tc.nodeID, err)
		}
	}

	items, err := s.GetMapNodes(ctx, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("get map nodes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 visible map node, got %d", len(items))
	}
	if items[0].Node.NodeID != "!recent111" {
		t.Fatalf("expected recent node to remain visible, got %q", items[0].Node.NodeID)
	}
	if items[0].Position == nil {
		t.Fatalf("expected visible map node to include position")
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
