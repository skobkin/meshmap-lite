package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"meshmap-lite/internal/dedup"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
	generated "meshmap-lite/internal/meshtasticpb"
	"meshmap-lite/internal/repo/testkit"
)

type testStore struct {
	testkit.FakeStore
	ops           []string
	lastNode      *domain.Node
	nodesSeen     []domain.Node
	lastPosition  *domain.NodePosition
	positionsSeen []domain.NodePosition
	lastTelemetry *domain.NodeTelemetrySnapshot
	lastChat      *domain.ChatEvent
	lastLogEvent  *domain.LogEvent
	logEventsSeen []domain.LogEvent
	topologySeen  []domain.TopologyEdge
}

func (s *testStore) UpsertNode(_ context.Context, node domain.Node) (bool, error) {
	n := node
	s.ops = append(s.ops, "node:"+n.NodeID)
	s.lastNode = &n
	s.nodesSeen = append(s.nodesSeen, n)

	return false, nil
}

func (s *testStore) UpsertPosition(_ context.Context, pos domain.NodePosition) error {
	p := pos
	s.ops = append(s.ops, "position:"+p.NodeID)
	s.lastPosition = &p
	s.positionsSeen = append(s.positionsSeen, p)

	return nil
}

func (s *testStore) MergeTelemetry(_ context.Context, snap domain.NodeTelemetrySnapshot) (domain.NodeTelemetrySnapshot, error) {
	telemetry := snap
	s.ops = append(s.ops, "telemetry:"+telemetry.NodeID)
	s.lastTelemetry = &telemetry

	return snap, nil
}

func (s *testStore) UpsertTopologyEdges(_ context.Context, edges []domain.TopologyEdge) error {
	s.ops = append(s.ops, "topology")
	s.topologySeen = append(s.topologySeen, edges...)

	return nil
}

func (s *testStore) InsertChatEvent(_ context.Context, event domain.ChatEvent) (int64, error) {
	chat := event
	s.ops = append(s.ops, "chat:"+chat.NodeID)
	s.lastChat = &chat

	return 0, nil
}

func (s *testStore) InsertLogEvent(_ context.Context, e domain.LogEvent) (int64, error) {
	ev := e
	s.ops = append(s.ops, "log:"+ev.NodeID)
	s.lastLogEvent = &ev
	s.logEventsSeen = append(s.logEventsSeen, ev)

	return int64(len(s.logEventsSeen)), nil
}

type testEmitter struct{}

func (testEmitter) Emit(domain.RealtimeEvent) {}

type capturingEmitter struct {
	events []domain.RealtimeEvent
}

func (e *capturingEmitter) Emit(event domain.RealtimeEvent) {
	e.events = append(e.events, event)
}

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
		NodeID: "!11223344",
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

	ok := svc.handleMapReport(context.Background(), evt, "!11223344", now)
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
	if store.lastPosition.MQTTUploaderNodeID != "!11223344" {
		t.Fatalf("expected map report uploader provenance, got %#v", store.lastPosition)
	}
}

func TestHandlePositionSkipsZeroCoordinates(t *testing.T) {
	store := &testStore{}
	emitter := &capturingEmitter{}
	svc := &Service{
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	now := time.Unix(1772296589, 0).UTC()

	ok := svc.handlePosition(context.Background(), meshtastic.ParsedEvent{
		NodeID:   "!sender",
		Position: &meshtastic.PositionPayload{Latitude: 0, Longitude: 0},
	}, "LongFast", "!gateway", now, domain.PositionSourceChannel)

	if ok {
		t.Fatalf("expected zero position to be skipped")
	}
	if len(store.positionsSeen) != 0 {
		t.Fatalf("expected no position upsert, got %#v", store.positionsSeen)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("expected no realtime event, got %#v", emitter.events)
	}
}

func TestHandlePositionAllowsSingleZeroCoordinate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		latitude  float64
		longitude float64
	}{
		{name: "zero latitude", latitude: 0, longitude: 40.6},
		{name: "zero longitude", latitude: 64.5, longitude: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &testStore{}
			svc := &Service{
				store:   store,
				emitter: testEmitter{},
				log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
			}
			now := time.Unix(1772296589, 0).UTC()

			ok := svc.handlePosition(context.Background(), meshtastic.ParsedEvent{
				NodeID:   "!sender",
				Position: &meshtastic.PositionPayload{Latitude: tc.latitude, Longitude: tc.longitude},
			}, "LongFast", "!gateway", now, domain.PositionSourceChannel)

			if !ok {
				t.Fatalf("expected position to be processed")
			}
			if store.lastPosition == nil {
				t.Fatalf("expected position upsert")
			}
			if store.lastPosition.Latitude != tc.latitude || store.lastPosition.Longitude != tc.longitude {
				t.Fatalf("unexpected position: %#v", store.lastPosition)
			}
		})
	}
}

func TestHandleMapReportSkipsZeroPositionButMergesNode(t *testing.T) {
	store := &testStore{}
	emitter := &capturingEmitter{}
	svc := &Service{
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	now := time.Unix(1772296589, 0).UTC()
	evt := meshtastic.ParsedEvent{
		NodeID: "!11223344",
		MapReport: &meshtastic.MapReportPayload{
			LongName:          "arkh-07",
			ShortName:         "am07",
			FirmwareVersion:   "2.7.18.fb3bf780",
			HasDefaultChannel: true,
			Latitude:          0,
			Longitude:         0,
		},
	}

	ok := svc.handleMapReport(context.Background(), evt, "!11223344", now)

	if !ok {
		t.Fatalf("expected map report identity to be processed")
	}
	if store.lastNode == nil {
		t.Fatalf("expected node upsert")
	}
	if store.lastNode.LongName != "arkh-07" || store.lastNode.FirmwareVersion != "2.7.18.fb3bf780" {
		t.Fatalf("unexpected node fields: %#v", store.lastNode)
	}
	if len(store.positionsSeen) != 0 {
		t.Fatalf("expected no position upsert, got %#v", store.positionsSeen)
	}
	for _, event := range emitter.events {
		if event.Type == "node.position" {
			t.Fatalf("expected no node.position event, got %#v", emitter.events)
		}
	}
}

func TestLogEventFromParsedTracerouteUsesSemanticDetails(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTraceroute,
		NodeID: "!11223344",
		Traceroute: &meshtastic.TraceroutePayload{
			Role:                "reply",
			Status:              "completed",
			RequestID:           321,
			ReplyID:             654,
			FromNodeID:          "!11223344",
			ToNodeID:            "!a55e5e56",
			Route:               []string{"!01020304"},
			SnrTowards:          []int32{22},
			RouteBack:           []string{"!0a0b0c0d"},
			SnrBack:             []int32{12},
			ForwardPath:         []string{"!a55e5e56", "!01020304", "!11223344"},
			ReturnPath:          []string{"!11223344", "!0a0b0c0d", "!a55e5e56"},
			InferredForwardPath: true,
			InferredDirect:      false,
			WantResponse:        false,
			HopStart:            7,
			HopLimit:            7,
			Bitfield:            3,
		},
	}, "LongFast", "", now)
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
		NodeID: "!11223344",
		Routing: &meshtastic.RoutingPayload{
			Variant:       meshtastic.RoutingVariantError,
			RequestID:     321,
			FromNodeID:    "!11223344",
			ToNodeID:      "!a55e5e56",
			ErrorReason:   "NO_ROUTE",
			TracerouteRef: true,
		},
	}, "LongFast", "", now)
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
		NodeID: "!11223344",
		Routing: &meshtastic.RoutingPayload{
			Variant:       meshtastic.RoutingVariantError,
			RequestID:     321,
			ErrorReason:   "NONE",
			TracerouteRef: true,
		},
	}, "LongFast", "", now)
	if !ok {
		t.Fatalf("expected routing log event")
	}
	if _, exists := event.Details["traceroute_status"]; exists {
		t.Fatalf("NONE routing error must not mark traceroute failed: %#v", event.Details)
	}
}

