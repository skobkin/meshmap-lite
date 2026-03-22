package ingest

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"meshmap-lite/internal/dedup"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
	generated "meshmap-lite/internal/meshtasticpb"
	"meshmap-lite/internal/repo"
	"meshmap-lite/internal/repo/testkit"
)

type testStore struct {
	testkit.FakeStore
	lastNode      *domain.Node
	nodesSeen     []domain.Node
	lastPosition  *domain.NodePosition
	lastLogEvent  *domain.LogEvent
	logEventsSeen []domain.LogEvent
	topologySeen  []domain.TopologyEdge
	nodeDisplay   string
}

func (s *testStore) UpsertNode(_ context.Context, node domain.Node) (bool, error) {
	n := node
	s.lastNode = &n
	s.nodesSeen = append(s.nodesSeen, n)

	return false, nil
}

func (s *testStore) UpsertPosition(_ context.Context, pos domain.NodePosition) error {
	p := pos
	s.lastPosition = &p

	return nil
}

func (*testStore) MergeTelemetry(_ context.Context, snap domain.NodeTelemetrySnapshot) (domain.NodeTelemetrySnapshot, error) {
	return snap, nil
}

func (s *testStore) UpsertTopologyEdges(_ context.Context, edges []domain.TopologyEdge) error {
	s.topologySeen = append(s.topologySeen, edges...)

	return nil
}

func (*testStore) InsertChatEvent(context.Context, domain.ChatEvent) (int64, error) {
	return 0, nil
}

func (s *testStore) InsertLogEvent(_ context.Context, e domain.LogEvent) (int64, error) {
	ev := e
	s.lastLogEvent = &ev
	s.logEventsSeen = append(s.logEventsSeen, ev)

	return int64(len(s.logEventsSeen)), nil
}

func (s *testStore) ResolveNodeDisplay(_ context.Context, nodeID string) (string, error) {
	if s.nodeDisplay != "" {
		return s.nodeDisplay, nil
	}

	return nodeID, nil
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
			Variant:       meshtastic.RoutingVariantError,
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
			Variant:       meshtastic.RoutingVariantError,
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

func TestHandleChatEmitsResolvedNodeDisplay(t *testing.T) {
	store := &testStore{nodeDisplay: "skobkin-cap"}
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
	}, "LongFast", now)
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

