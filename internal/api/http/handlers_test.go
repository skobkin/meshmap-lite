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

	srv := New(Config{Web: config.WebConfig{Relevance: config.RelevanceConfig{TopologyEvidenceMaxAge: 72 * time.Hour}}}, store, nil, nil, nil, nil)
	srv.now = func() time.Time { return now }
	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/edges?node_id=!49b5976c&channel=LongFast&source_kind=neighbor_info,mqtt_direct", nil)
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

func TestLogEventsHandlerForwardsNodeIDFilter(t *testing.T) {
	store := &testkit.FakeStore{
		ListLogEventsFn: func(_ context.Context, q domain.LogEventQuery) ([]domain.LogEventView, error) {
			if q.Limit != 25 || q.BeforeID != 44 || q.Channel != "LongFast" || q.NodeID != "!49b5976c" {
				t.Fatalf("unexpected log query: %+v", q)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/events?limit=25&before=44&channel=LongFast&node_id=!49b5976c&event_kind=4", nil)
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
