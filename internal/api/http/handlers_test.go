package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
	"meshmap-lite/internal/repo/testkit"
)

func TestTopologyEdgesHandlerReturnsFilteredItems(t *testing.T) {
	store := &testkit.FakeStore{
		ListTopologyEdgesFn: func(_ context.Context, q repo.TopologyEdgeQuery) ([]domain.TopologyEdge, error) {
			if q.NodeID != "!49b5976c" || q.Channel != "LongFast" {
				t.Fatalf("unexpected query: %+v", q)
			}
			if len(q.SourceKinds) != 2 ||
				q.SourceKinds[0] != domain.TopologySourceNeighborInfo ||
				q.SourceKinds[1] != domain.TopologySourceRoutingReturn {
				t.Fatalf("unexpected source kinds: %+v", q.SourceKinds)
			}

			now := time.Unix(1772296589, 0).UTC()

			return []domain.TopologyEdge{{
				SourceKind:       domain.TopologySourceNeighborInfo,
				ChannelName:      "LongFast",
				FromNodeID:       "!49b5976c",
				ToNodeID:         "!11111111",
				ReportedByNodeID: "!49b5976c",
				FirstObservedAt:  now,
				LastObservedAt:   now,
				UpdatedAt:        now,
			}}, nil
		},
	}

	srv := New(Config{}, store, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/edges?node_id=!49b5976c&channel=LongFast&source_kind=neighbor_info,routing_return", nil)
	rec := httptest.NewRecorder()

	srv.topologyEdges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var items []domain.TopologyEdge
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 || items[0].SourceKind != domain.TopologySourceNeighborInfo {
		t.Fatalf("unexpected response payload: %#v", items)
	}
}

func TestNodeByIDReturnsNeighbors(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	store := &testkit.FakeStore{
		GetNodeDetailsFn: func(_ context.Context, nodeID string) (repo.NodeDetails, error) {
			if nodeID != "!49b5976c" {
				t.Fatalf("unexpected node id: %q", nodeID)
			}

			return repo.NodeDetails{
				Node: domain.Node{
					NodeID:             nodeID,
					LastSeenAnyEventAt: now,
					FirstSeenAt:        now,
					UpdatedAt:          now,
				},
				Neighbors: []repo.NodeNeighbor{{
					NodeID:         "!11111111",
					DisplayName:    "Alpha",
					HasPosition:    true,
					EvidenceKind:   "neighbor_info",
					LastObservedAt: now,
				}},
			}, nil
		},
	}

	srv := New(Config{}, store, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/%2149b5976c", nil)
	rec := httptest.NewRecorder()

	srv.nodeByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var item repo.NodeDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(item.Neighbors) != 1 || item.Neighbors[0].DisplayName != "Alpha" {
		t.Fatalf("unexpected neighbors payload: %#v", item.Neighbors)
	}
}
