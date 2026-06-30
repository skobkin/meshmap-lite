package httpapi

import (
	"bytes"
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
	"meshmap-lite/internal/siteinfo"
)

func TestTopologyEdgesHandlerReturnsFilteredItems(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	store := &testkit.FakeStore{
		ListTopologyEdgesFn: func(_ context.Context, q repo.TopologyEdgeQuery) ([]domain.TopologyEdge, error) {
			if q.NodeID != "!49b5976c" || q.Channel != "LongFast" {
				t.Fatalf("unexpected query: %+v", q)
			}
			if len(q.SourceKinds) != 2 ||
				q.SourceKinds[0] != domain.TopologySourceNeighborInfo ||
				q.SourceKinds[1] != domain.TopologySourceMQTTDirect {
				t.Fatalf("unexpected source kinds: %+v", q.SourceKinds)
			}
			if q.Limit != 2000 {
				t.Fatalf("expected limit to default to web.map.topology_max_edges (2000), got %d", q.Limit)
			}
			wantCutoff := now.Add(-72 * time.Hour)
			if !q.UpdatedSince.Equal(wantCutoff) {
				t.Fatalf("expected updated cutoff %s, got %s", wantCutoff, q.UpdatedSince)
			}

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

	srv := New(Config{Web: config.WebConfig{Map: config.MapConfig{TopologyMaxEdges: 2000}, Relevance: config.RelevanceConfig{TopologyEvidenceMaxAge: 72 * time.Hour}}}, store, nil, nil, nil, nil)
	srv.now = func() time.Time { return now }
	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/edges?node_id=!49b5976c&channel=LongFast&source_kind=neighbor_info,mqtt_direct", nil)
	rec := httptest.NewRecorder()

	srv.topologyEdges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload topologyEdgesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].SourceKind != domain.TopologySourceNeighborInfo {
		t.Fatalf("unexpected response payload: %#v", payload.Items)
	}
	if payload.Truncated {
		t.Fatalf("expected truncated=false, got true")
	}
}

func TestTopologyEdgesHandlerReusesCacheAndReportsTruncated(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	calls := 0
	store := &testkit.FakeStore{
		ListTopologyEdgesFn: func(_ context.Context, q repo.TopologyEdgeQuery) ([]domain.TopologyEdge, error) {
			calls++
			if q.Limit != 3 {
				t.Fatalf("expected limit=3, got %d", q.Limit)
			}
			items := make([]domain.TopologyEdge, 0, q.Limit)
			for i := 0; i < q.Limit; i++ {
				items = append(items, domain.TopologyEdge{
					SourceKind:      domain.TopologySourceNeighborInfo,
					ChannelName:     "LongFast",
					FromNodeID:      "!49b5976c",
					ToNodeID:        "!11111111",
					FirstObservedAt: now,
					LastObservedAt:  now.Add(-time.Duration(i) * time.Minute),
					UpdatedAt:       now,
				})
			}

			return items, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Map: config.MapConfig{TopologyMaxEdges: 3, TopologyCacheTTL: time.Minute}}}, store, nil, nil, nil, nil)
	srv.now = func() time.Time { return now }

	hit := func() topologyEdgesPayload {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/edges", nil)
		rec := httptest.NewRecorder()
		srv.topologyEdges(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload topologyEdgesPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		return payload
	}

	first := hit()
	if !first.Truncated {
		t.Fatalf("expected truncated=true on first response, got false")
	}
	if len(first.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(first.Items))
	}
	if calls != 1 {
		t.Fatalf("expected one store call, got %d", calls)
	}

	second := hit()
	if calls != 1 {
		t.Fatalf("expected cache reuse (no extra store call), got %d", calls)
	}
	if !second.Truncated || len(second.Items) != 3 {
		t.Fatalf("unexpected cached payload: %#v", second)
	}

	// Defensive copy check: mutating the second response must not poison the cache.
	second.Items[0].ToNodeID = "!mutated"
	third := hit()
	if third.Items[0].ToNodeID == "!mutated" {
		t.Fatalf("cached payload was mutated by caller")
	}

	// Cache expires: one more store call expected.
	srv.now = func() time.Time { return now.Add(2 * time.Minute) }
	fourth := hit()
	if calls != 2 {
		t.Fatalf("expected cache expiry to trigger a new store call, got %d calls", calls)
	}
	if !fourth.Truncated {
		t.Fatalf("expected truncated=true after expiry, got false")
	}
}

