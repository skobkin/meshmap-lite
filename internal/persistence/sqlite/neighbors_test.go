package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

func TestGetNodeDetails_CollapsesTopologyNeighbors(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, node := range []domain.Node{
		{NodeID: "!origin", LongName: "Origin", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
		{NodeID: "!peer-a", LongName: "Peer A", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
		{NodeID: "!peer-b", ShortName: "Peer B", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
		{NodeID: "!peer-c", LongName: "Peer C", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
		{NodeID: "!peer-d", LongName: "Peer D", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
		{NodeID: "!peer-ignore", LongName: "Ignored", FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now},
	} {
		if _, err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("upsert node %s: %v", node.NodeID, err)
		}
	}
	if err := s.UpsertPosition(ctx, domain.NodePosition{
		NodeID:     "!peer-a",
		Latitude:   1,
		Longitude:  2,
		ObservedAt: now,
		UpdatedAt:  now,
		SourceKind: domain.PositionSourceChannel,
	}); err != nil {
		t.Fatalf("upsert peer-a position: %v", err)
	}

	snr := 9.5
	later := now.Add(time.Minute)
	if err := s.UpsertTopologyEdges(ctx, []domain.TopologyEdge{
		{
			SourceKind:       domain.TopologySourceRoutingForward,
			ChannelName:      "LongFast",
			FromNodeID:       "!origin",
			ToNodeID:         "!peer-a",
			ReportedByNodeID: "!origin",
			FirstObservedAt:  now,
			LastObservedAt:   now,
			UpdatedAt:        now,
		},
		{
			SourceKind:       domain.TopologySourceNeighborInfo,
			ChannelName:      "LongFast",
			FromNodeID:       "!origin",
			ToNodeID:         "!peer-a",
			ReportedByNodeID: "!origin",
			SNR:              &snr,
			FirstObservedAt:  later,
			LastObservedAt:   later,
			UpdatedAt:        later,
		},
		{
			SourceKind:       domain.TopologySourceNeighborInfo,
			ChannelName:      "LongFast",
			FromNodeID:       "!origin",
			ToNodeID:         "!peer-b",
			ReportedByNodeID: "!origin",
			FirstObservedAt:  now,
			LastObservedAt:   now,
			UpdatedAt:        now,
		},
		{
			SourceKind:       domain.TopologySourceRoutingReturn,
			ChannelName:      "LongFast",
			FromNodeID:       "!peer-c",
			ToNodeID:         "!origin",
			ReportedByNodeID: "!peer-c",
			FirstObservedAt:  now,
			LastObservedAt:   now,
			UpdatedAt:        now,
		},
		{
			SourceKind:       domain.TopologySourceMQTTDirect,
			ChannelName:      "LongFast",
			FromNodeID:       "!peer-d",
			ToNodeID:         "!origin",
			ReportedByNodeID: "!origin",
			Inferred:         true,
			FirstObservedAt:  later,
			LastObservedAt:   later,
			UpdatedAt:        later,
		},
		{
			SourceKind:       domain.TopologySourceTracerouteForward,
			ChannelName:      "LongFast",
			FromNodeID:       "!origin",
			ToNodeID:         "!peer-ignore",
			ReportedByNodeID: "!origin",
			FirstObservedAt:  now,
			LastObservedAt:   now,
			UpdatedAt:        now,
		},
	}); err != nil {
		t.Fatalf("upsert topology edges: %v", err)
	}

	details, err := s.GetNodeDetails(ctx, "!origin")
	if err != nil {
		t.Fatalf("get node details: %v", err)
	}
	if len(details.Neighbors) != 4 {
		t.Fatalf("expected 4 collapsed neighbors, got %#v", details.Neighbors)
	}
	neighborsByID := make(map[string]repo.NodeNeighbor, len(details.Neighbors))
	for _, item := range details.Neighbors {
		neighborsByID[item.NodeID] = item
	}
	if neighborsByID["!peer-a"].EvidenceKind != "neighbor_info" || neighborsByID["!peer-a"].SNR == nil {
		t.Fatalf("expected peer-a to prefer neighbor info with snr, got %#v", neighborsByID["!peer-a"])
	}
	if !neighborsByID["!peer-a"].HasPosition {
		t.Fatalf("expected peer-a to report has_position")
	}
	if neighborsByID["!peer-b"].EvidenceKind != "neighbor_info" || neighborsByID["!peer-b"].SNR != nil {
		t.Fatalf("expected peer-b to keep neighbor info without snr, got %#v", neighborsByID["!peer-b"])
	}
	if neighborsByID["!peer-c"].EvidenceKind != "inferred" {
		t.Fatalf("expected peer-c inferred neighbor, got %#v", neighborsByID["!peer-c"])
	}
	if neighborsByID["!peer-d"].EvidenceKind != "mqtt_direct" {
		t.Fatalf("expected peer-d mqtt_direct neighbor, got %#v", neighborsByID["!peer-d"])
	}
	if details.Neighbors[2].NodeID != "!peer-d" || details.Neighbors[3].NodeID != "!peer-c" {
		t.Fatalf("expected mqtt_direct to sort above inferred, got %#v", details.Neighbors)
	}
}