func TestPersistLogEvent_EmitsResolvedNodeDisplayName(t *testing.T) {
	store := &testStore{}
	store.GetNodeDetailsFn = func(_ context.Context, nodeID string) (repo.NodeDetails, error) {
		if nodeID != "!9028d008" {
			t.Fatalf("unexpected node id lookup: %q", nodeID)
		}

		return repo.NodeDetails{
			Node: domain.Node{
				NodeID:    nodeID,
				LongName:  "Alpha Base",
				ShortName: "AB",
			},
		}, nil
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
		ObservedAt: now,
		NodeID:     "!9028d008",
		EventKind:  domain.LogEventKindRoutingValue,
		Encrypted:  true,
		Channel:    "LongFast",
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
}

func TestCollectNodeEvidenceTracksMQTTGatewaySeparatelyFromSender(t *testing.T) {
	evidences := collectNodeEvidence(meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedTelemetry,
		NodeID: "!a55e5e56",
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!9028d008",
		IsFromMQTT: true,
	})

	if len(evidences) != 2 {
		t.Fatalf("expected sender and gateway evidence, got %#v", evidences)
	}
	if evidences[0].NodeID != "!9028d008" || !evidences[0].MQTTConnected || !evidences[0].MQTTGatewayCapable {
		t.Fatalf("unexpected gateway evidence: %#v", evidences[0])
	}
	if evidences[1].NodeID != "!a55e5e56" || evidences[1].MQTTConnected {
		t.Fatalf("unexpected sender evidence: %#v", evidences[1])
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
		GatewayID:  "!9028d008",
		IsFromMQTT: true,
	}, "LongFast", now)
	if !ok {
		t.Fatalf("expected neighbor evidence upserts to succeed")
	}

	if len(store.nodesSeen) != 4 {
		t.Fatalf("expected reporter, gateway, and two neighbors, got %#v", store.nodesSeen)
	}
	for _, nodeID := range []string{"!49b5976c", "!9028d008", "!11111111", "!22222222"} {
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
	}, "LongFast", now)
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
		NodeID: "!9028d008",
		Traceroute: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "completed",
			FromNodeID:  "!9028d008",
			ToNodeID:    "!a55e5e56",
			ForwardPath: []string{"!a55e5e56", "!01020304", "!9028d008"},
			ReturnPath:  []string{"!9028d008", "!0a0b0c0d", "!a55e5e56"},
		},
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!9028d008",
		IsFromMQTT: true,
	}, "LongFast", now) {
		t.Fatalf("expected traceroute evidence upserts to succeed")
	}
	if !svc.upsertNodeEvidenceSet(context.Background(), meshtastic.ParsedEvent{
		Kind:   meshtastic.ParsedRouting,
		NodeID: "!9028d008",
		Routing: &meshtastic.RoutingPayload{
			Variant:    meshtastic.RoutingVariantReply,
			FromNodeID: "!9028d008",
			ToNodeID:   "!abcdef01",
			Route:      []string{"!22222222"},
			RouteBack:  []string{"!33333333"},
		},
	}, meshtastic.TopicInfo{
		Kind:       meshtastic.TopicKindChannel,
		Channel:    "LongFast",
		GatewayID:  "!9028d008",
		IsFromMQTT: true,
	}, "LongFast", now) {
		t.Fatalf("expected routing evidence upserts to succeed")
	}

	for _, nodeID := range []string{"!9028d008", "!a55e5e56", "!01020304", "!0a0b0c0d", "!abcdef01", "!22222222", "!33333333"} {
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
	}, "LongFast", now)

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
		NodeID: "!9028d008",
		Traceroute: &meshtastic.TraceroutePayload{
			ForwardPath:         []string{"!a55e5e56", "!01020304", "!9028d008"},
			ReturnPath:          []string{"!9028d008", "!0a0b0c0d", "!a55e5e56"},
			InferredForwardPath: true,
		},
	}, "LongFast", now)
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
		NodeID: "!9028d008",
		Routing: &meshtastic.RoutingPayload{
			FromNodeID: "!9028d008",
			ToNodeID:   "!abcdef01",
			Route:      []string{"!22222222"},
			RouteBack:  []string{"!33333333"},
		},
	}, "LongFast", now)
	if len(routingEdges) != 4 {
		t.Fatalf("expected 4 routing edges, got %#v", routingEdges)
	}
	if routingEdges[0].SourceKind != domain.TopologySourceRoutingForward || routingEdges[0].FromNodeID != "!9028d008" || routingEdges[0].ToNodeID != "!22222222" {
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
		NodeID: "!9028d008",
		Routing: &meshtastic.RoutingPayload{
			Variant:     meshtastic.RoutingVariantError,
			FromNodeID:  "!9028d008",
			ToNodeID:    "!abcdef01",
			ErrorReason: "NO_ROUTE",
		},
	}, "LongFast", now)
	if len(edges) != 0 {
		t.Fatalf("expected no topology edges for routing error without route, got %#v", edges)
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

	svc.HandleMessage(context.Background(), "msh/RU/ARKH/e/LongSlow/!9028d008", envelopePayload)

	if len(store.topologySeen) != 1 {
		t.Fatalf("expected topology edge to persist for secondary channel, got %#v", store.topologySeen)
	}
	if store.topologySeen[0].ChannelName != "LongSlow" {
		t.Fatalf("unexpected topology channel: %#v", store.topologySeen[0])
	}
	if len(store.nodesSeen) != 0 {
		t.Fatalf("secondary channel should still skip primary-gated node upserts, got %#v", store.nodesSeen)
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