func TestLogEventsHandlerForwardsNodeIDFilter(t *testing.T) {
	store := &testkit.FakeStore{
		ListLogEventsFn: func(_ context.Context, q domain.LogEventQuery) ([]domain.LogEventView, error) {
			if q.Limit != 25 || q.BeforeID != 44 || q.Channel != "LongFast" || q.NodeID != "!49b5976c" {
				t.Fatalf("unexpected log query: %+v", q)
			}
			if q.HopsMin == nil || *q.HopsMin != 0 || q.HopsMax == nil || *q.HopsMax != 3 {
				t.Fatalf("unexpected hop filters: %+v", q)
			}
			if len(q.EventKinds) != 1 || q.EventKinds[0] != domain.LogEventKindTelemetryValue {
				t.Fatalf("unexpected event kinds: %+v", q.EventKinds)
			}

			return []domain.LogEventView{{
				ID:             43,
				ObservedAt:     time.Unix(1772296589, 0).UTC(),
				NodeID:         q.NodeID,
				EventKindValue: domain.LogEventKindTelemetryValue,
				EventKindTitle: domain.LogEventKindTitle(domain.LogEventKindTelemetryValue),
			}}, nil
		},
	}

	srv := New(Config{Web: config.WebConfig{Log: config.LogConfig{PageSizeDefault: 100}}}, store, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/events?limit=25&before=44&channel=LongFast&node_id=!49b5976c&event_kind=4&hops_min=0&hops_max=3", nil)
	rec := httptest.NewRecorder()

	srv.logEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var items []domain.LogEventView
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 || items[0].NodeID != "!49b5976c" {
		t.Fatalf("unexpected response payload: %#v", items)
	}
}

func TestChatMessagesHandlerAppliesHistoryWindow(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		ListChatEventsFn: func(_ context.Context, q repo.ChatEventQuery) ([]domain.ChatEvent, error) {
			if q.Channel != "LongFast" || q.Limit != 25 || q.BeforeID != 44 {
				t.Fatalf("unexpected chat query: %+v", q)
			}
			wantCutoff := now.Add(-48 * time.Hour)
			if !q.ObservedSinceAt.Equal(wantCutoff) {
				t.Fatalf("expected observed cutoff %s, got %s", wantCutoff, q.ObservedSinceAt)
			}

			return []domain.ChatEvent{{
				ID:         43,
				EventType:  domain.ChatEventMessage,
				ObservedAt: now,
			}}, nil
		},
	}
	srv := New(Config{
		Web: config.WebConfig{
			Chat: config.ChatConfig{
				DefaultChannel:     "LongFast",
				ShowRecentMessages: 50,
				HistoryWindow:      48 * time.Hour,
			},
		},
	}, store, nil, nil, nil, nil)
	srv.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/messages?limit=25&before=44", nil)
	rec := httptest.NewRecorder()

	srv.chatMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestMetaHandlerReturnsRelevance(t *testing.T) {
	srv := New(Config{
		Web: config.WebConfig{
			Relevance: config.RelevanceConfig{
				TelemetryMaxAge:        24 * time.Hour,
				TopologyEvidenceMaxAge: 72 * time.Hour,
				MapPositionMaxAge:      14 * 24 * time.Hour,
			},
		},
	}, &testkit.FakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	rec := httptest.NewRecorder()

	srv.meta(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload metaPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Relevance.TelemetryMaxAge != "24h0m0s" ||
		payload.Relevance.TopologyEvidenceMaxAge != "72h0m0s" ||
		payload.Relevance.MapPositionMaxAge != "336h0m0s" {
		t.Fatalf("unexpected relevance payload: %+v", payload.Relevance)
	}
}

func TestMetaHandlerReturnsInfoAvailability(t *testing.T) {
	srv := New(Config{
		Info: &siteinfo.Info{
			SourceHash: "abc123",
		},
	}, &testkit.FakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	rec := httptest.NewRecorder()

	srv.meta(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload metaPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.InfoAvailable || payload.InfoSourceHash != "abc123" {
		t.Fatalf("unexpected info payload: available=%v hash=%q", payload.InfoAvailable, payload.InfoSourceHash)
	}
}

func TestInfoHandlerReturnsHTMLByDefault(t *testing.T) {
	srv := New(Config{
		Info: &siteinfo.Info{
			Markdown:   "# Hello",
			HTML:       "<h1>Hello</h1>\n",
			SourceHash: "abc123",
		},
	}, &testkit.FakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rec := httptest.NewRecorder()

	srv.info(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload infoPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Format != "html" || payload.SourceHash != "abc123" || payload.Content != "<h1>Hello</h1>\n" {
		t.Fatalf("unexpected info payload: %+v", payload)
	}
}

func TestInfoHandlerReturnsMarkdown(t *testing.T) {
	srv := New(Config{
		Info: &siteinfo.Info{
			Markdown:   "# Hello",
			HTML:       "<h1>Hello</h1>\n",
			SourceHash: "abc123",
		},
	}, &testkit.FakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info?format=markdown", nil)
	rec := httptest.NewRecorder()

	srv.info(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload infoPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Format != "markdown" || payload.Content != "# Hello" {
		t.Fatalf("unexpected info payload: %+v", payload)
	}
}

func TestChatMessagesHandlerExposesHopMetadata(t *testing.T) {
	hopStart := uint32(7)
	hopLimit := uint32(4)
	store := &testkit.FakeStore{
		ListChatEventsFn: func(_ context.Context, _ repo.ChatEventQuery) ([]domain.ChatEvent, error) {
			return []domain.ChatEvent{{
				ID:          1,
				EventType:   domain.ChatEventMessage,
				ObservedAt:  time.Unix(1772296589, 0).UTC(),
				MessageText: "relayed",
				HopStart:    &hopStart,
				HopLimit:    &hopLimit,
			}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Chat: config.ChatConfig{DefaultChannel: "LongFast"}}}, store, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/messages", nil)
	rec := httptest.NewRecorder()

	srv.chatMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	body := rec.Body.Bytes()
	if len(body) == 0 || body[0] != '[' {
		idx := bytes.IndexByte(body, '[')
		if idx < 0 {
			t.Fatalf("expected array in response, got: %s", body)
		}
		body = body[idx:]
	}
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode array response: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 chat event, got %d", len(arr))
	}
	if got, ok := arr[0]["hop_start"].(float64); !ok || got != 7 {
		t.Fatalf("expected hop_start=7, got %#v", arr[0]["hop_start"])
	}
	if got, ok := arr[0]["hop_limit"].(float64); !ok || got != 4 {
		t.Fatalf("expected hop_limit=4, got %#v", arr[0]["hop_limit"])
	}
}

func TestLogEventsHandlerExposesHopMetadata(t *testing.T) {
	hopStart := uint32(5)
	hopLimit := uint32(2)
	store := &testkit.FakeStore{
		ListLogEventsFn: func(_ context.Context, _ domain.LogEventQuery) ([]domain.LogEventView, error) {
			return []domain.LogEventView{{
				ID:             1,
				ObservedAt:     time.Unix(1772296589, 0).UTC(),
				EventKindValue: domain.LogEventKindTelemetryValue,
				EventKindTitle: domain.LogEventKindTitle(domain.LogEventKindTelemetryValue),
				Encrypted:      false,
				HopStart:       &hopStart,
				HopLimit:       &hopLimit,
			}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Log: config.LogConfig{PageSizeDefault: 100}}}, store, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/events", nil)
	rec := httptest.NewRecorder()

	srv.logEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	body := rec.Body.Bytes()
	if body[0] != '[' {
		idx := bytes.IndexByte(body, '[')
		if idx < 0 {
			t.Fatalf("expected array in response, got: %s", body)
		}
		body = body[idx:]
	}
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode array response: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 log event, got %d", len(arr))
	}
	if got, ok := arr[0]["hop_start"].(float64); !ok || got != 5 {
		t.Fatalf("expected hop_start=5, got %#v", arr[0]["hop_start"])
	}
	if got, ok := arr[0]["hop_limit"].(float64); !ok || got != 2 {
		t.Fatalf("expected hop_limit=2, got %#v", arr[0]["hop_limit"])
	}
}

func TestInfoHandlerReturnsNotConfigured(t *testing.T) {
	srv := New(Config{}, &testkit.FakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rec := httptest.NewRecorder()

	srv.info(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "info_not_configured" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestInfoHandlerRejectsInvalidFormat(t *testing.T) {
	srv := New(Config{
		Info: &siteinfo.Info{
			Markdown:   "# Hello",
			HTML:       "<h1>Hello</h1>\n",
			SourceHash: "abc123",
		},
	}, &testkit.FakeStore{}, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info?format=json", nil)
	rec := httptest.NewRecorder()

	srv.info(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "invalid_format" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestSnapshotHandlersPassRelevanceCutoffs(t *testing.T) {
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		GetMapNodesFn: func(_ context.Context, q repo.MapNodeQuery) ([]repo.MapNode, error) {
			if !q.PositionObservedSince.Equal(now.Add(-14*24*time.Hour)) || !q.TelemetryObservedSince.Equal(now.Add(-24*time.Hour)) {
				t.Fatalf("unexpected map query: %+v", q)
			}

			return []repo.MapNode{}, nil
		},
		ListNodesFn: func(_ context.Context, q repo.NodeListQuery) ([]repo.NodeSummary, error) {
			if !q.PositionObservedSince.Equal(now.Add(-14 * 24 * time.Hour)) {
				t.Fatalf("unexpected nodes query: %+v", q)
			}

			return []repo.NodeSummary{}, nil
		},
		GetNodeDetailsFn: func(_ context.Context, q repo.NodeDetailsQuery) (repo.NodeDetails, error) {
			if q.NodeID != "!49b5976c" ||
				!q.PositionObservedSince.Equal(now.Add(-14*24*time.Hour)) ||
				!q.TelemetryObservedSince.Equal(now.Add(-24*time.Hour)) ||
				!q.TopologyUpdatedSince.Equal(now.Add(-72*time.Hour)) {
				t.Fatalf("unexpected node details query: %+v", q)
			}

			return repo.NodeDetails{Node: domain.Node{NodeID: q.NodeID, LastSeenAnyEventAt: now}}, nil
		},
	}
	srv := New(Config{
		Web: config.WebConfig{
			Relevance: config.RelevanceConfig{
				TelemetryMaxAge:        24 * time.Hour,
				TopologyEvidenceMaxAge: 72 * time.Hour,
				MapPositionMaxAge:      14 * 24 * time.Hour,
			},
		},
	}, store, nil, nil, nil, nil)
	srv.now = func() time.Time { return now }

	for _, tc := range []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "map", path: "/api/v1/map/nodes", call: srv.mapNodes},
		{name: "nodes", path: "/api/v1/nodes", call: srv.nodes},
		{name: "details", path: "/api/v1/nodes/%2149b5976c", call: srv.nodeByID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			tc.call(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("unexpected status: %d", rec.Code)
			}
		})
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

func TestNodeByIDReturnsNodeDetails(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	store := &testkit.FakeStore{
		GetNodeDetailsFn: func(_ context.Context, q repo.NodeDetailsQuery) (repo.NodeDetails, error) {
			if q.NodeID != "!49b5976c" {
				t.Fatalf("unexpected node id: %q", q.NodeID)
			}

			return repo.NodeDetails{
				Node: domain.Node{
					NodeID:             q.NodeID,
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
				PreviousNames: []repo.NodeNameHistory{{
					PreviousLongName:  "Old Alpha",
					PreviousShortName: "OA",
					NewLongName:       "Alpha",
					NewShortName:      "ALP",
					ChangedAt:         now,
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
	if len(item.PreviousNames) != 1 || item.PreviousNames[0].PreviousLongName != "Old Alpha" {
		t.Fatalf("unexpected previous names payload: %#v", item.PreviousNames)
	}
}

// firmwareSoftwareConfig is the test-side default StatsSoftwareConfig.
// Tests that don't need to vary a field use this verbatim; tests that
// do override individual fields on the returned struct. The TTLs mirror
// production defaults — their effect on the cache is tested directly in
// cache_test.go.
var firmwareSoftwareConfig = config.StatsSoftwareConfig{
	SnapshotCacheTTL: time.Hour,
	HistoryCacheTTL:  24 * time.Hour,
	HistoryWeeks:     54,
	TopVersions:      15,
	MapReportMaxAge:  14 * 24 * time.Hour,
}

func TestFirmwareSnapshotHandler_ReturnsVersionsAndTotal(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.FirmwareVersionCount, error) {
			return []repo.FirmwareVersionCount{
				{Version: "2.7.15", Count: 3, LastSeenAt: now.Add(-1 * time.Hour)},
				{Version: "2.7.10", Count: 2, LastSeenAt: now.Add(-2 * time.Hour)},
				{Version: "2.6.5", Count: 1, LastSeenAt: now.Add(-24 * time.Hour)},
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware", nil)
	rec := httptest.NewRecorder()
	srv.firmwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload firmwareSnapshotPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.TotalNodesWithVersion != 6 {
		t.Errorf("expected total_nodes_with_version 6, got %d", payload.TotalNodesWithVersion)
	}
	if len(payload.Versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(payload.Versions))
	}
	if payload.Versions[0].Version != "2.7.15" || payload.Versions[0].Count != 3 {
		t.Errorf("expected first version 2.7.15 count 3, got %+v", payload.Versions[0])
	}
	if !payload.GeneratedAt.Equal(now) {
		t.Errorf("expected generated_at %s, got %s", now, payload.GeneratedAt)
	}
	// The handler echoes the resolved TTL so the client can poll on
	// the operator's cadence rather than a hardcoded 1h.
	if payload.CacheTtlSeconds != 3600 {
		t.Errorf("expected cache_ttl_seconds 3600 (1h default), got %d", payload.CacheTtlSeconds)
	}
}

func TestFirmwareSnapshotHandler_ReusesCacheUntilTTL(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.FirmwareVersionCount, error) {
			calls++

			return []repo.FirmwareVersionCount{{Version: "2.7.15", Count: calls, LastSeenAt: now}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	hit := func() firmwareSnapshotPayload {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware", nil)
		rec := httptest.NewRecorder()
		srv.firmwareSnapshot(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload firmwareSnapshotPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		return payload
	}

	first := hit()
	if first.Versions[0].Count != 1 {
		t.Fatalf("expected count 1, got %d", first.Versions[0].Count)
	}

	second := hit()
	if calls != 1 {
		t.Fatalf("expected cache reuse (one store call), got %d", calls)
	}
	if second.Versions[0].Count != 1 {
		t.Fatalf("expected cached count 1, got %d", second.Versions[0].Count)
	}

	// Advance past the snapshot TTL — next request must recompute.
	advanced := now.Add(2 * time.Hour)
	srv.now = func() time.Time { return advanced }
	srv.firmwareCache.now = func() time.Time { return advanced }
	third := hit()
	if calls != 2 {
		t.Fatalf("expected cache expiry to trigger store call, got %d calls", calls)
	}
	if third.Versions[0].Count != 2 {
		t.Fatalf("expected fresh count 2 after expiry, got %d", third.Versions[0].Count)
	}
}

func TestFirmwareSnapshotHandler_InvalidateBustsCache(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.FirmwareVersionCount, error) {
			calls++

			return []repo.FirmwareVersionCount{{Version: "2.7.15", Count: calls, LastSeenAt: now}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	hit := func() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware", nil)
		rec := httptest.NewRecorder()
		srv.firmwareSnapshot(rec, req)
	}

	hit()
	hit()
	if calls != 1 {
		t.Fatalf("expected cache reuse before invalidation, got %d calls", calls)
	}

	// Scheduled-job callback hits the cache.
	srv.InvalidateFirmwareCaches()

	hit()
	if calls != 2 {
		t.Fatalf("expected invalidation to trigger a store call, got %d calls", calls)
	}
}

// TestFirmwareHistoryHandler_RespectsConfig pins the window math
// from a single source of truth: config. The endpoint no longer
// accepts ?weeks or ?top overrides (see TestFirmwareHistoryHandler_
// IgnoresQueryParams), so the values seen by the store are exactly
// firmwareSoftwareConfig.{HistoryWeeks,TopVersions}. For now = 2026-
// 06-21 (Sunday) the current week is 2026-06-15 (Mon), so weeks=54
// → since = 2026-06-15 - 53*7d = 2025-07-21 (Mon).
func TestFirmwareHistoryHandler_RespectsConfig(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		FirmwareVersionHistoryFn: func(_ context.Context, since time.Time, topN, totalWeeks int) (repo.FirmwareHistoryResult, error) {
			wantSince := time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC)
			if !since.Equal(wantSince) {
				t.Errorf("unexpected since: got %s, want %s", since, wantSince)
			}
			if topN != 15 {
				t.Errorf("expected top=15 (config default), got %d", topN)
			}
			if totalWeeks != 54 {
				t.Errorf("expected weeks=54 (config default), got %d", totalWeeks)
			}

			return repo.FirmwareHistoryResult{
				Weeks: totalWeeks,
				TopN:  topN,
				Versions: []string{
					"2.7.15", "2.7.10", "2.6.5", "2.5.0", "2.4.0",
				},
				VersionsByWeek: [][]int{
					make([]int, totalWeeks),
					make([]int, totalWeeks),
					make([]int, totalWeeks),
					make([]int, totalWeeks),
					make([]int, totalWeeks),
				},
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware/history", nil)
	rec := httptest.NewRecorder()
	srv.firmwareHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload firmwareHistoryPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Weeks != 54 || payload.Top != 15 {
		t.Fatalf("unexpected payload metadata: %+v", payload)
	}
	if len(payload.Versions) != 5 {
		t.Fatalf("expected 5 versions, got %d", len(payload.Versions))
	}
	if len(payload.VersionsByWeek) != 5 || len(payload.VersionsByWeek[0]) != 54 {
		t.Fatalf("unexpected versions_by_week shape: %d series x %d weeks",
			len(payload.VersionsByWeek), len(payload.VersionsByWeek[0]))
	}
	// The handler echoes the resolved TTL so the client can poll on
	// the operator's cadence rather than a hardcoded 24h.
	if payload.CacheTtlSeconds != 86400 {
		t.Errorf("expected cache_ttl_seconds 86400 (24h default), got %d", payload.CacheTtlSeconds)
	}
}

func TestFirmwareHistoryHandler_HistoryCacheIsSeparateFromSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	snapCalls, histCalls := 0, 0
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.FirmwareVersionCount, error) {
			snapCalls++

			return []repo.FirmwareVersionCount{{Version: "2.7.15", Count: 1, LastSeenAt: now}}, nil
		},
		FirmwareVersionHistoryFn: func(_ context.Context, _ time.Time, _, _ int) (repo.FirmwareHistoryResult, error) {
			histCalls++

			return repo.FirmwareHistoryResult{
				Weeks:          1,
				TopN:           1,
				Versions:       []string{"2.7.15"},
				VersionsByWeek: [][]int{{1}},
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	hit := func(path string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		switch path {
		case "/api/v1/stats/firmware":
			srv.firmwareSnapshot(rec, req)
		case "/api/v1/stats/firmware/history":
			srv.firmwareHistory(rec, req)
		}
	}

	hit("/api/v1/stats/firmware")
	hit("/api/v1/stats/firmware")
	hit("/api/v1/stats/firmware/history")
	hit("/api/v1/stats/firmware/history")

	if snapCalls != 1 {
		t.Errorf("expected snapshot cache reuse (1 call), got %d", snapCalls)
	}
	if histCalls != 1 {
		t.Errorf("expected history cache reuse (1 call), got %d", histCalls)
	}
}

// TestFirmwareHistoryHandler_IncludesCurrentWeek pins the window math
// in firmwareHistory: the current week must land at the LAST column,
// not fall off the end. The previous formulation computed
// `since = now - 7*weeks` (anchored to a mid-week day), which pushed
// the current week past the allocated `weeks` columns AND pulled in
// an extra older week. The fix anchors `since` to the Monday of the
// current week minus (weeks-1)*7 days.
//
// The test exercises multiple `now` positions (Sun, Mon, Wed, Sat)
// because the bug only manifested when `now` was mid-week.
func TestFirmwareHistoryHandler_IncludesCurrentWeek(t *testing.T) {
	cases := []struct {
		name  string
		now   time.Time
		weeks int
	}{
		// 2026-06-15 is a Monday.
		{"Monday of current week, 4 weeks", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), 4},
		{"Wednesday of current week, 4 weeks", time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC), 4},
		{"Sunday of current week, 4 weeks", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 4},
		{"Saturday of current week, 4 weeks", time.Date(2026, 6, 20, 23, 59, 59, 0, time.UTC), 4},
		{"Sunday, 1 week window", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 1},
		{"Sunday, 54 weeks window", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 54},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := tc.now
			cfg := firmwareSoftwareConfig
			cfg.HistoryWeeks = tc.weeks
			store := &testkit.FakeStore{
				FirmwareVersionHistoryFn: func(_ context.Context, since time.Time, _, totalWeeks int) (repo.FirmwareHistoryResult, error) {
					if totalWeeks != tc.weeks {
						t.Errorf("expected weeks=%d, got %d", tc.weeks, totalWeeks)
					}
					// The Monday of `now`'s week, regardless of where in
					// the week `now` sits.
					wantCurrentWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
					wantSince := wantCurrentWeek.AddDate(0, 0, -7*(tc.weeks-1))
					if !since.Equal(wantSince) {
						t.Errorf("since=%s, want %s (currentWeek - %d weeks)", since, wantSince, tc.weeks-1)
					}

					versionsByWeek := make([]int, tc.weeks)
					for i := range versionsByWeek {
						versionsByWeek[i] = 1
					}

					return repo.FirmwareHistoryResult{
						Weeks:          tc.weeks,
						TopN:           1,
						Versions:       []string{"2.7.15"},
						VersionsByWeek: [][]int{versionsByWeek},
					}, nil
				},
			}
			srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: cfg}}},
				store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
			srv.now = func() time.Time { return now }
			srv.firmwareCache.now = func() time.Time { return now }
			srv.firmwareCache.now = func() time.Time { return now }

			req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware/history", nil)
			rec := httptest.NewRecorder()
			srv.firmwareHistory(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
			}
			var payload firmwareHistoryPayload
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(payload.VersionsByWeek) != 1 || len(payload.VersionsByWeek[0]) != tc.weeks {
				t.Fatalf("unexpected payload shape: %d series x %d weeks (want 1 x %d)",
					len(payload.VersionsByWeek), len(payload.VersionsByWeek[0]), tc.weeks)
			}
		})
	}
}

// TestFirmwareHistoryHandler_IgnoresQueryParams pins the
// config-as-source-of-truth decision: even if a client sends ?weeks
// or ?top, the handler must use the config values. This closes the
// unbounded-allocation DoS vector (?weeks=100000000) and keeps the
// response cacheable behind a single key.
func TestFirmwareHistoryHandler_IgnoresQueryParams(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		FirmwareVersionHistoryFn: func(_ context.Context, _ time.Time, topN, totalWeeks int) (repo.FirmwareHistoryResult, error) {
			if totalWeeks != 54 {
				t.Errorf("query-param weeks ignored: expected config weeks=54, got %d", totalWeeks)
			}
			if topN != 15 {
				t.Errorf("query-param top ignored: expected config top=15, got %d", topN)
			}

			return repo.FirmwareHistoryResult{
				Weeks:          totalWeeks,
				TopN:           topN,
				Versions:       []string{"2.7.15"},
				VersionsByWeek: [][]int{make([]int, totalWeeks)},
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	// Adversarial query params: massive weeks + top, plus a tiny top
	// (which would change topN). The handler must use the config
	// values (54 / 15) for both.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware/history?weeks=100000000&top=3", nil)
	rec := httptest.NewRecorder()
	srv.firmwareHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestFirmwareHistoryHandler_EchoesWeekStarts pins the new
// week_starts contract: the handler must surface the store's
// resolved week starts (oldest-first) in the payload so the
// front-end chart labels buckets with the server's Monday
// math instead of re-deriving from the browser's clock. The
// response is verified after a JSON round-trip so the wire
// format (RFC3339) is exercised end-to-end.
func TestFirmwareHistoryHandler_EchoesWeekStarts(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC) // Sunday
	// currentWeek = 2026-06-15 (Monday). For weeks=4 the slice is
	// [2025-12-22, 2025-12-29, 2026-01-05, 2026-01-12] — wait, that's
	// a typo. Correct math: currentWeek - 3*7d = 2026-06-15 - 21d =
	// 2026-05-25, then [25, +7, +14, +21] = [05-25, 06-01, 06-08, 06-15].
	wantStarts := []time.Time{
		time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
	}
	store := &testkit.FakeStore{
		FirmwareVersionHistoryFn: func(_ context.Context, _ time.Time, _, totalWeeks int) (repo.FirmwareHistoryResult, error) {
			return repo.FirmwareHistoryResult{
				Weeks:          totalWeeks,
				TopN:           1,
				Versions:       []string{"2.7.15"},
				VersionsByWeek: [][]int{make([]int, totalWeeks)},
				WeekStarts:     wantStarts,
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware/history", nil)
	rec := httptest.NewRecorder()
	srv.firmwareHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	// Round-trip through JSON to pin the wire format.
	var raw struct {
		WeekStarts []string `json:"week_starts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode week_starts: %v", err)
	}
	if len(raw.WeekStarts) != len(wantStarts) {
		t.Fatalf("expected %d week_starts, got %d", len(wantStarts), len(raw.WeekStarts))
	}
	for i, want := range wantStarts {
		got, err := time.Parse(time.RFC3339Nano, raw.WeekStarts[i])
		if err != nil {
			t.Fatalf("week_starts[%d]=%q is not RFC3339: %v", i, raw.WeekStarts[i], err)
		}
		if !got.Equal(want) {
			t.Errorf("week_starts[%d]=%s, want %s", i, got, want)
		}
	}
}

// TestFirmwareHistoryHandler_FallsBackToDerivedWeekStarts pins the
// defensive path: if a third-party or rolled-back ReadStore
// implementation doesn't populate WeekStarts, the handler must
// reconstruct it from `since` (Monday of the current week minus
// (weeks-1)*7d) so the chart still labels buckets correctly.
func TestFirmwareHistoryHandler_FallsBackToDerivedWeekStarts(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC) // Sunday → currentWeek 2026-06-15
	store := &testkit.FakeStore{
		FirmwareVersionHistoryFn: func(_ context.Context, _ time.Time, _, totalWeeks int) (repo.FirmwareHistoryResult, error) {
			return repo.FirmwareHistoryResult{
				Weeks:          totalWeeks,
				TopN:           1,
				Versions:       []string{"2.7.15"},
				VersionsByWeek: [][]int{make([]int, totalWeeks)},
				// WeekStarts deliberately omitted.
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: firmwareSoftwareConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware/history", nil)
	rec := httptest.NewRecorder()
	srv.firmwareHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload firmwareHistoryPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.WeekStarts) != payload.Weeks {
		t.Fatalf("expected fallback to populate %d week_starts, got %d", payload.Weeks, len(payload.WeekStarts))
	}
	// Last entry must be the current Monday — that's the value the
	// chart most often displays and the value most affected by the
	// original bug (browser vs server week boundary).
	last := payload.WeekStarts[len(payload.WeekStarts)-1]
	wantLast := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if !last.Equal(wantLast) {
		t.Errorf("fallback last week_start=%s, want %s", last, wantLast)
	}
}

// TestFirmwareSnapshotHandler_EchoesResolvedTTL pins that the
// snapshot endpoint echoes the operator's resolved TTL (not a
// hardcoded default) so the UI can poll on the operator's cadence.
// Regression for the CodeX review of PR #111.
func TestFirmwareSnapshotHandler_EchoesResolvedTTL(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	cfg := firmwareSoftwareConfig
	cfg.SnapshotCacheTTL = 5 * time.Minute // short, to prove the echo isn't just the default
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.FirmwareVersionCount, error) {
			return []repo.FirmwareVersionCount{{Version: "2.7.15", Count: 1, LastSeenAt: now}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware", nil)
	rec := httptest.NewRecorder()
	srv.firmwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload firmwareSnapshotPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CacheTtlSeconds != 300 {
		t.Errorf("expected cache_ttl_seconds 300 (5m resolved), got %d", payload.CacheTtlSeconds)
	}
}

// TestFirmwareHistoryHandler_EchoesResolvedTTL is the history
// counterpart to TestFirmwareSnapshotHandler_EchoesResolvedTTL.
// Together they prove the client poll cadence tracks the operator's
// configured TTL on both firmware endpoints.
func TestFirmwareHistoryHandler_EchoesResolvedTTL(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	cfg := firmwareSoftwareConfig
	cfg.HistoryCacheTTL = 90 * time.Minute
	store := &testkit.FakeStore{
		FirmwareVersionHistoryFn: func(_ context.Context, _ time.Time, _, totalWeeks int) (repo.FirmwareHistoryResult, error) {
			return repo.FirmwareHistoryResult{
				Weeks:          totalWeeks,
				TopN:           1,
				Versions:       []string{"2.7.15"},
				VersionsByWeek: [][]int{make([]int, totalWeeks)},
				WeekStarts:     make([]time.Time, totalWeeks),
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware/history", nil)
	rec := httptest.NewRecorder()
	srv.firmwareHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload firmwareHistoryPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CacheTtlSeconds != 5400 {
		t.Errorf("expected cache_ttl_seconds 5400 (90m resolved), got %d", payload.CacheTtlSeconds)
	}
}

// TestFirmwareSnapshotHandler_PassesMapReportMaxAgeToStore verifies that
// the snapshot handler threads web.stats.software.map_report_max_age
// straight through to the store as the staleness cutoff for the live
// snapshot bar chart.
func TestFirmwareSnapshotHandler_PassesMapReportMaxAgeToStore(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	var observed time.Duration
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, maxAge time.Duration) ([]repo.FirmwareVersionCount, error) {
			observed = maxAge

			return []repo.FirmwareVersionCount{{Version: "2.7.15", Count: 1, LastSeenAt: now}}, nil
		},
	}
	cfg := firmwareSoftwareConfig
	cfg.MapReportMaxAge = 7 * 24 * time.Hour
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware", nil)
	rec := httptest.NewRecorder()
	srv.firmwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if observed != 7*24*time.Hour {
		t.Errorf("expected store to receive 7d maxAge, got %s", observed)
	}
}

// TestFirmwareSnapshotHandler_DefaultsMapReportMaxAgeWhenZero pins the
// fallback: if an operator sets map_report_max_age to 0 (or omits it
// entirely and config loading produces a zero value), the handler must
// default to 14d so the firmware stats don't suddenly exclude every
// node.
func TestFirmwareSnapshotHandler_DefaultsMapReportMaxAgeWhenZero(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	var observed time.Duration
	store := &testkit.FakeStore{
		FirmwareVersionSnapshotFn: func(_ context.Context, maxAge time.Duration) ([]repo.FirmwareVersionCount, error) {
			observed = maxAge

			return []repo.FirmwareVersionCount{{Version: "2.7.15", Count: 1, LastSeenAt: now}}, nil
		},
	}
	cfg := firmwareSoftwareConfig
	cfg.MapReportMaxAge = 0
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Software: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.firmwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/firmware", nil)
	rec := httptest.NewRecorder()
	srv.firmwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if observed != 14*24*time.Hour {
		t.Errorf("expected store to receive 14d default, got %s", observed)
	}
}

// hardwareStatsConfig is the Hardware analogue of firmwareSoftwareConfig:
// production-default-shaped config so the cache/window tests observe real
// values — their effect on the cache is tested directly in cache_test.go.
// Note MaxAge gates last_seen_any_event_at (not last_map_report_at).
var hardwareStatsConfig = config.StatsHardwareConfig{
	SnapshotCacheTTL: time.Hour,
	HistoryCacheTTL:  24 * time.Hour,
	HistoryWeeks:     54,
	TopModels:        15,
	MaxAge:           14 * 24 * time.Hour,
}

func TestHardwareSnapshotHandler_ReturnsModelsAndTotal(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		HardwareModelSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.HardwareModelCount, error) {
			return []repo.HardwareModelCount{
				{Model: "heltec-v3", Count: 3, LastSeenAt: now.Add(-1 * time.Hour)},
				{Model: "tbeam", Count: 2, LastSeenAt: now.Add(-2 * time.Hour)},
				{Model: "rak4631", Count: 1, LastSeenAt: now.Add(-24 * time.Hour)},
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: hardwareStatsConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware", nil)
	rec := httptest.NewRecorder()
	srv.hardwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload hardwareSnapshotPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.TotalNodesWithModel != 6 {
		t.Errorf("expected total_nodes_with_model 6, got %d", payload.TotalNodesWithModel)
	}
	if len(payload.Models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(payload.Models))
	}
	if payload.Models[0].Model != "heltec-v3" || payload.Models[0].Count != 3 {
		t.Errorf("expected first model heltec-v3 count 3, got %+v", payload.Models[0])
	}
	if !payload.GeneratedAt.Equal(now) {
		t.Errorf("expected generated_at %s, got %s", now, payload.GeneratedAt)
	}
	// The handler echoes the resolved TTL so the client can poll on the
	// operator's cadence rather than a hardcoded 1h.
	if payload.CacheTtlSeconds != 3600 {
		t.Errorf("expected cache_ttl_seconds 3600 (1h default), got %d", payload.CacheTtlSeconds)
	}
}

func TestHardwareSnapshotHandler_ReusesCacheUntilTTL(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	store := &testkit.FakeStore{
		HardwareModelSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.HardwareModelCount, error) {
			calls++

			return []repo.HardwareModelCount{{Model: "heltec-v3", Count: calls, LastSeenAt: now}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: hardwareStatsConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	hit := func() hardwareSnapshotPayload {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware", nil)
		rec := httptest.NewRecorder()
		srv.hardwareSnapshot(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload hardwareSnapshotPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		return payload
	}

	first := hit()
	if first.Models[0].Count != 1 {
		t.Fatalf("expected count 1, got %d", first.Models[0].Count)
	}

	second := hit()
	if calls != 1 {
		t.Fatalf("expected cache reuse (one store call), got %d", calls)
	}
	if second.Models[0].Count != 1 {
		t.Fatalf("expected cached count 1, got %d", second.Models[0].Count)
	}

	// Advance past the snapshot TTL — next request must recompute.
	advanced := now.Add(2 * time.Hour)
	srv.now = func() time.Time { return advanced }
	srv.hardwareCache.now = func() time.Time { return advanced }
	third := hit()
	if calls != 2 {
		t.Fatalf("expected cache expiry to trigger store call, got %d calls", calls)
	}
	if third.Models[0].Count != 2 {
		t.Fatalf("expected fresh count 2 after expiry, got %d", third.Models[0].Count)
	}
}

func TestHardwareSnapshotHandler_InvalidateBustsCache(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	store := &testkit.FakeStore{
		HardwareModelSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.HardwareModelCount, error) {
			calls++

			return []repo.HardwareModelCount{{Model: "heltec-v3", Count: calls, LastSeenAt: now}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: hardwareStatsConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	hit := func() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware", nil)
		rec := httptest.NewRecorder()
		srv.hardwareSnapshot(rec, req)
	}

	hit()
	hit()
	if calls != 1 {
		t.Fatalf("expected cache reuse before invalidation, got %d calls", calls)
	}

	// Scheduled-job callback hits the cache.
	srv.InvalidateHardwareCaches()

	hit()
	if calls != 2 {
		t.Fatalf("expected invalidation to trigger a store call, got %d calls", calls)
	}
}

// TestHardwareHistoryHandler_RespectsConfig pins the window math from a single
// source of truth: config. The endpoint does not accept ?weeks or ?top, so the
// values seen by the store are exactly hardwareStatsConfig.{HistoryWeeks,
// TopModels}. For now = 2026-06-21 (Sunday) the current week is 2026-06-15
// (Mon), so weeks=54 → since = 2026-06-15 - 53*7d = 2025-06-09 (Mon).
func TestHardwareHistoryHandler_RespectsConfig(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := &testkit.FakeStore{
		HardwareModelHistoryFn: func(_ context.Context, since time.Time, topN, totalWeeks int) (repo.HardwareHistoryResult, error) {
			wantSince := time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC)
			if !since.Equal(wantSince) {
				t.Errorf("unexpected since: got %s, want %s", since, wantSince)
			}
			if topN != 15 {
				t.Errorf("expected top=15 (config default), got %d", topN)
			}
			if totalWeeks != 54 {
				t.Errorf("expected weeks=54 (config default), got %d", totalWeeks)
			}

			return repo.HardwareHistoryResult{
				Weeks: totalWeeks,
				TopN:  topN,
				Models: []string{
					"heltec-v3", "tbeam", "rak4631", "t-echo", "station-g1",
				},
				ModelsByWeek: [][]int{
					make([]int, totalWeeks),
					make([]int, totalWeeks),
					make([]int, totalWeeks),
					make([]int, totalWeeks),
					make([]int, totalWeeks),
				},
			}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: hardwareStatsConfig}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware/history", nil)
	rec := httptest.NewRecorder()
	srv.hardwareHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload hardwareHistoryPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Weeks != 54 || payload.Top != 15 {
		t.Fatalf("unexpected payload metadata: %+v", payload)
	}
	if len(payload.Models) != 5 {
		t.Fatalf("expected 5 models, got %d", len(payload.Models))
	}
	if len(payload.ModelsByWeek) != 5 || len(payload.ModelsByWeek[0]) != 54 {
		t.Fatalf("unexpected models_by_week shape: %d series x %d weeks",
			len(payload.ModelsByWeek), len(payload.ModelsByWeek[0]))
	}
	// The handler echoes the resolved TTL so the client can poll on the
	// operator's cadence rather than a hardcoded 24h.
	if payload.CacheTtlSeconds != 86400 {
		t.Errorf("expected cache_ttl_seconds 86400 (24h default), got %d", payload.CacheTtlSeconds)
	}
}

// TestHardwareHistoryHandler_IncludesCurrentWeek pins the window math in
// hardwareHistory: the current week must land at the LAST column, not fall off
// the end. The fix (shared with firmware) anchors since to the Monday of the
// current week minus (weeks-1)*7 days. The bug only manifested mid-week, so the
// test exercises several `now` positions.
func TestHardwareHistoryHandler_IncludesCurrentWeek(t *testing.T) {
	cases := []struct {
		name  string
		now   time.Time
		weeks int
	}{
		// 2026-06-15 is a Monday.
		{"Monday of current week, 4 weeks", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), 4},
		{"Wednesday of current week, 4 weeks", time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC), 4},
		{"Sunday of current week, 4 weeks", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 4},
		{"Saturday of current week, 4 weeks", time.Date(2026, 6, 20, 23, 59, 59, 0, time.UTC), 4},
		{"Sunday, 1 week window", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 1},
		{"Sunday, 54 weeks window", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 54},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := tc.now
			cfg := hardwareStatsConfig
			cfg.HistoryWeeks = tc.weeks
			store := &testkit.FakeStore{
				HardwareModelHistoryFn: func(_ context.Context, since time.Time, _, totalWeeks int) (repo.HardwareHistoryResult, error) {
					if totalWeeks != tc.weeks {
						t.Errorf("expected weeks=%d, got %d", tc.weeks, totalWeeks)
					}
					// The Monday of `now`'s week, regardless of where in the
					// week `now` sits.
					wantCurrentWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
					wantSince := wantCurrentWeek.AddDate(0, 0, -7*(tc.weeks-1))
					if !since.Equal(wantSince) {
						t.Errorf("since=%s, want %s (currentWeek - %d weeks)", since, wantSince, tc.weeks-1)
					}

					modelsByWeek := make([]int, tc.weeks)
					for i := range modelsByWeek {
						modelsByWeek[i] = 1
					}

					return repo.HardwareHistoryResult{
						Weeks:        tc.weeks,
						TopN:         1,
						Models:       []string{"heltec-v3"},
						ModelsByWeek: [][]int{modelsByWeek},
					}, nil
				},
			}
			srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: cfg}}},
				store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
			srv.now = func() time.Time { return now }
			srv.hardwareCache.now = func() time.Time { return now }

			req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware/history", nil)
			rec := httptest.NewRecorder()
			srv.hardwareHistory(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
			}
			var payload hardwareHistoryPayload
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(payload.ModelsByWeek) != 1 || len(payload.ModelsByWeek[0]) != tc.weeks {
				t.Fatalf("unexpected payload shape: %d series x %d weeks (want 1 x %d)",
					len(payload.ModelsByWeek), len(payload.ModelsByWeek[0]), tc.weeks)
			}
		})
	}
}

// TestHardwareSnapshotHandler_PassesMaxAgeToStore verifies the snapshot handler
// threads web.stats.hardware.max_age straight through to the store as the
// staleness cutoff (applied to last_seen_any_event_at, the broadest liveness
// column — unlike firmware's last_map_report_at).
func TestHardwareSnapshotHandler_PassesMaxAgeToStore(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	var observed time.Duration
	store := &testkit.FakeStore{
		HardwareModelSnapshotFn: func(_ context.Context, maxAge time.Duration) ([]repo.HardwareModelCount, error) {
			observed = maxAge

			return []repo.HardwareModelCount{{Model: "heltec-v3", Count: 1, LastSeenAt: now}}, nil
		},
	}
	cfg := hardwareStatsConfig
	cfg.MaxAge = 7 * 24 * time.Hour
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware", nil)
	rec := httptest.NewRecorder()
	srv.hardwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if observed != 7*24*time.Hour {
		t.Errorf("expected store to receive 7d maxAge, got %s", observed)
	}
}

// TestHardwareSnapshotHandler_DefaultsMaxAgeWhenZero pins the fallback: if an
// operator sets max_age to 0 (or omits it), the handler must default to 14d so
// the hardware stats don't suddenly exclude every node.
func TestHardwareSnapshotHandler_DefaultsMaxAgeWhenZero(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	var observed time.Duration
	store := &testkit.FakeStore{
		HardwareModelSnapshotFn: func(_ context.Context, maxAge time.Duration) ([]repo.HardwareModelCount, error) {
			observed = maxAge

			return []repo.HardwareModelCount{{Model: "heltec-v3", Count: 1, LastSeenAt: now}}, nil
		},
	}
	cfg := hardwareStatsConfig
	cfg.MaxAge = 0
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware", nil)
	rec := httptest.NewRecorder()
	srv.hardwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if observed != 14*24*time.Hour {
		t.Errorf("expected store to receive 14d default, got %s", observed)
	}
}

// TestHardwareSnapshotHandler_EchoesResolvedTTL pins that the snapshot endpoint
// echoes the operator's resolved TTL (not a hardcoded default) so the UI polls
// on the operator's cadence. Mirrors the firmware PR #111 regression guard.
func TestHardwareSnapshotHandler_EchoesResolvedTTL(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	cfg := hardwareStatsConfig
	cfg.SnapshotCacheTTL = 5 * time.Minute // short, to prove the echo isn't just the default
	store := &testkit.FakeStore{
		HardwareModelSnapshotFn: func(_ context.Context, _ time.Duration) ([]repo.HardwareModelCount, error) {
			return []repo.HardwareModelCount{{Model: "heltec-v3", Count: 1, LastSeenAt: now}}, nil
		},
	}
	srv := New(Config{Web: config.WebConfig{Stats: config.StatsConfig{Hardware: cfg}}},
		store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	srv.now = func() time.Time { return now }
	srv.hardwareCache.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/hardware", nil)
	rec := httptest.NewRecorder()
	srv.hardwareSnapshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload hardwareSnapshotPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CacheTtlSeconds != 300 {
		t.Errorf("expected cache_ttl_seconds 300 (5m resolved), got %d", payload.CacheTtlSeconds)
	}
}
