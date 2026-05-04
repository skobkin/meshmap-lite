package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"meshmap-lite/internal/config"
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

func TestStatsActivityHandlerReturnsConfiguredPeriodsAndReusesCache(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 7, 0, 0, time.UTC)
	calls := 0
	store := &testkit.FakeStore{
		ActivityBucketsFn: func(_ context.Context, q domain.ActivityQuery) ([]domain.ActivityBucket, error) {
			calls++
			if q.End.Second() != 0 || q.End.Nanosecond() != 0 {
				t.Fatalf("expected aligned end time, got %s", q.End)
			}

			return []domain.ActivityBucket{{
				BucketStart:  q.Start,
				TextMessages: calls,
			}}, nil
		},
	}
	srv := New(Config{
		Web: config.WebConfig{
			Stats: config.StatsConfig{
				Activity: config.StatsActivityConfig{
					Daily:  config.StatsActivityBucketConfig{Bucket: 5 * time.Minute},
					Weekly: config.StatsActivityBucketConfig{Bucket: time.Hour},
				},
			},
		},
	}, store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/activity", nil)
		rec := httptest.NewRecorder()
		srv.statsActivity(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload activityPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.Periods) != 2 {
			t.Fatalf("expected two periods, got %d", len(payload.Periods))
		}
		if payload.Periods[0].Key != "daily" || payload.Periods[0].Window != "24h" || payload.Periods[0].Bucket != "5m" {
			t.Fatalf("unexpected daily period: %+v", payload.Periods[0])
		}
	}
	if calls != 2 {
		t.Fatalf("expected one store call per period due cache reuse, got %d", calls)
	}

	srv.now = func() time.Time { return now.Add(5 * time.Minute) }
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/activity", nil)
	rec := httptest.NewRecorder()
	srv.statsActivity(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status after daily expiry: %d", rec.Code)
	}
	if calls != 3 {
		t.Fatalf("expected daily cache to expire on next boundary, got %d calls", calls)
	}

	srv.now = func() time.Time { return now.Add(time.Hour) }
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats/activity", nil)
	rec = httptest.NewRecorder()
	srv.statsActivity(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status after weekly expiry: %d", rec.Code)
	}
	if calls != 5 {
		t.Fatalf("expected both caches to expire after one hour, got %d calls", calls)
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
