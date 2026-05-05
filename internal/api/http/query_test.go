package httpapi

import (
	"net/url"
	"testing"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
)

func TestParseLogQueryDeduplicatesKinds(t *testing.T) {
	values := url.Values{
		"limit":       []string{"25"},
		"before":      []string{"44"},
		"channel":     []string{"LongFast"},
		"node_id":     []string{"!49b5976c"},
		"event_kind":  []string{"1,2"},
		"event_kinds": []string{"2,999,1"},
	}

	got := parseLogQuery(values, config.LogConfig{PageSizeDefault: 100})
	if got.Limit != 25 || got.BeforeID != 44 || got.Channel != "LongFast" || got.NodeID != "!49b5976c" {
		t.Fatalf("unexpected parsed query: %+v", got)
	}
	if len(got.EventKinds) != 2 || got.EventKinds[0] != domain.LogEventKindMapReportValue || got.EventKinds[1] != domain.LogEventKindNodeInfoValue {
		t.Fatalf("unexpected event kinds: %+v", got.EventKinds)
	}
}

func TestNodeIDFromPath(t *testing.T) {
	nodeID, ok := nodeIDFromPath("/api/v1/nodes/!abcd")
	if !ok || nodeID != "!abcd" {
		t.Fatalf("expected node id, got %q %v", nodeID, ok)
	}

	if _, ok := nodeIDFromPath("/api/v1/nodes/"); ok {
		t.Fatalf("expected empty node path to fail")
	}
	if _, ok := nodeIDFromPath("/api/v1/nodes/a/b"); ok {
		t.Fatalf("expected nested node path to fail")
	}
}

func TestParseTopologyEdgeQueryDeduplicatesKinds(t *testing.T) {
	values := url.Values{
		"node_id":      []string{"!49b5976c"},
		"channel":      []string{"LongFast"},
		"source_kind":  []string{"neighbor_info,traceroute_forward"},
		"source_kinds": []string{"traceroute_forward,invalid,routing_return"},
	}

	got := parseTopologyEdgeQuery(values)
	if got.NodeID != "!49b5976c" || got.Channel != "LongFast" {
		t.Fatalf("unexpected parsed topology query: %+v", got)
	}
	if len(got.SourceKinds) != 3 ||
		got.SourceKinds[0] != domain.TopologySourceNeighborInfo ||
		got.SourceKinds[1] != domain.TopologySourceTracerouteForward ||
		got.SourceKinds[2] != domain.TopologySourceRoutingReturn {
		t.Fatalf("unexpected topology source kinds: %+v", got.SourceKinds)
	}
}