func TestLogEventFromParsedRangeTestUsesDedicatedKindWithoutFallbackDetails(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:    meshtastic.ParsedOtherPortnum,
		NodeID:  "!11223344",
		Portnum: generated.PortNum_RANGE_TEST_APP,
		Other: &meshtastic.OtherPortnumPayload{
			PortnumValue: int32(generated.PortNum_RANGE_TEST_APP),
			PortnumName:  generated.PortNum_RANGE_TEST_APP.String(),
		},
	}, "LongFast", "", now)
	if !ok {
		t.Fatalf("expected range test log event")
	}
	if event.EventKind != domain.LogEventKindRangeTestValue {
		t.Fatalf("unexpected event kind: %v", event.EventKind)
	}
	if event.Details != nil {
		t.Fatalf("expected range test log event without fallback details: %#v", event.Details)
	}
}

func TestLogEventFromParsedOtherPortnumKeepsFallbackDetails(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:    meshtastic.ParsedOtherPortnum,
		NodeID:  "!11223344",
		Portnum: generated.PortNum_SERIAL_APP,
		Other: &meshtastic.OtherPortnumPayload{
			PortnumValue: int32(generated.PortNum_SERIAL_APP),
			PortnumName:  generated.PortNum_SERIAL_APP.String(),
		},
	}, "LongFast", "", now)
	if !ok {
		t.Fatalf("expected other-portnum log event")
	}
	if event.EventKind != domain.LogEventKindOtherPortnumValue {
		t.Fatalf("unexpected event kind: %v", event.EventKind)
	}
	if event.Details["portnum_name"] != generated.PortNum_SERIAL_APP.String() {
		t.Fatalf("unexpected fallback details: %#v", event.Details)
	}
}

func TestLogEventFromParsedPKIUsesDedicatedKindWithOuterHeaderDetails(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:      meshtastic.ParsedPKI,
		NodeID:    "!a55e5e56",
		PacketID:  3350416627,
		Encrypted: true,
		Decrypted: false,
		PKI: &meshtastic.PKIPayload{
			SenderNodeID:      "!a55e5e56",
			DestinationNodeID: "!698509f8",
			GatewayID:         "!11223344",
			TopicChannel:      "PKI",
			EnvelopeChannelID: "PKI",
			PacketID:          3350416627,
			Encrypted:         true,
			Decrypted:         false,
			PKIEncrypted:      true,
			PayloadSizeBytes:  79,
			HopStart:          7,
			HopLimit:          7,
			Priority:          "UNSET",
		},
	}, "PKI", "", now)
	if !ok {
		t.Fatalf("expected PKI log event")
	}
	if event.EventKind != domain.LogEventKindPKIValue {
		t.Fatalf("unexpected event kind: %v", event.EventKind)
	}
	if event.Details["gateway_id"] != "!11223344" || event.Details["destination_node_id"] != "!698509f8" {
		t.Fatalf("unexpected PKI details: %#v", event.Details)
	}
	if event.Details["pki_encrypted"] != true || event.Details["payload_size_bytes"] != 79 {
		t.Fatalf("unexpected PKI flags: %#v", event.Details)
	}
}

