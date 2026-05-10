package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
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

func TestUpsertNode_FirstInsertWithNamesCreatesNoNameHistory(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		LongName:           "Alpha",
		ShortName:          "A",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	if got := countNodeNameHistory(t, ctx, s, "!aaaa1111"); got != 0 {
		t.Fatalf("expected no name history on first insert, got %d", got)
	}
}

func TestUpsertNode_RecordsNameHistoryForEffectiveChanges(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	firstSeen := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		LongName:           "Alpha",
		ShortName:          "A",
		FirstSeenAt:        firstSeen,
		LastSeenAnyEventAt: firstSeen,
		UpdatedAt:          firstSeen,
	}); err != nil {
		t.Fatalf("initial upsert node: %v", err)
	}

	longChangedAt := firstSeen.Add(time.Minute)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		LongName:           "Bravo",
		FirstSeenAt:        longChangedAt,
		LastSeenAnyEventAt: longChangedAt,
		UpdatedAt:          longChangedAt,
	}); err != nil {
		t.Fatalf("long-name upsert node: %v", err)
	}

	shortChangedAt := firstSeen.Add(2 * time.Minute)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		ShortName:          "B",
		FirstSeenAt:        shortChangedAt,
		LastSeenAnyEventAt: shortChangedAt,
		UpdatedAt:          shortChangedAt,
	}); err != nil {
		t.Fatalf("short-name upsert node: %v", err)
	}

	details, err := s.GetNodeDetails(ctx, repo.NodeDetailsQuery{NodeID: "!aaaa1111"})
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if len(details.PreviousNames) != 2 {
		t.Fatalf("expected 2 history rows, got %#v", details.PreviousNames)
	}
	if got := details.PreviousNames[0]; got.PreviousLongName != "Bravo" || got.PreviousShortName != "A" || got.NewLongName != "Bravo" || got.NewShortName != "B" || !got.ChangedAt.Equal(shortChangedAt) {
		t.Fatalf("unexpected newest history row: %#v", got)
	}
	if got := details.PreviousNames[1]; got.PreviousLongName != "Alpha" || got.PreviousShortName != "A" || got.NewLongName != "Bravo" || got.NewShortName != "A" || !got.ChangedAt.Equal(longChangedAt) {
		t.Fatalf("unexpected older history row: %#v", got)
	}
}

