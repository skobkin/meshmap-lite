package ingest

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
	"meshmap-lite/internal/repo"
)

type testStore struct {
	lastNode     *domain.Node
	lastPosition *domain.NodePosition
	lastLogEvent *domain.LogEvent
}

func (s *testStore) UpsertNode(_ context.Context, node domain.Node) (bool, error) {
	n := node
	s.lastNode = &n

	return false, nil
}

func (s *testStore) UpsertPosition(_ context.Context, pos domain.NodePosition) error {
	p := pos
	s.lastPosition = &p

	return nil
}

func (*testStore) MergeTelemetry(context.Context, domain.NodeTelemetrySnapshot) error {
	return nil
}

func (*testStore) InsertChatEvent(context.Context, domain.ChatEvent) (int64, error) {
	return 0, nil
}

func (s *testStore) InsertLogEvent(_ context.Context, e domain.LogEvent) (int64, error) {
	ev := e
	s.lastLogEvent = &ev

	return 0, nil
}

func (*testStore) GetMapNodes(context.Context) ([]repo.MapNode, error) {
	return nil, nil
}

func (*testStore) ListNodes(context.Context) ([]repo.NodeSummary, error) {
	return nil, nil
}

func (*testStore) GetNodeDetails(context.Context, string) (repo.NodeDetails, error) {
	return repo.NodeDetails{}, nil
}

func (*testStore) ListChatEvents(context.Context, string, int, int64) ([]domain.ChatEvent, error) {
	return nil, nil
}

func (*testStore) ListLogEvents(context.Context, domain.LogEventQuery) ([]domain.LogEventView, error) {
	return nil, nil
}

func (*testStore) Stats(context.Context, time.Duration) (domain.Stats, error) {
	return domain.Stats{}, nil
}

type testEmitter struct{}

func (testEmitter) Emit(domain.RealtimeEvent) {}

