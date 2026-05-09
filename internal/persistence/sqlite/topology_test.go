package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

func TestUpsertTopologyEdges_KeepsDistinctSourceKindsAndSupportsFilters(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, nodeID := range []string{"!49b5976c", "!11111111", "!22222222"} {
		if _, err := s.UpsertNode(ctx, domain.Node{
			NodeID:             nodeID,
			FirstSeenAt:        now,
			LastSeenAnyEventAt: now,
			UpdatedAt:          now,
		}); err != nil {
			t.Fatalf("upsert node %s: %v", nodeID, err)
		}
	}

	reportedAt := now.Add(-time.Minute)
	snr := 12.5
	interval := uint32(14400)
	if err := s.UpsertTopologyEdges(ctx, []domain.TopologyEdge{
		{
			SourceKind:                   domain.TopologySourceNeighborInfo,
			ChannelName:                  "LongFast",
			FromNodeID:                   "!49b5976c",
			ToNodeID:                     "!11111111",
			ReportedByNodeID:             "!49b5976c",
			SNR:                          &snr,
			NeighborBroadcastIntervalSec: &interval,
			FirstObservedAt:              now,
			LastObservedAt:               now,
			LastReportedAt:               &reportedAt,
			UpdatedAt:                    now,
		},
		{
			SourceKind:       domain.TopologySourceRoutingForward,
			ChannelName:      "LongFast",
			FromNodeID:       "!49b5976c",
			ToNodeID:         "!11111111",
			ReportedByNodeID: "!22222222",
			FirstObservedAt:  now,
			LastObservedAt:   now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("initial upsert topology edges: %v", err)
	}

	later := now.Add(30 * time.Second)
	if err := s.UpsertTopologyEdges(ctx, []domain.TopologyEdge{{
		SourceKind:       domain.TopologySourceNeighborInfo,
		ChannelName:      "LongFast",
		FromNodeID:       "!49b5976c",
		ToNodeID:         "!11111111",
		ReportedByNodeID: "!49b5976c",
		FirstObservedAt:  later,
		LastObservedAt:   later,
		UpdatedAt:        later,
	}}); err != nil {
		t.Fatalf("second upsert topology edge: %v", err)
	}

	all, err := s.ListTopologyEdges(ctx, repo.TopologyEdgeQuery{})
	if err != nil {
		t.Fatalf("list topology edges: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 topology rows, got %#v", all)
	}

	neighborEdges, err := s.ListTopologyEdges(ctx, repo.TopologyEdgeQuery{
		NodeID:      "!49b5976c",
		Channel:     "LongFast",
		SourceKinds: []domain.TopologySourceKind{domain.TopologySourceNeighborInfo},
	})
	if err != nil {
		t.Fatalf("list filtered topology edges: %v", err)
	}
	if len(neighborEdges) != 1 {
		t.Fatalf("expected 1 filtered topology row, got %#v", neighborEdges)
	}
	if neighborEdges[0].SNR == nil || *neighborEdges[0].SNR != 12.5 {
		t.Fatalf("expected snr to survive nil update, got %#v", neighborEdges[0].SNR)
	}
	if !neighborEdges[0].LastObservedAt.Equal(later) {
		t.Fatalf("expected last observed to update, got %v want %v", neighborEdges[0].LastObservedAt, later)
	}

	var sourceKind int
	if err := s.db.QueryRowContext(ctx, `SELECT source_kind FROM topology_edges WHERE channel_name=? AND from_node_id=? AND to_node_id=?`, "LongFast", "!49b5976c", "!11111111").Scan(&sourceKind); err != nil {
		t.Fatalf("query stored topology source_kind: %v", err)
	}
	if sourceKind != domain.TopologySourceNeighborInfoValue {
		t.Fatalf("expected compact source_kind %d, got %d", domain.TopologySourceNeighborInfoValue, sourceKind)
	}
}