func TestUpsertNode_NameHistoryIgnoresEmptyAndDuplicateEvidence(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	firstSeen := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		LongName:           "Alpha",
		ShortName:          "A",
		FirstSeenAt:        firstSeen,
		LastSeenAnyEventAt: firstSeen,
		UpdatedAt:          firstSeen,
	}); err != nil {
		t.Fatalf("initial upsert node: %v", err)
	}

	for _, node := range []domain.Node{
		{
			NodeID:             "!aaaa1111",
			FirstSeenAt:        firstSeen.Add(time.Minute),
			LastSeenAnyEventAt: firstSeen.Add(time.Minute),
			UpdatedAt:          firstSeen.Add(time.Minute),
		},
		{
			NodeID:             "!aaaa1111",
			LongName:           "Alpha",
			ShortName:          "A",
			FirstSeenAt:        firstSeen.Add(2 * time.Minute),
			LastSeenAnyEventAt: firstSeen.Add(2 * time.Minute),
			UpdatedAt:          firstSeen.Add(2 * time.Minute),
		},
	} {
		if _, err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("upsert node: %v", err)
		}
	}

	if got := countNodeNameHistory(t, ctx, s, "!aaaa1111"); got != 0 {
		t.Fatalf("expected no name history from empty or duplicate names, got %d", got)
	}

	details, err := s.GetNodeDetails(ctx, repo.NodeDetailsQuery{NodeID: "!aaaa1111"})
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if details.Node.LongName != "Alpha" || details.Node.ShortName != "A" {
		t.Fatalf("unexpected effective names: %#v", details.Node)
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

func TestUpsertPosition_SkipsZeroCoordinates(t *testing.T) {
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
		NodeID:     "!bbbb2222",
		Latitude:   0,
		Longitude:  0,
		ObservedAt: observedAt,
		UpdatedAt:  observedAt,
		SourceKind: domain.PositionSourceChannel,
	}); err != nil {
		t.Fatalf("upsert position: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_positions WHERE node_id = ?`, "!bbbb2222").Scan(&count); err != nil {
		t.Fatalf("count node positions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no stored zero position, got %d", count)
	}

	var lastSeenPositionAt sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT last_seen_position_at FROM nodes WHERE node_id = ?`, "!bbbb2222").Scan(&lastSeenPositionAt); err != nil {
		t.Fatalf("query node timestamp: %v", err)
	}
	if lastSeenPositionAt.Valid {
		t.Fatalf("expected last_seen_position_at to remain null, got %q", lastSeenPositionAt.String)
	}
}

func TestUpsertPosition_AllowsSingleZeroCoordinate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		nodeID    string
		latitude  float64
		longitude float64
	}{
		{name: "zero latitude", nodeID: "!bbbb2222", latitude: 0, longitude: 20.2},
		{name: "zero longitude", nodeID: "!cccc3333", latitude: 10.1, longitude: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
			if err != nil {
				t.Fatalf("open sqlite: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })

			now := time.Now().UTC().Truncate(time.Microsecond)
			if _, err := s.UpsertNode(ctx, domain.Node{
				NodeID:             tc.nodeID,
				FirstSeenAt:        now,
				LastSeenAnyEventAt: now,
				UpdatedAt:          now,
			}); err != nil {
				t.Fatalf("upsert node: %v", err)
			}
			if err := s.UpsertPosition(ctx, domain.NodePosition{
				NodeID:     tc.nodeID,
				Latitude:   tc.latitude,
				Longitude:  tc.longitude,
				ObservedAt: now,
				UpdatedAt:  now,
				SourceKind: domain.PositionSourceChannel,
			}); err != nil {
				t.Fatalf("upsert position: %v", err)
			}

			var latitude, longitude float64
			if err := s.db.QueryRowContext(ctx, `SELECT latitude, longitude FROM node_positions WHERE node_id = ?`, tc.nodeID).Scan(&latitude, &longitude); err != nil {
				t.Fatalf("query node position: %v", err)
			}
			if latitude != tc.latitude || longitude != tc.longitude {
				t.Fatalf("unexpected position: got (%v,%v), want (%v,%v)", latitude, longitude, tc.latitude, tc.longitude)
			}
		})
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
	if _, err := s.MergeTelemetry(ctx, domain.NodeTelemetrySnapshot{
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

	details, err := s.GetNodeDetails(detailsCtx, repo.NodeDetailsQuery{NodeID: "!dddd4444"})
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
	if len(details.Neighbors) != 0 {
		t.Fatalf("expected no topology neighbors, got %#v", details.Neighbors)
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

	items, err := s.GetMapNodes(ctx, repo.MapNodeQuery{PositionObservedSince: now.Add(-14 * 24 * time.Hour)})
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
	if items[0].Telemetry != nil {
		t.Fatalf("expected map node without telemetry to have nil telemetry, got %#v", items[0].Telemetry)
	}
}

func TestGetMapNodes_OmitsStaleTelemetry(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!telemetry",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := s.UpsertPosition(ctx, domain.NodePosition{
		NodeID:     "!telemetry",
		Latitude:   10.1,
		Longitude:  20.2,
		ObservedAt: now,
		UpdatedAt:  now,
		SourceKind: domain.PositionSourceChannel,
	}); err != nil {
		t.Fatalf("upsert position: %v", err)
	}
	staleTelemetryAt := now.Add(-25 * time.Hour)
	if _, err := s.MergeTelemetry(ctx, domain.NodeTelemetrySnapshot{
		NodeID:     "!telemetry",
		ObservedAt: staleTelemetryAt,
		UpdatedAt:  staleTelemetryAt,
		Power:      domain.TelemetrySectionPower{Voltage: ptrFloat64(4.1)},
	}); err != nil {
		t.Fatalf("merge telemetry: %v", err)
	}

	items, err := s.GetMapNodes(ctx, repo.MapNodeQuery{
		PositionObservedSince:  now.Add(-14 * 24 * time.Hour),
		TelemetryObservedSince: now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("get map nodes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected visible node, got %#v", items)
	}
	if items[0].Telemetry != nil {
		t.Fatalf("expected stale telemetry to be omitted, got %#v", items[0].Telemetry)
	}
}

func TestNodeDetailsAndListApplyRelevanceCutoffs(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	stalePositionAt := now.Add(-15 * 24 * time.Hour)
	staleTelemetryAt := now.Add(-25 * time.Hour)
	staleTopologyAt := now.Add(-73 * time.Hour)
	for _, node := range []domain.Node{
		{NodeID: "!origin", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
		{NodeID: "!peer", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
	} {
		if _, err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("upsert node %s: %v", node.NodeID, err)
		}
	}
	if err := s.UpsertPosition(ctx, domain.NodePosition{
		NodeID:     "!origin",
		Latitude:   10.1,
		Longitude:  20.2,
		ObservedAt: stalePositionAt,
		UpdatedAt:  stalePositionAt,
		SourceKind: domain.PositionSourceChannel,
	}); err != nil {
		t.Fatalf("upsert stale position: %v", err)
	}
	if _, err := s.MergeTelemetry(ctx, domain.NodeTelemetrySnapshot{
		NodeID:     "!origin",
		ObservedAt: staleTelemetryAt,
		UpdatedAt:  staleTelemetryAt,
		Power:      domain.TelemetrySectionPower{Voltage: ptrFloat64(4.1)},
	}); err != nil {
		t.Fatalf("merge stale telemetry: %v", err)
	}
	if err := s.UpsertTopologyEdges(ctx, []domain.TopologyEdge{{
		SourceKind:      domain.TopologySourceNeighborInfo,
		ChannelName:     "LongFast",
		FromNodeID:      "!origin",
		ToNodeID:        "!peer",
		FirstObservedAt: staleTopologyAt,
		LastObservedAt:  staleTopologyAt,
		UpdatedAt:       staleTopologyAt,
	}}); err != nil {
		t.Fatalf("upsert stale topology: %v", err)
	}

	query := repo.NodeDetailsQuery{
		NodeID:                 "!origin",
		PositionObservedSince:  now.Add(-14 * 24 * time.Hour),
		TelemetryObservedSince: now.Add(-24 * time.Hour),
		TopologyUpdatedSince:   now.Add(-72 * time.Hour),
	}
	details, err := s.GetNodeDetails(ctx, query)
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if details.Position != nil {
		t.Fatalf("expected stale position to be omitted, got %#v", details.Position)
	}
	if details.Telemetry != nil {
		t.Fatalf("expected stale telemetry to be omitted, got %#v", details.Telemetry)
	}
	if len(details.Neighbors) != 0 {
		t.Fatalf("expected stale neighbors to be omitted, got %#v", details.Neighbors)
	}

	summaries, err := s.ListNodes(ctx, repo.NodeListQuery{PositionObservedSince: query.PositionObservedSince})
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	for _, summary := range summaries {
		if summary.NodeID == "!origin" && (summary.HasPosition || summary.LastSeenPositionAt != nil) {
			t.Fatalf("expected stale position metadata to be omitted, got %#v", summary)
		}
	}
}

func TestNodeProvenancePersistsForMapDetailsPositionAndTelemetry(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:                 "!sender",
		LongName:               "Sender",
		FirstSeenAt:            now,
		LastSeenAnyEventAt:     now,
		LastMQTTUploaderNodeID: "!gateway",
		LastMQTTUploaderAt:     &now,
		UpdatedAt:              now,
	}); err != nil {
		t.Fatalf("upsert sender: %v", err)
	}
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!gateway",
		LongName:           "Gateway",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert gateway: %v", err)
	}
	if err := s.UpsertPosition(ctx, domain.NodePosition{
		NodeID:             "!sender",
		Latitude:           10.1,
		Longitude:          20.2,
		ObservedAt:         now,
		UpdatedAt:          now,
		SourceKind:         domain.PositionSourceChannel,
		MQTTUploaderNodeID: "!gateway",
	}); err != nil {
		t.Fatalf("upsert position: %v", err)
	}
	if _, err := s.MergeTelemetry(ctx, domain.NodeTelemetrySnapshot{
		NodeID:             "!sender",
		MQTTUploaderNodeID: "!gateway",
		ObservedAt:         now,
		UpdatedAt:          now,
		Power: domain.TelemetrySectionPower{
			Voltage: ptrFloat64(4.1),
		},
	}); err != nil {
		t.Fatalf("merge telemetry: %v", err)
	}

	details, err := s.GetNodeDetails(ctx, repo.NodeDetailsQuery{NodeID: "!sender"})
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if details.Node.LastMQTTUploaderNodeID != "!gateway" || details.Node.LastMQTTUploaderDisplayName != "Gateway" {
		t.Fatalf("unexpected node uploader: %#v", details.Node)
	}
	if details.Position == nil || details.Position.MQTTUploaderNodeID != "!gateway" || details.Position.MQTTUploaderDisplayName != "Gateway" {
		t.Fatalf("unexpected position uploader: %#v", details.Position)
	}
	if details.Telemetry == nil || details.Telemetry.MQTTUploaderNodeID != "!gateway" || details.Telemetry.MQTTUploaderDisplayName != "Gateway" {
		t.Fatalf("unexpected telemetry uploader: %#v", details.Telemetry)
	}

	mapNodes, err := s.GetMapNodes(ctx, repo.MapNodeQuery{
		PositionObservedSince:  now.Add(-24 * time.Hour),
		TelemetryObservedSince: now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("get map nodes: %v", err)
	}
	if len(mapNodes) != 1 || mapNodes[0].Node.LastMQTTUploaderDisplayName != "Gateway" {
		t.Fatalf("unexpected map node uploader: %#v", mapNodes)
	}
}

func TestUpsertNode_MinimalEvidenceDoesNotClearStructuredFields(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	firstSeen := time.Now().UTC().Truncate(time.Microsecond)
	mqttCapable := true
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:                "!aaaa1111",
		LongName:              "Alpha",
		ShortName:             "A",
		MQTTGatewayCapable:    &mqttCapable,
		FirstSeenAt:           firstSeen,
		LastSeenAnyEventAt:    firstSeen,
		LastSeenMQTTGatewayAt: &firstSeen,
		UpdatedAt:             firstSeen,
	}); err != nil {
		t.Fatalf("initial upsert node: %v", err)
	}

	observed := firstSeen.Add(30 * time.Second)
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!aaaa1111",
		FirstSeenAt:        observed,
		LastSeenAnyEventAt: observed,
		UpdatedAt:          observed,
	}); err != nil {
		t.Fatalf("minimal evidence upsert node: %v", err)
	}

	details, err := s.GetNodeDetails(ctx, repo.NodeDetailsQuery{NodeID: "!aaaa1111"})
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if details.Node.LongName != "Alpha" || details.Node.ShortName != "A" {
		t.Fatalf("minimal evidence cleared identity fields: %#v", details.Node)
	}
	if details.Node.MQTTGatewayCapable == nil || !*details.Node.MQTTGatewayCapable {
		t.Fatalf("minimal evidence cleared mqtt capability: %#v", details.Node.MQTTGatewayCapable)
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func countNodeNameHistory(t *testing.T, ctx context.Context, s *Store, nodeID string) int {
	t.Helper()

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_name_history WHERE node_id = ?`, nodeID).Scan(&count); err != nil {
		t.Fatalf("count node name history: %v", err)
	}

	return count
}