func TestHandleMapReportMergesNodeAndPositionFields(t *testing.T) {
	store := &testStore{}
	svc := &Service{
		store:   store,
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	neighbors := 9
	precision := uint32(12)
	alt := 131.0
	now := time.Now().UTC()
	evt := meshtastic.ParsedEvent{
		NodeID: "!9028d008",
		MapReport: &meshtastic.MapReportPayload{
			LongName:               "arkh-07",
			ShortName:              "am07",
			Role:                   "CLIENT",
			BoardModel:             "T_BEAM",
			FirmwareVersion:        "2.7.18.fb3bf780",
			LoRaRegion:             "RU",
			ModemPreset:            "LONG_FAST",
			HasDefaultChannel:      true,
			HasOptedReportLocation: true,
			NeighborNodesCount:     &neighbors,
			Latitude:               64.5,
			Longitude:              40.6,
			AltitudeM:              &alt,
			PositionPrecision:      &precision,
		},
	}

	ok := svc.handleMapReport(context.Background(), evt, now)
	if !ok {
		t.Fatalf("expected map report to be processed")
	}
	if store.lastNode == nil {
		t.Fatalf("expected node upsert")
	}
	if store.lastNode.FirmwareVersion != "2.7.18.fb3bf780" {
		t.Fatalf("unexpected firmware version: %q", store.lastNode.FirmwareVersion)
	}
	if store.lastNode.LoRaRegion != "RU" {
		t.Fatalf("unexpected region: %q", store.lastNode.LoRaRegion)
	}
	if store.lastNode.ModemPreset != "LONG_FAST" {
		t.Fatalf("unexpected modem preset: %q", store.lastNode.ModemPreset)
	}
	if store.lastNode.HasDefaultChannel == nil || !*store.lastNode.HasDefaultChannel {
		t.Fatalf("unexpected has_default_channel: %v", store.lastNode.HasDefaultChannel)
	}
	if store.lastNode.HasOptedReportLocation == nil || !*store.lastNode.HasOptedReportLocation {
		t.Fatalf("unexpected has_opted_report_location: %v", store.lastNode.HasOptedReportLocation)
	}
	if store.lastNode.NeighborNodesCount == nil || *store.lastNode.NeighborNodesCount != 9 {
		t.Fatalf("unexpected neighbor count: %v", store.lastNode.NeighborNodesCount)
	}
	if store.lastPosition == nil {
		t.Fatalf("expected position upsert")
	}
	if store.lastPosition.SourceKind != domain.PositionSourceMapReport {
		t.Fatalf("unexpected source kind: %q", store.lastPosition.SourceKind)
	}
	if store.lastPosition.PositionPrecision == nil || *store.lastPosition.PositionPrecision != 12 {
		t.Fatalf("unexpected position precision: %v", store.lastPosition.PositionPrecision)
	}
}

func TestLogEventFromParsedTracerouteUsesSemanticDetails(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTraceroute,
		NodeID: "!9028d008",
		Traceroute: &meshtastic.TraceroutePayload{
			Role:                "reply",
			Status:              "completed",
			RequestID:           321,
			ReplyID:             654,
			FromNodeID:          "!9028d008",
			ToNodeID:            "!a55e5e56",
			Route:               []string{"!01020304"},
			SnrTowards:          []int32{22},
			RouteBack:           []string{"!0a0b0c0d"},
			SnrBack:             []int32{12},
			ForwardPath:         []string{"!a55e5e56", "!01020304", "!9028d008"},
			ReturnPath:          []string{"!9028d008", "!0a0b0c0d", "!a55e5e56"},
			InferredForwardPath: true,
			InferredDirect:      false,
			WantResponse:        false,
			HopStart:            7,
			HopLimit:            7,
			Bitfield:            3,
		},
	}, "LongFast", now)
	if !ok {
		t.Fatalf("expected traceroute log event")
	}
	if event.EventKind != domain.LogEventKindTracerouteValue {
		t.Fatalf("unexpected event kind: %v", event.EventKind)
	}
	if event.Details["role"] != "reply" || event.Details["status"] != "completed" {
		t.Fatalf("unexpected traceroute summary details: %#v", event.Details)
	}
	if event.Details["request_id"] != uint32(321) || event.Details["reply_id"] != uint32(654) {
		t.Fatalf("unexpected traceroute correlation details: %#v", event.Details)
	}
	if _, ok := event.Details["forward_path"]; !ok {
		t.Fatalf("expected forward_path in details: %#v", event.Details)
	}
	if _, ok := event.Details["return_path"]; !ok {
		t.Fatalf("expected return_path in details: %#v", event.Details)
	}
	if event.Details["inferred_forward_path"] != true {
		t.Fatalf("expected inferred forward path marker: %#v", event.Details)
	}
}

func TestLogEventFromParsedRoutingKeepsTracerouteFailureSignal(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedRouting,
		NodeID: "!9028d008",
		Routing: &meshtastic.RoutingPayload{
			Variant:       "error",
			RequestID:     321,
			FromNodeID:    "!9028d008",
			ToNodeID:      "!a55e5e56",
			ErrorReason:   "NO_ROUTE",
			TracerouteRef: true,
		},
	}, "LongFast", now)
	if !ok {
		t.Fatalf("expected routing log event")
	}
	if event.EventKind != domain.LogEventKindRoutingValue {
		t.Fatalf("unexpected event kind: %v", event.EventKind)
	}
	if event.Details["error_reason"] != "NO_ROUTE" {
		t.Fatalf("unexpected routing error details: %#v", event.Details)
	}
	if event.Details["traceroute_status"] != "failed" {
		t.Fatalf("expected traceroute failure signal in routing details: %#v", event.Details)
	}

	event, ok = svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedRouting,
		NodeID: "!9028d008",
		Routing: &meshtastic.RoutingPayload{
			Variant:       "error",
			RequestID:     321,
			ErrorReason:   "NONE",
			TracerouteRef: true,
		},
	}, "LongFast", now)
	if !ok {
		t.Fatalf("expected routing log event")
	}
	if _, exists := event.Details["traceroute_status"]; exists {
		t.Fatalf("NONE routing error must not mark traceroute failed: %#v", event.Details)
	}
}
