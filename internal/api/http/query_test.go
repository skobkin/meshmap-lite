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
		"event_kind":  []string{"1,2"},
		"event_kinds": []string{"2,999,1"},
	}

	got := parseLogQuery(values, config.LogConfig{PageSizeDefault: 100})
	if got.Limit != 25 || got.BeforeID != 44 || got.Channel != "LongFast" {
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