func TestHandleMessagePersistsPKILogEvent(t *testing.T) {
	store := &testStore{}
	svc := New(Config{
		RootTopic: "msh/RU/ARKH",
		Channels: map[string]ChannelConfig{
			"PKI": {Primary: true},
		},
		MapReports: MapReportsConfig{TopicSuffix: "2/map"},
	}, store, dedup.New(dedup.Options{Size: 32, TTL: time.Minute}), testEmitter{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	envelopePayload, err := proto.Marshal(&generated.ServiceEnvelope{
		ChannelId: "PKI",
		GatewayId: "!11223344",
		Packet: &generated.MeshPacket{
			From:         0xa55e5e56,
			To:           0x698509f8,
			Id:           3350416627,
			HopStart:     7,
			HopLimit:     7,
			PkiEncrypted: true,
			PayloadVariant: &generated.MeshPacket_Encrypted{
				Encrypted: make([]byte, 79),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/2/e/PKI/!11223344", envelopePayload)

	if store.lastLogEvent == nil {
		t.Fatalf("expected PKI log event to persist")
	}
	if store.lastLogEvent.EventKind != domain.LogEventKindPKIValue {
		t.Fatalf("unexpected event kind: %#v", store.lastLogEvent)
	}
	if store.lastLogEvent.Details["gateway_id"] != "!11223344" {
		t.Fatalf("unexpected gateway details: %#v", store.lastLogEvent.Details)
	}
	if store.lastLogEvent.Details["topic_channel"] != "PKI" {
		t.Fatalf("unexpected topic channel details: %#v", store.lastLogEvent.Details)
	}
	if !sawNode(store.nodesSeen, "!698509f8") {
		t.Fatalf("expected destination node evidence, got %#v", store.nodesSeen)
	}
}

func TestHandleMessagePersistsPKILogEventForPKITopicWithoutPKIFlag(t *testing.T) {
	store := &testStore{}
	svc := New(Config{
		RootTopic: "msh/RU/ARKH",
		Channels: map[string]ChannelConfig{
			"PKI": {Primary: true},
		},
		MapReports: MapReportsConfig{TopicSuffix: "2/map"},
	}, store, dedup.New(dedup.Options{Size: 32, TTL: time.Minute}), testEmitter{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	envelopePayload, err := proto.Marshal(&generated.ServiceEnvelope{
		ChannelId: "PKI",
		GatewayId: "!11223344",
		Packet: &generated.MeshPacket{
			From:     0xa55e5e56,
			To:       0x11223344,
			Id:       3350416642,
			HopStart: 7,
			HopLimit: 7,
			PayloadVariant: &generated.MeshPacket_Encrypted{
				Encrypted: []byte{0xde, 0xad, 0xbe, 0xef, 0xca},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/2/e/PKI/!11223344", envelopePayload)

	if store.lastLogEvent == nil {
		t.Fatalf("expected PKI log event to persist")
	}
	if store.lastLogEvent.EventKind != domain.LogEventKindPKIValue {
		t.Fatalf("unexpected event kind: %#v", store.lastLogEvent)
	}
	if store.lastLogEvent.Details["pki_encrypted"] != false {
		t.Fatalf("expected PKI log details to preserve wire flag, got %#v", store.lastLogEvent.Details)
	}
	if store.lastLogEvent.Details["destination_node_id"] != "!11223344" {
		t.Fatalf("unexpected destination details: %#v", store.lastLogEvent.Details)
	}
	if !sawNode(store.nodesSeen, "!11223344") {
		t.Fatalf("expected destination node evidence, got %#v", store.nodesSeen)
	}
}

func TestHandleMessagePersistsPKILogEventWithoutConfiguredPKIChannel(t *testing.T) {
	store := &testStore{}
	svc := New(Config{
		RootTopic:  "msh/RU/ARKH",
		Channels:   map[string]ChannelConfig{},
		MapReports: MapReportsConfig{TopicSuffix: "2/map"},
	}, store, dedup.New(dedup.Options{Size: 32, TTL: time.Minute}), testEmitter{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	envelopePayload, err := proto.Marshal(&generated.ServiceEnvelope{
		ChannelId: "PKI",
		GatewayId: "!11223344",
		Packet: &generated.MeshPacket{
			From:         0x11223344,
			To:           0xa55e5e56,
			Id:           3986283477,
			HopStart:     2,
			HopLimit:     2,
			PkiEncrypted: true,
			Priority:     generated.MeshPacket_RELIABLE,
			PayloadVariant: &generated.MeshPacket_Encrypted{
				Encrypted: make([]byte, 110),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/2/e/PKI/!11223344", envelopePayload)

	if store.lastLogEvent == nil {
		t.Fatalf("expected PKI log event to persist")
	}
	if store.lastLogEvent.EventKind != domain.LogEventKindPKIValue {
		t.Fatalf("unexpected event kind: %#v", store.lastLogEvent)
	}
	if store.lastLogEvent.Details["topic_channel"] != "PKI" {
		t.Fatalf("unexpected topic channel details: %#v", store.lastLogEvent.Details)
	}
	if !sawNode(store.nodesSeen, "!a55e5e56") {
		t.Fatalf("expected destination node evidence, got %#v", store.nodesSeen)
	}
	if !sawNode(store.nodesSeen, "!11223344") {
		t.Fatalf("expected sender/gateway node evidence, got %#v", store.nodesSeen)
	}
}

func TestHandleChatEmitsResolvedNodeDisplay(t *testing.T) {
	store := &testStore{}
	store.ResolveNodeDisplayFn = func(_ context.Context, _ string) (string, error) {
		return "skobkin-cap", nil
	}
	emitter := &capturingEmitter{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	ok := svc.handleChat(context.Background(), meshtastic.ParsedEvent{
		NodeID: "!a55e5e56",
		Chat:   &meshtastic.ChatPayload{Text: "hello"},
	}, "LongFast", "", now)
	if !ok {
		t.Fatalf("expected chat to be processed")
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected one emitted event, got %d", len(emitter.events))
	}
	payload, ok := emitter.events[0].Payload.(domain.ChatEvent)
	if !ok {
		t.Fatalf("unexpected payload type: %T", emitter.events[0].Payload)
	}
	if payload.NodeDisplay != "skobkin-cap" {
		t.Fatalf("expected resolved node display, got %q", payload.NodeDisplay)
	}
}

func TestPersistLogEvent_EmitsResolvedNodeDisplayNames(t *testing.T) {
	store := &testStore{}
	var resolvedNodeIDs []string
	store.ResolveNodeDisplayFn = func(_ context.Context, nodeID string) (string, error) {
		resolvedNodeIDs = append(resolvedNodeIDs, nodeID)
		switch nodeID {
		case "!11223344":
			return "Alpha Base", nil
		case "!a55e5e56":
			return "Gateway", nil
		default:
			t.Fatalf("unexpected node id lookup: %q", nodeID)

			return "", nil
		}
	}
	emitter := &capturingEmitter{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		cfg: Config{
			Log: LogConfig{LiveUpdates: true},
		},
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	svc.persistLogEvent(context.Background(), domain.LogEvent{
		ObservedAt:         now,
		NodeID:             "!11223344",
		MQTTUploaderNodeID: "!a55e5e56",
		EventKind:          domain.LogEventKindRoutingValue,
		Encrypted:          true,
		Channel:            "LongFast",
	})

	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 emitted event, got %d", len(emitter.events))
	}
	view, ok := emitter.events[0].Payload.(domain.LogEventView)
	if !ok {
		t.Fatalf("unexpected payload type: %T", emitter.events[0].Payload)
	}
	if view.NodeDisplay != "Alpha Base" {
		t.Fatalf("expected resolved node display, got %q", view.NodeDisplay)
	}
	if view.MQTTUploaderDisplayName != "Gateway" {
		t.Fatalf("expected resolved MQTT uploader display, got %q", view.MQTTUploaderDisplayName)
	}
	if len(resolvedNodeIDs) != 2 || resolvedNodeIDs[0] != "!11223344" || resolvedNodeIDs[1] != "!a55e5e56" {
		t.Fatalf("unexpected display lookups: %#v", resolvedNodeIDs)
	}
}

func TestResolveNodeDisplayName_FallsBackToRawNodeID(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	tests := []struct {
		name     string
		nodeID   string
		resolved string
		err      error
		want     string
	}{
		{
			name:     "unknown node",
			nodeID:   "!unknown",
			resolved: "!unknown",
			want:     "!unknown",
		},
		{
			name:   "lookup error",
			nodeID: "!error",
			err:    lookupErr,
			want:   "!error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &testStore{}
			store.ResolveNodeDisplayFn = func(_ context.Context, nodeID string) (string, error) {
				if nodeID != tt.nodeID {
					t.Fatalf("unexpected node id lookup: %q", nodeID)
				}

				return tt.resolved, tt.err
			}
			svc := &Service{
				store: store,
				log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			if got := svc.resolveNodeDisplayName(context.Background(), tt.nodeID); got != tt.want {
				t.Fatalf("resolveNodeDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectNodeEvidenceTracksMQTTGatewaySeparatelyFromSender(t *testing.T) {
	evidences := collectNodeEvidence(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTelemetry,
		NodeID: "!a55e5e56",
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!11223344",
		IsFromMQTT: true,
	}, "!11223344", time.Unix(1772296589, 0).UTC())

	if len(evidences) != 2 {
		t.Fatalf("expected sender and gateway evidence, got %#v", evidences)
	}
	if evidences[0].NodeID != "!11223344" || !evidences[0].MQTTConnected || !evidences[0].MQTTGatewayCapable {
		t.Fatalf("unexpected gateway evidence: %#v", evidences[0])
	}
	if evidences[1].NodeID != "!a55e5e56" || evidences[1].MQTTConnected {
		t.Fatalf("unexpected sender evidence: %#v", evidences[1])
	}
	if evidences[1].MQTTUploaderNodeID != "!11223344" || evidences[1].MQTTUploaderAt == nil {
		t.Fatalf("expected sender uploader provenance, got %#v", evidences[1])
	}
	if evidences[0].MQTTUploaderNodeID != "" {
		t.Fatalf("gateway evidence must not mark itself as packet uploader provenance: %#v", evidences[0])
	}
}

func TestCollectNodeEvidenceMarksMapReportSenderAsMQTTGateway(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	evidences := collectNodeEvidence(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedMapReport,
		NodeID: "!11223344",
	}, meshtastic.TopicInfo{
		Kind:      meshtastic.TopicKindMapReport,
		MapNodeID: "!11223344",
	}, "!11223344", now)

	if len(evidences) != 1 {
		t.Fatalf("expected only map report sender evidence, got %#v", evidences)
	}
	if evidences[0].NodeID != "!11223344" || !evidences[0].MQTTConnected || !evidences[0].MQTTGatewayCapable {
		t.Fatalf("expected map report sender to be marked as MQTT gateway, got %#v", evidences[0])
	}
	if evidences[0].MQTTUploaderNodeID != "!11223344" || evidences[0].MQTTUploaderAt == nil {
		t.Fatalf("expected map report uploader provenance, got %#v", evidences[0])
	}
}

func TestHandlersPersistMQTTUploaderProvenance(t *testing.T) {
	store := &testStore{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		store:   store,
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if !svc.handleChat(context.Background(), meshtastic.ParsedEvent{
		NodeID: "!sender",
		Chat:   &meshtastic.ChatPayload{Text: "hello"},
	}, "LongFast", "!gateway", now) {
		t.Fatalf("expected chat to be processed")
	}
	if store.lastChat == nil || store.lastChat.MQTTUploaderNodeID != "!gateway" {
		t.Fatalf("expected chat uploader provenance, got %#v", store.lastChat)
	}

	if !svc.handlePosition(context.Background(), meshtastic.ParsedEvent{
		NodeID:   "!sender",
		Position: &meshtastic.PositionPayload{Latitude: 64.5, Longitude: 40.6},
	}, "LongFast", "!gateway", now, domain.PositionSourceChannel) {
		t.Fatalf("expected position to be processed")
	}
	if store.lastPosition == nil || store.lastPosition.MQTTUploaderNodeID != "!gateway" {
		t.Fatalf("expected position uploader provenance, got %#v", store.lastPosition)
	}

	if !svc.handleTelemetry(context.Background(), meshtastic.ParsedEvent{
		NodeID:    "!sender",
		Telemetry: &meshtastic.TelemetryPayload{},
	}, "LongFast", "!gateway", now) {
		t.Fatalf("expected telemetry to be processed")
	}
	if store.lastTelemetry == nil || store.lastTelemetry.MQTTUploaderNodeID != "!gateway" {
		t.Fatalf("expected telemetry uploader provenance, got %#v", store.lastTelemetry)
	}

	logEvent, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTelemetry,
		NodeID: "!sender",
	}, "LongFast", "!gateway", now)
	if !ok {
		t.Fatalf("expected log event")
	}
	if logEvent.MQTTUploaderNodeID != "!gateway" {
		t.Fatalf("expected log uploader provenance, got %#v", logEvent)
	}
}

func TestUpsertNodeEvidenceSetDiscoversIndirectNodesFromNeighborInfo(t *testing.T) {
	store := &testStore{}
	svc := &Service{
		store:   store,
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	now := time.Unix(1772296589, 0).UTC()
	ok := svc.upsertNodeEvidenceSet(context.Background(), meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedNeighborInfo,
		NodeID: "!49b5976c",
		Neighbor: &meshtastic.NeighborInfoPayload{
			NodeID:         "!49b5976c",
			NeighborsCount: 2,
			Neighbors: []meshtastic.NeighborInfoNeighbor{
				{NodeID: "!11111111", SNR: 12.5, LastRxTime: 100, NodeBroadcastIntervalSecs: 300},
				{NodeID: "!22222222", SNR: 7.25},
			},
		},
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!11223344",
		IsFromMQTT: true,
	}, "LongFast", "!11223344", now, nodeDiscoveryAuthoritative, nil)
	if !ok {
		t.Fatalf("expected neighbor evidence upserts to succeed")
	}

	if len(store.nodesSeen) != 4 {
		t.Fatalf("expected reporter, gateway, and two neighbors, got %#v", store.nodesSeen)
	}
	for _, nodeID := range []string{"!49b5976c", "!11223344", "!11111111", "!22222222"} {
		if !sawNode(store.nodesSeen, nodeID) {
			t.Fatalf("expected node %s in evidence upserts: %#v", nodeID, store.nodesSeen)
		}
	}
	if findNode(store.nodesSeen, "!11111111").LastSeenMQTTGatewayAt != nil {
		t.Fatalf("indirect neighbor must not be marked MQTT-connected: %#v", findNode(store.nodesSeen, "!11111111"))
	}
}

func TestLogEventFromParsedNeighborInfoIncludesNeighbors(t *testing.T) {
	svc := &Service{}
	now := time.Unix(1772296589, 0).UTC()

	event, ok := svc.logEventFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedNeighborInfo,
		NodeID: "!49b5976c",
		Neighbor: &meshtastic.NeighborInfoPayload{
			NodeID:         "!49b5976c",
			NeighborsCount: 2,
			Neighbors: []meshtastic.NeighborInfoNeighbor{
				{NodeID: "!11111111", SNR: 12.5},
				{NodeID: "!22222222", SNR: 7.25},
			},
		},
	}, "LongFast", "", now)
	if !ok {
		t.Fatalf("expected neighbor log event")
	}
	neighbors, ok := event.Details["neighbors"].([]meshtastic.NeighborInfoNeighbor)
	if !ok || len(neighbors) != 2 {
		t.Fatalf("expected neighbor list in details, got %#v", event.Details)
	}
}

func TestUpsertNodeEvidenceSetDiscoversIndirectNodesFromTracerouteAndRouting(t *testing.T) {
	store := &testStore{}
	svc := &Service{
		store:   store,
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	now := time.Unix(1772296589, 0).UTC()
	if !svc.upsertNodeEvidenceSet(context.Background(), meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTraceroute,
		NodeID: "!11223344",
		Traceroute: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "completed",
			FromNodeID:  "!11223344",
			ToNodeID:    "!a55e5e56",
			ForwardPath: []string{"!a55e5e56", "!01020304", "!11223344"},
			ReturnPath:  []string{"!11223344", "!0a0b0c0d", "!a55e5e56"},
		},
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!11223344",
		IsFromMQTT: true,
	}, "LongFast", "!11223344", now, nodeDiscoveryAuthoritative, nil) {
		t.Fatalf("expected traceroute evidence upserts to succeed")
	}
	if !svc.upsertNodeEvidenceSet(context.Background(), meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedRouting,
		NodeID: "!11223344",
		Routing: &meshtastic.RoutingPayload{
			Variant:    meshtastic.RoutingVariantReply,
			FromNodeID: "!11223344",
			ToNodeID:   "!abcdef01",
			Route:      []string{"!22222222"},
			RouteBack:  []string{"!33333333"},
		},
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!11223344",
		IsFromMQTT: true,
	}, "LongFast", "!11223344", now, nodeDiscoveryAuthoritative, nil) {
		t.Fatalf("expected routing evidence upserts to succeed")
	}

	for _, nodeID := range []string{"!11223344", "!a55e5e56", "!01020304", "!0a0b0c0d", "!abcdef01", "!22222222", "!33333333"} {
		if !sawNode(store.nodesSeen, nodeID) {
			t.Fatalf("expected node %s in indirect discovery set: %#v", nodeID, store.nodesSeen)
		}
	}
}

func TestTopologyEdgesFromParsedNeighborInfo(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	reportedAt := now.Add(-30 * time.Second)

	edges := topologyEdgesFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedNeighborInfo,
		NodeID: "!49b5976c",
		Neighbor: &meshtastic.NeighborInfoPayload{
			NodeID: "!49b5976c",
			Neighbors: []meshtastic.NeighborInfoNeighbor{
				{NodeID: "!11111111", SNR: 0, LastRxTime: 1772296500, NodeBroadcastIntervalSecs: 14400},
				{NodeID: "!11111111", SNR: 0, LastRxTime: 1772296500, NodeBroadcastIntervalSecs: 14400},
				{NodeID: "!22222222", SNR: 7.25},
			},
		},
		Timestamp: &reportedAt,
	}, meshtastic.TopicInfo{Kind: meshtastic.TopicKindChannel, Channel: "LongFast", GatewayID: "!11223344", IsFromMQTT: true}, "LongFast", "!11223344", now)

	if len(edges) != 2 {
		t.Fatalf("expected 2 deduplicated neighbor edges, got %#v", edges)
	}
	if edges[0].SourceKind != domain.TopologySourceNeighborInfo || edges[0].FromNodeID != "!49b5976c" || edges[0].ToNodeID != "!11111111" {
		t.Fatalf("unexpected first neighbor edge: %#v", edges[0])
	}
	if edges[0].SNR == nil || *edges[0].SNR != 0 {
		t.Fatalf("expected zero SNR to be preserved, got %#v", edges[0].SNR)
	}
	if edges[0].NeighborLastRXAt == nil || edges[0].NeighborLastRXAt.UTC().Unix() != 1772296500 {
		t.Fatalf("unexpected last rx time: %#v", edges[0].NeighborLastRXAt)
	}
	if edges[0].NeighborBroadcastIntervalSec == nil || *edges[0].NeighborBroadcastIntervalSec != 14400 {
		t.Fatalf("unexpected broadcast interval: %#v", edges[0].NeighborBroadcastIntervalSec)
	}
}

func TestTopologyEdgesFromParsedTracerouteAndRoutingStayDistinct(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()

	tracerouteEdges := topologyEdgesFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTraceroute,
		NodeID: "!11223344",
		Traceroute: &meshtastic.TraceroutePayload{
			ForwardPath:         []string{"!a55e5e56", "!01020304", "!11223344"},
			ReturnPath:          []string{"!11223344", "!0a0b0c0d", "!a55e5e56"},
			InferredForwardPath: true,
		},
	}, meshtastic.TopicInfo{Kind: meshtastic.TopicKindChannel, Channel: "LongFast", GatewayID: "!99887766", IsFromMQTT: true}, "LongFast", "!99887766", now)
	if len(tracerouteEdges) != 4 {
		t.Fatalf("expected 4 traceroute edges, got %#v", tracerouteEdges)
	}
	if tracerouteEdges[0].SourceKind != domain.TopologySourceTracerouteForward || !tracerouteEdges[0].Inferred {
		t.Fatalf("unexpected traceroute forward edge: %#v", tracerouteEdges[0])
	}
	if tracerouteEdges[2].SourceKind != domain.TopologySourceTracerouteReturn || tracerouteEdges[2].Inferred {
		t.Fatalf("unexpected traceroute return edge: %#v", tracerouteEdges[2])
	}

	routingEdges := topologyEdgesFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedRouting,
		NodeID: "!11223344",
		Routing: &meshtastic.RoutingPayload{
			FromNodeID: "!11223344",
			ToNodeID:   "!abcdef01",
			Route:      []string{"!22222222"},
			RouteBack:  []string{"!33333333"},
		},
	}, meshtastic.TopicInfo{Kind: meshtastic.TopicKindChannel, Channel: "LongFast", GatewayID: "!99887766", IsFromMQTT: true}, "LongFast", "!99887766", now)
	if len(routingEdges) != 4 {
		t.Fatalf("expected 4 routing edges, got %#v", routingEdges)
	}
	if routingEdges[0].SourceKind != domain.TopologySourceRoutingForward || routingEdges[0].FromNodeID != "!11223344" || routingEdges[0].ToNodeID != "!22222222" {
		t.Fatalf("unexpected routing forward edge: %#v", routingEdges[0])
	}
	if routingEdges[2].SourceKind != domain.TopologySourceRoutingReturn || routingEdges[2].FromNodeID != "!abcdef01" || routingEdges[2].ToNodeID != "!33333333" {
		t.Fatalf("unexpected routing return edge: %#v", routingEdges[2])
	}
}

func TestTopologyEdgesFromParsedRoutingErrorWithoutRouteSkipsTopology(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()

	edges := topologyEdgesFromParsed(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedRouting,
		NodeID: "!11223344",
		Routing: &meshtastic.RoutingPayload{
			Variant:     meshtastic.RoutingVariantError,
			FromNodeID:  "!11223344",
			ToNodeID:    "!abcdef01",
			ErrorReason: "NO_ROUTE",
		},
	}, meshtastic.TopicInfo{Kind: meshtastic.TopicKindChannel, Channel: "LongFast", GatewayID: "!99887766", IsFromMQTT: true}, "LongFast", "!99887766", now)
	if len(edges) != 0 {
		t.Fatalf("expected no topology edges for routing error without route, got %#v", edges)
	}
}

func TestTopologyEdgesFromParsedMQTTDirectEvidence(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	reportedAt := now.Add(-10 * time.Second)
	topicInfo := meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!11223344",
		IsFromMQTT: true,
	}
	rxSNR := 6.75

	edges := topologyEdgesFromParsed(meshtastic.ParsedEvent{
		Kind:      meshtastic.ParsedUnknownEncrypted,
		NodeID:    "!49b5976c",
		HopStart:  7,
		HopLimit:  7,
		Timestamp: &reportedAt,
		RxSNR:     &rxSNR,
	}, topicInfo, "LongFast", "!11223344", now)

	if len(edges) != 1 {
		t.Fatalf("expected one mqtt_direct edge, got %#v", edges)
	}
	if edges[0].SourceKind != domain.TopologySourceMQTTDirect || edges[0].FromNodeID != "!49b5976c" || edges[0].ToNodeID != "!11223344" {
		t.Fatalf("unexpected mqtt_direct edge: %#v", edges[0])
	}
	if !edges[0].Inferred || edges[0].ReportedByNodeID != "!11223344" {
		t.Fatalf("unexpected mqtt_direct metadata: %#v", edges[0])
	}
	if edges[0].LastReportedAt == nil || !edges[0].LastReportedAt.Equal(reportedAt) {
		t.Fatalf("unexpected reported at: %#v", edges[0].LastReportedAt)
	}
	if edges[0].SNR == nil || *edges[0].SNR != rxSNR {
		t.Fatalf("expected mqtt_direct SNR, got %#v", edges[0].SNR)
	}

	edges = topologyEdgesFromParsed(meshtastic.ParsedEvent{
		Kind:     meshtastic.ParsedUnknownEncrypted,
		NodeID:   "!49b5976c",
		HopStart: 7,
		HopLimit: 7,
	}, topicInfo, "LongFast", "!11223344", now)
	if len(edges) != 1 {
		t.Fatalf("expected one mqtt_direct edge, got %#v", edges)
	}
	if edges[0].SNR != nil {
		t.Fatalf("expected absent mqtt_direct SNR to stay nil, got %#v", edges[0].SNR)
	}
}

func TestTopologyEdgesFromParsedMQTTDirectSkipsAmbiguousPackets(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	topicInfo := meshtastic.TopicInfo{Kind: meshtastic.TopicKindChannel, Channel: "LongFast", GatewayID: "!11223344", IsFromMQTT: true}
	tests := []struct {
		name     string
		evt      meshtastic.ParsedEvent
		topic    meshtastic.TopicInfo
		uploader string
	}{
		{
			name:     "relayed",
			evt:      meshtastic.ParsedEvent{Kind: meshtastic.ParsedChat, NodeID: "!49b5976c", HopStart: 7, HopLimit: 6},
			topic:    topicInfo,
			uploader: "!11223344",
		},
		{
			name:     "missing hop metadata",
			evt:      meshtastic.ParsedEvent{Kind: meshtastic.ParsedChat, NodeID: "!49b5976c", HopStart: 0, HopLimit: 0},
			topic:    topicInfo,
			uploader: "!11223344",
		},
		{
			name:     "self upload",
			evt:      meshtastic.ParsedEvent{Kind: meshtastic.ParsedChat, NodeID: "!11223344", HopStart: 7, HopLimit: 7},
			topic:    topicInfo,
			uploader: "!11223344",
		},
		{
			name:     "map report topic",
			evt:      meshtastic.ParsedEvent{Kind: meshtastic.ParsedMapReport, NodeID: "!49b5976c", HopStart: 7, HopLimit: 7},
			topic:    meshtastic.TopicInfo{Kind: meshtastic.TopicKindMapReport, MapNodeID: "!49b5976c"},
			uploader: "!11223344",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges := topologyEdgesFromParsed(tt.evt, tt.topic, "LongFast", tt.uploader, now)
			if len(edges) != 0 {
				t.Fatalf("expected no mqtt_direct edge, got %#v", edges)
			}
		})
	}
}

func TestHandleMessagePersistsTopologyEdgesForSecondaryChannel(t *testing.T) {
	store := &testStore{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		cfg: Config{
			RootTopic: "msh/RU/ARKH",
			MapReports: MapReportsConfig{
				Enabled:     false,
				TopicSuffix: "2/map",
			},
			Channels: map[string]ChannelConfig{
				"LongFast": {Primary: true},
				"LongSlow": {Primary: false},
			},
		},
		store:   store,
		dedup:   dedup.New(dedup.Options{Size: 32, TTL: time.Minute}),
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time {
			return now
		},
	}

	neighborPayload, err := proto.Marshal(&generated.NeighborInfo{
		NodeId: 0x49b5976c,
		Neighbors: []*generated.Neighbor{
			{NodeId: 0x11111111, Snr: 12.5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	envelopePayload, err := proto.Marshal(&generated.ServiceEnvelope{
		ChannelId: "LongSlow",
		GatewayId: "gw",
		Packet: &generated.MeshPacket{
			From: 0x49b5976c,
			Id:   42,
			PayloadVariant: &generated.MeshPacket_Decoded{
				Decoded: &generated.Data{
					Portnum: generated.PortNum_NEIGHBORINFO_APP,
					Payload: neighborPayload,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/e/LongSlow/!11223344", envelopePayload)

	if len(store.topologySeen) != 1 {
		t.Fatalf("expected topology edge to persist for secondary channel, got %#v", store.topologySeen)
	}
	if store.topologySeen[0].ChannelName != "LongSlow" {
		t.Fatalf("unexpected topology channel: %#v", store.topologySeen[0])
	}
	for _, nodeID := range []string{"!49b5976c", "!11223344", "!11111111"} {
		if !sawNode(store.nodesSeen, nodeID) {
			t.Fatalf("expected minimal node evidence for %s before topology/log exposure, got %#v", nodeID, store.nodesSeen)
		}
	}
	if len(store.positionsSeen) != 0 || store.lastTelemetry != nil {
		t.Fatalf("secondary channel should still skip primary-gated rich state, positions=%#v telemetry=%#v", store.positionsSeen, store.lastTelemetry)
	}
	assertOpBefore(t, store.ops, "node:!49b5976c", "log:!49b5976c")
	assertOpBefore(t, store.ops, "node:!11111111", "topology")
}

func TestHandleMessagePersistsMQTTDirectTopologyForSecondaryChannel(t *testing.T) {
	store := &testStore{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		cfg: Config{
			RootTopic: "msh/RU/ARKH",
			MapReports: MapReportsConfig{
				Enabled:     false,
				TopicSuffix: "2/map",
			},
			Channels: map[string]ChannelConfig{
				"LongFast": {Primary: true},
				"LongSlow": {Primary: false},
			},
		},
		store:   store,
		dedup:   dedup.New(dedup.Options{Size: 32, TTL: time.Minute}),
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time {
			return now
		},
	}

	envelopePayload, err := proto.Marshal(&generated.ServiceEnvelope{
		ChannelId: "LongSlow",
		GatewayId: "gw",
		Packet: &generated.MeshPacket{
			From:     0x49b5976c,
			Id:       43,
			HopStart: 7,
			HopLimit: 7,
			PayloadVariant: &generated.MeshPacket_Encrypted{
				Encrypted: []byte{0xde, 0xad, 0xbe, 0xef},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/e/LongSlow/!11223344", envelopePayload)

	if len(store.topologySeen) != 1 {
		t.Fatalf("expected mqtt_direct topology edge to persist for secondary channel, got %#v", store.topologySeen)
	}
	if store.topologySeen[0].SourceKind != domain.TopologySourceMQTTDirect || store.topologySeen[0].ChannelName != "LongSlow" {
		t.Fatalf("unexpected topology edge: %#v", store.topologySeen[0])
	}
	for _, nodeID := range []string{"!49b5976c", "!11223344"} {
		if !sawNode(store.nodesSeen, nodeID) {
			t.Fatalf("expected minimal node evidence for %s before topology/log exposure, got %#v", nodeID, store.nodesSeen)
		}
	}
	if len(store.positionsSeen) != 0 || store.lastTelemetry != nil {
		t.Fatalf("secondary channel should still skip primary-gated rich state, positions=%#v telemetry=%#v", store.positionsSeen, store.lastTelemetry)
	}
	assertOpBefore(t, store.ops, "node:!49b5976c", "log:!49b5976c")
	assertOpBefore(t, store.ops, "node:!11223344", "topology")
}

func TestHandleMessageDiscoversSecondaryPositionNodeWithoutMergingPosition(t *testing.T) {
	store := &testStore{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		cfg: Config{
			RootTopic:  "msh/RU/ARKH",
			MapReports: MapReportsConfig{TopicSuffix: "2/map"},
			Channels: map[string]ChannelConfig{
				"LongFast": {Primary: true},
				"LongSlow": {Primary: false},
			},
		},
		store:   store,
		dedup:   dedup.New(dedup.Options{Size: 32, TTL: time.Minute}),
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time {
			return now
		},
	}

	positionPayload, err := proto.Marshal(&generated.Position{
		LatitudeI:  proto.Int32(645000000),
		LongitudeI: proto.Int32(406000000),
	})
	if err != nil {
		t.Fatal(err)
	}
	envelopePayload, err := proto.Marshal(&generated.ServiceEnvelope{
		ChannelId: "LongSlow",
		GatewayId: "gw",
		Packet: &generated.MeshPacket{
			From: 0x49b5976c,
			Id:   44,
			PayloadVariant: &generated.MeshPacket_Decoded{
				Decoded: &generated.Data{
					Portnum: generated.PortNum_POSITION_APP,
					Payload: positionPayload,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/e/LongSlow/!11223344", envelopePayload)

	if !sawNode(store.nodesSeen, "!49b5976c") || !sawNode(store.nodesSeen, "!11223344") {
		t.Fatalf("expected sender and uploader minimal node evidence, got %#v", store.nodesSeen)
	}
	if len(store.positionsSeen) != 0 {
		t.Fatalf("secondary channel position should not update rich state, got %#v", store.positionsSeen)
	}
	if store.lastLogEvent == nil || store.lastLogEvent.EventKind != domain.LogEventKindPositionValue {
		t.Fatalf("expected secondary position log event, got %#v", store.lastLogEvent)
	}
	assertOpBefore(t, store.ops, "node:!49b5976c", "log:!49b5976c")
}

func assertOpBefore(t *testing.T, ops []string, before, after string) {
	t.Helper()
	beforeIndex := -1
	afterIndex := -1
	for i, op := range ops {
		if op == before && beforeIndex == -1 {
			beforeIndex = i
		}
		if op == after && afterIndex == -1 {
			afterIndex = i
		}
	}
	if beforeIndex == -1 || afterIndex == -1 || beforeIndex >= afterIndex {
		t.Fatalf("expected %q before %q in ops %#v", before, after, ops)
	}
}

func sawNode(nodes []domain.Node, nodeID string) bool {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return true
		}
	}

	return false
}

func findNode(nodes []domain.Node, nodeID string) domain.Node {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return node
		}
	}

	return domain.Node{}
}

func TestHandleChatCapturesHopMetadata(t *testing.T) {
	store := &testStore{}
	emitter := &capturingEmitter{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if !svc.handleChat(context.Background(), meshtastic.ParsedEvent{
		Kind:     meshtastic.ParsedChat,
		NodeID:   "!a55e5e56",
		Chat:     &meshtastic.ChatPayload{Text: "relayed"},
		HopStart: 7,
		HopLimit: 4,
	}, "LongFast", "!gateway", now) {
		t.Fatalf("expected chat to be processed")
	}

	if store.lastChat == nil {
		t.Fatalf("expected chat to be persisted")
	}
	if store.lastChat.HopStart == nil || *store.lastChat.HopStart != 7 {
		t.Fatalf("expected HopStart=7, got %#v", store.lastChat.HopStart)
	}
	if store.lastChat.HopLimit == nil || *store.lastChat.HopLimit != 4 {
		t.Fatalf("expected HopLimit=4, got %#v", store.lastChat.HopLimit)
	}
}

func TestHandleChatOmitsHopMetadataWhenZero(t *testing.T) {
	store := &testStore{}
	emitter := &capturingEmitter{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if !svc.handleChat(context.Background(), meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedChat,
		NodeID: "!a55e5e56",
		Chat:   &meshtastic.ChatPayload{Text: "no hops"},
	}, "LongFast", "!gateway", now) {
		t.Fatalf("expected chat to be processed")
	}

	if store.lastChat == nil {
		t.Fatalf("expected chat to be persisted")
	}
	if store.lastChat.HopStart != nil {
		t.Fatalf("expected nil HopStart, got %#v", *store.lastChat.HopStart)
	}
	if store.lastChat.HopLimit != nil {
		t.Fatalf("expected nil HopLimit, got %#v", *store.lastChat.HopLimit)
	}
}

func TestLogEventFromParsedCapturesHopMetadata(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	evt := meshtastic.ParsedEvent{
		Kind:     meshtastic.ParsedTelemetry,
		NodeID:   "!a55e5e56",
		HopStart: 5,
		HopLimit: 2,
	}

	e, ok := (&Service{log: slog.New(slog.NewTextHandler(io.Discard, nil))}).
		logEventFromParsed(evt, "LongFast", "!gateway", now)
	if !ok {
		t.Fatalf("expected log event to be produced")
	}
	if e.HopStart == nil || *e.HopStart != 5 {
		t.Fatalf("expected HopStart=5, got %#v", e.HopStart)
	}
	if e.HopLimit == nil || *e.HopLimit != 2 {
		t.Fatalf("expected HopLimit=2, got %#v", e.HopLimit)
	}
}

func TestLogEventFromParsedOmitsHopMetadataWhenZero(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	evt := meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTelemetry,
		NodeID: "!a55e5e56",
	}

	e, ok := (&Service{log: slog.New(slog.NewTextHandler(io.Discard, nil))}).
		logEventFromParsed(evt, "LongFast", "!gateway", now)
	if !ok {
		t.Fatalf("expected log event to be produced")
	}
	if e.HopStart != nil {
		t.Fatalf("expected nil HopStart, got %#v", *e.HopStart)
	}
	if e.HopLimit != nil {
		t.Fatalf("expected nil HopLimit, got %#v", *e.HopLimit)
	}
}

// TestHandleChatPreservesZeroHopLimit guards the regression that
// the old `if evt.HopLimit > 0` guard silently dropped a hop_limit
// of 0, hiding the signal-exhausted state from the UI. A packet
// with HopStart=5 and HopLimit=0 means the packet used the last of
// its hop budget — the most informative hop state — and must be
// persisted verbatim.
func TestHandleChatPreservesZeroHopLimit(t *testing.T) {
	store := &testStore{}
	emitter := &capturingEmitter{}
	now := time.Unix(1772296589, 0).UTC()
	svc := &Service{
		store:   store,
		emitter: emitter,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if !svc.handleChat(context.Background(), meshtastic.ParsedEvent{
		Kind:     meshtastic.ParsedChat,
		NodeID:   "!a55e5e56",
		Chat:     &meshtastic.ChatPayload{Text: "exhausted"},
		HopStart: 5,
		HopLimit: 0,
	}, "LongFast", "!gateway", now) {
		t.Fatalf("expected chat to be processed")
	}

	if store.lastChat == nil {
		t.Fatalf("expected chat to be persisted")
	}
	if store.lastChat.HopStart == nil || *store.lastChat.HopStart != 5 {
		t.Fatalf("expected HopStart=5, got %#v", store.lastChat.HopStart)
	}
	if store.lastChat.HopLimit == nil {
		t.Fatalf("expected non-nil HopLimit pointer for exhausted packet, got nil")
	}
	if *store.lastChat.HopLimit != 0 {
		t.Fatalf("expected HopLimit=0 (exhausted), got %d", *store.lastChat.HopLimit)
	}
}

// TestLogEventFromParsedPreservesZeroHopLimit is the log-event-side
// counterpart to TestHandleChatPreservesZeroHopLimit.
func TestLogEventFromParsedPreservesZeroHopLimit(t *testing.T) {
	now := time.Unix(1772296589, 0).UTC()
	evt := meshtastic.ParsedEvent{
		Kind:     meshtastic.ParsedTelemetry,
		NodeID:   "!a55e5e56",
		HopStart: 5,
		HopLimit: 0,
	}

	e, ok := (&Service{log: slog.New(slog.NewTextHandler(io.Discard, nil))}).
		logEventFromParsed(evt, "LongFast", "!gateway", now)
	if !ok {
		t.Fatalf("expected log event to be produced")
	}
	if e.HopStart == nil || *e.HopStart != 5 {
		t.Fatalf("expected HopStart=5, got %#v", e.HopStart)
	}
	if e.HopLimit == nil {
		t.Fatalf("expected non-nil HopLimit pointer for exhausted packet, got nil")
	}
	if *e.HopLimit != 0 {
		t.Fatalf("expected HopLimit=0 (exhausted), got %d", *e.HopLimit)
	}
}
