package ingest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"meshmap-lite/internal/dedup"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
	generated "meshmap-lite/internal/meshtasticpb"
	"meshmap-lite/internal/repo"
)

// RealtimeEmitter emits websocket-compatible realtime events.
type RealtimeEmitter interface {
	Emit(domain.RealtimeEvent)
}

// Service ingests decoded Meshtastic events into storage and realtime streams.
type Service struct {
	cfg     Config
	store   repo.WriteStore
	dedup   *dedup.Store
	emitter RealtimeEmitter
	log     *slog.Logger
	tracker *tracerouteTracker
	now     func() time.Time
}

type nodeEvidence struct {
	NodeID             string
	MQTTConnected      bool
	MQTTGatewayCapable bool
	MQTTUploaderNodeID string
	MQTTUploaderAt     *time.Time
	EmitSystemEvent    bool
	Reason             string
}

// Config contains the subset of app config required by the ingest service.
type Config struct {
	RootTopic  string
	Traceroute TracerouteConfig
	MapReports MapReportsConfig
	Channels   map[string]ChannelConfig
	Log        LogConfig
}

// TracerouteConfig bounds ingest-side traceroute lifecycle tracking.
type TracerouteConfig struct {
	Timeout        time.Duration
	MaxEntries     int
	FinalRetention time.Duration
}

// MapReportsConfig controls optional Meshtastic map report ingest.
type MapReportsConfig struct {
	Enabled     bool
	TopicSuffix string
}

// ChannelConfig contains the per-channel fields used by ingest.
type ChannelConfig struct {
	PSK     string
	Primary bool
}

// LogConfig contains the ingest-relevant log settings.
type LogConfig struct {
	LiveUpdates bool
}

// New constructs ingest service and configures parser channel keys.
func New(cfg Config, store repo.WriteStore, dedupStore *dedup.Store, emitter RealtimeEmitter, log *slog.Logger) *Service {
	keys := make(map[string]string, len(cfg.Channels))
	for name, ch := range cfg.Channels {
		keys[name] = ch.PSK
	}
	meshtastic.ConfigureChannelKeys(keys)

	return &Service{
		cfg:     cfg,
		store:   store,
		dedup:   dedupStore,
		emitter: emitter,
		log:     log,
		tracker: newTracerouteTracker(log, tracerouteTrackerOptions{
			timeout:        cfg.Traceroute.Timeout,
			maxEntries:     cfg.Traceroute.MaxEntries,
			finalRetention: cfg.Traceroute.FinalRetention,
		}),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// HandleMessage processes one MQTT message through topic classification and ingest pipeline.
func (s *Service) HandleMessage(ctx context.Context, topic string, payload []byte) {
	now := s.currentTime()
	s.flushExpiredTraceroutes(ctx, now)
	s.log.Debug("ingest mqtt payload", "topic", topic, "bytes", len(payload))
	topicInfo := meshtastic.ClassifyTopic(s.cfg.RootTopic, s.cfg.MapReports.TopicSuffix, topic)
	if topicInfo.Kind == meshtastic.TopicKindUnknown {
		s.log.Debug("skip message with unknown topic", "topic", topic)

		return
	}
	if topicInfo.Kind == meshtastic.TopicKindMapReport && !s.cfg.MapReports.Enabled {
		s.log.Debug("skip map report because feature disabled", "topic", topic)

		return
	}
	evt, err := meshtastic.ParsePayload(topicInfo.Kind, payload, topicInfo.Channel, topicInfo.MapNodeID)
	if err != nil {
		s.log.Debug("drop undecodable payload", "topic", topic, "err", err)

		return
	}
	s.log.Debug("parsed payload",
		"topic", topic,
		"kind", evt.Kind,
		"node_id", evt.NodeID,
		"packet_id", evt.PacketID,
		"format", evt.Format,
		"encrypted", evt.Encrypted,
		"decrypted", evt.Decrypted,
	)
	if evt.NodeID == "" {
		if evt.Kind == meshtastic.ParsedUnknownEncrypted {
			s.log.Debug("continue unknown encrypted packet without node_id", "topic", topic)
		} else {
			s.log.Debug("drop payload without node_id", "topic", topic)

			return
		}
	}
	if evt.PacketID > 0 {
		if s.dedup.CheckAndMark(fmt.Sprintf("%s:%d", evt.NodeID, evt.PacketID), now) {
			s.log.Debug("skip duplicated packet", "node_id", evt.NodeID, "packet_id", evt.PacketID)

			return
		}
	}
	channel := strings.TrimSpace(topicInfo.Channel)
	mqttUploaderNodeID := mqttUploaderFromTopic(topicInfo)
	if mqttUploaderNodeID == "" && topicInfo.Kind == meshtastic.TopicKindMapReport {
		mqttUploaderNodeID = strings.TrimSpace(evt.NodeID)
	}
	logAllowed := s.allowLogEvent(topicInfo.Kind, channel, evt.Kind)
	tracerouteDecision := tracerouteLogDecision{}
	if logAllowed {
		tracerouteDecision = s.tracerouteLogDecision(evt, channel, now)
	}
	if logEvent, ok := s.logEventFromParsed(evt, channel, mqttUploaderNodeID, now); ok && logAllowed && !tracerouteDecision.suppressPacketLog {
		s.persistLogEvent(ctx, logEvent)
	}
	if logAllowed {
		for _, lifecycleEvent := range tracerouteDecision.lifecycleEvents {
			s.persistLogEvent(ctx, lifecycleEvent)
		}
	}
	if logAllowed {
		if !s.persistTopologyEdges(ctx, evt, channel, now) {
			return
		}
	}
	if !s.allowEvent(channel, evt.Kind) {
		s.log.Debug("skip packet by channel policy", "channel", channel, "kind", evt.Kind, "node_id", evt.NodeID)

		return
	}
	if evt.NodeID == "" {
		return
	}
	if !s.upsertNodeEvidenceSet(ctx, evt, topicInfo, channel, mqttUploaderNodeID, now) {
		return
	}

	switch evt.Kind {
	case meshtastic.ParsedChat:
		if s.handleChat(ctx, evt, channel, mqttUploaderNodeID, now) {
			// Info logs are intentionally limited to decrypted Meshtastic chat only.
			if evt.Format == "protobuf" && evt.Encrypted && evt.Decrypted && evt.Chat != nil {
				s.log.Info("processed decrypted chat message",
					"channel", channel,
					"node_id", evt.NodeID,
					"packet_id", evt.PacketID,
					"text", evt.Chat.Text,
				)
			} else {
				s.log.Debug("processed chat message",
					"channel", channel,
					"node_id", evt.NodeID,
					"packet_id", evt.PacketID,
					"format", evt.Format,
					"encrypted", evt.Encrypted,
					"decrypted", evt.Decrypted,
				)
			}
		}
	case meshtastic.ParsedNodeInfo:
		if s.handleNodeInfo(ctx, evt, now) {
			s.log.Debug("processed node info",
				"node_id", evt.NodeID,
				"long_name", evt.NodeInfo.LongName,
				"short_name", evt.NodeInfo.ShortName,
				"role", evt.NodeInfo.Role,
				"board_model", evt.NodeInfo.BoardModel,
				"firmware_version", evt.NodeInfo.FirmwareVersion,
				"format", evt.Format,
				"encrypted", evt.Encrypted,
				"decrypted", evt.Decrypted,
			)
		}
	case meshtastic.ParsedPosition:
		if s.handlePosition(ctx, evt, channel, mqttUploaderNodeID, now, domain.PositionSourceChannel) {
			s.log.Info("processed position",
				"channel", channel,
				"node_id", evt.NodeID,
				"packet_id", evt.PacketID,
				"lat", evt.Position.Latitude,
				"lon", evt.Position.Longitude,
				"format", evt.Format,
				"encrypted", evt.Encrypted,
				"decrypted", evt.Decrypted,
			)
		}
	case meshtastic.ParsedTelemetry:
		if s.handleTelemetry(ctx, evt, channel, mqttUploaderNodeID, now) {
			s.log.Info("processed telemetry",
				"channel", channel,
				"node_id", evt.NodeID,
				"packet_id", evt.PacketID,
				"format", evt.Format,
				"encrypted", evt.Encrypted,
				"decrypted", evt.Decrypted,
			)
		}
	case meshtastic.ParsedMapReport:
		if s.handleMapReport(ctx, evt, mqttUploaderNodeID, now) {
			s.log.Info("processed position",
				"channel", "",
				"node_id", evt.NodeID,
				"packet_id", evt.PacketID,
				"lat", evt.MapReport.Latitude,
				"lon", evt.MapReport.Longitude,
				"format", evt.Format,
				"source", "map_report",
			)
		}
	}
}

func (s *Service) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}

	return time.Now().UTC()
}

func (s *Service) persistLogEvent(ctx context.Context, logEvent domain.LogEvent) {
	id, err := s.store.InsertLogEvent(ctx, logEvent)
	if err != nil {
		s.log.Error("insert log event failed", "event_kind", logEvent.EventKind, "node_id", logEvent.NodeID, "err", err)

		return
	}
	view := domain.LogEventView{
		ID:                      id,
		ObservedAt:              logEvent.ObservedAt,
		NodeID:                  logEvent.NodeID,
		NodeDisplay:             s.resolveNodeDisplayName(ctx, logEvent.NodeID),
		MQTTUploaderNodeID:      logEvent.MQTTUploaderNodeID,
		MQTTUploaderDisplayName: s.resolveNodeDisplayName(ctx, logEvent.MQTTUploaderNodeID),
		EventKindValue:          logEvent.EventKind,
		EventKindTitle:          domain.LogEventKindTitle(logEvent.EventKind),
		Encrypted:               logEvent.Encrypted,
		Details:                 logEvent.Details,
	}
	if logEvent.Channel != "" {
		ch := logEvent.Channel
		view.ChannelName = &ch
	}
	if s.cfg.Log.LiveUpdates {
		s.emitter.Emit(domain.RealtimeEvent{Type: "log.event", TS: logEvent.ObservedAt, Payload: view})
	}
}

func (s *Service) resolveNodeDisplayName(ctx context.Context, nodeID string) string {
	if nodeID == "" {
		return ""
	}

	type nodeDetailsReader interface {
		GetNodeDetails(context.Context, string) (repo.NodeDetails, error)
	}

	reader, ok := s.store.(nodeDetailsReader)
	if !ok {
		return nodeID
	}

	details, err := reader.GetNodeDetails(ctx, nodeID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.log.Debug("resolve log event node display failed", "node_id", nodeID, "err", err)
		}

		return nodeID
	}
	if details.Node.LongName != "" {
		return details.Node.LongName
	}
	if details.Node.ShortName != "" {
		return details.Node.ShortName
	}

	return nodeID
}

func (s *Service) flushExpiredTraceroutes(ctx context.Context, now time.Time) {
	if s.tracker == nil {
		return
	}
	for _, lifecycle := range s.tracker.Sweep(now) {
		s.persistLogEvent(ctx, tracerouteLifecycleLogEvent(lifecycle))
	}
}

type tracerouteLogDecision struct {
	suppressPacketLog bool
	lifecycleEvents   []domain.LogEvent
}

func (s *Service) tracerouteLogDecision(evt meshtastic.ParsedEvent, channel string, now time.Time) tracerouteLogDecision {
	if s.tracker == nil {
		return tracerouteLogDecision{}
	}

	switch evt.Kind {
	case meshtastic.ParsedTraceroute:
		if evt.Traceroute == nil {
			return tracerouteLogDecision{}
		}
		var result tracerouteTrackerResult
		switch evt.Traceroute.Role {
		case "request":
			result = s.tracker.OnRequest(tracerouteObservation{
				packetID:   evt.PacketID,
				channel:    channel,
				now:        now,
				reportedAt: evt.Timestamp,
				payload:    evt.Traceroute,
			})
		case "reply":
			result = s.tracker.OnReply(tracerouteObservation{
				packetID:   evt.PacketID,
				channel:    channel,
				now:        now,
				reportedAt: evt.Timestamp,
				payload:    evt.Traceroute,
			})
		}

		return tracerouteDecisionFromTracker(result)
	case meshtastic.ParsedRouting:
		if evt.Routing == nil {
			return tracerouteLogDecision{}
		}

		return tracerouteDecisionFromTracker(s.tracker.OnRouting(tracerouteRoutingObservation{
			packetID:   evt.PacketID,
			channel:    channel,
			now:        now,
			reportedAt: evt.Timestamp,
			payload:    evt.Routing,
		}))
	default:
		return tracerouteLogDecision{}
	}
}

func tracerouteDecisionFromTracker(result tracerouteTrackerResult) tracerouteLogDecision {
	decision := tracerouteLogDecision{suppressPacketLog: result.suppressPacketLog}
	if result.lifecycle != nil {
		decision.lifecycleEvents = append(decision.lifecycleEvents, tracerouteLifecycleLogEvent(*result.lifecycle))
	}

	return decision
}

func (s *Service) logEventFromParsed(evt meshtastic.ParsedEvent, channel, mqttUploaderNodeID string, now time.Time) (domain.LogEvent, bool) {
	e := domain.LogEvent{
		ObservedAt:         now,
		NodeID:             evt.NodeID,
		MQTTUploaderNodeID: mqttUploaderNodeID,
		Encrypted:          evt.Encrypted,
		Channel:            channel,
	}
	switch evt.Kind {
	case meshtastic.ParsedMapReport:
		e.Channel = ""
		e.EventKind = domain.LogEventKindMapReportValue

		return e, true
	case meshtastic.ParsedNodeInfo:
		e.EventKind = domain.LogEventKindNodeInfoValue

		return e, true
	case meshtastic.ParsedPosition:
		e.EventKind = domain.LogEventKindPositionValue

		return e, true
	case meshtastic.ParsedTelemetry:
		e.EventKind = domain.LogEventKindTelemetryValue

		return e, true
	case meshtastic.ParsedTraceroute:
		e.EventKind = domain.LogEventKindTracerouteValue
		if evt.Traceroute != nil {
			e.Details = tracerouteLogDetails(evt.Traceroute)
		}

		return e, true
	case meshtastic.ParsedNeighborInfo:
		e.EventKind = domain.LogEventKindNeighborInfoValue
		if evt.Neighbor != nil {
			e.Details = map[string]any{
				"neighbors_count":         evt.Neighbor.NeighborsCount,
				"broadcast_interval_secs": evt.Neighbor.BroadcastInterval,
			}
			if evt.Neighbor.NodeID != "" {
				e.Details["neighbor_node_id"] = evt.Neighbor.NodeID
			}
			if len(evt.Neighbor.Neighbors) > 0 {
				e.Details["neighbors"] = evt.Neighbor.Neighbors
			}
		}

		return e, true
	case meshtastic.ParsedRouting:
		e.EventKind = domain.LogEventKindRoutingValue
		if evt.Routing != nil {
			e.Details = map[string]any{
				"variant": evt.Routing.Variant,
			}
			if evt.Routing.RequestID > 0 {
				e.Details["request_id"] = evt.Routing.RequestID
			}
			if evt.Routing.FromNodeID != "" {
				e.Details["from"] = evt.Routing.FromNodeID
			}
			if evt.Routing.ToNodeID != "" {
				e.Details["to"] = evt.Routing.ToNodeID
			}
			if len(evt.Routing.Route) > 0 {
				e.Details["route"] = evt.Routing.Route
			}
			if len(evt.Routing.RouteBack) > 0 {
				e.Details["route_back"] = evt.Routing.RouteBack
			}
			if evt.Routing.ErrorReason != "" {
				e.Details["error_reason"] = evt.Routing.ErrorReason
				if evt.Routing.RequestID > 0 && evt.Routing.ErrorReason != "NONE" {
					e.Details["traceroute_status"] = "failed"
				}
			}
			if evt.Routing.TracerouteRef {
				e.Details["traceroute_ref"] = true
			}
		}

		return e, true
	case meshtastic.ParsedPKI:
		e.EventKind = domain.LogEventKindPKIValue
		if evt.PKI != nil {
			e.Details = pkiLogDetails(evt.PKI)
		}

		return e, true
	case meshtastic.ParsedOtherPortnum:
		e.EventKind = domain.LogEventKindOtherPortnumValue
		if evt.Portnum == generated.PortNum_RANGE_TEST_APP {
			e.EventKind = domain.LogEventKindRangeTestValue
		}
		if evt.Other != nil {
			if e.EventKind == domain.LogEventKindOtherPortnumValue {
				e.Details = map[string]any{
					"portnum_value": evt.Other.PortnumValue,
					"portnum_name":  evt.Other.PortnumName,
				}
			}
		}

		return e, true
	case meshtastic.ParsedUnknownEncrypted:
		e.EventKind = domain.LogEventKindUnknownEncryptedValue
		e.Encrypted = true

		return e, true
	default:
		return domain.LogEvent{}, false
	}
}

func tracerouteLogDetails(in *meshtastic.TraceroutePayload) map[string]any {
	details := map[string]any{
		"role":          in.Role,
		"status":        in.Status,
		"want_response": in.WantResponse,
		"hop_start":     in.HopStart,
		"hop_limit":     in.HopLimit,
	}
	if in.RequestID > 0 {
		details["request_id"] = in.RequestID
	}
	if in.ReplyID > 0 {
		details["reply_id"] = in.ReplyID
	}
	if in.FromNodeID != "" {
		details["from"] = in.FromNodeID
	}
	if in.ToNodeID != "" {
		details["to"] = in.ToNodeID
	}
	if len(in.Route) > 0 {
		details["route"] = in.Route
	}
	if len(in.SnrTowards) > 0 {
		details["forward_snr"] = in.SnrTowards
	}
	if len(in.RouteBack) > 0 {
		details["route_back"] = in.RouteBack
	}
	if len(in.SnrBack) > 0 {
		details["return_snr"] = in.SnrBack
	}
	if len(in.ForwardPath) > 0 {
		details["forward_path"] = in.ForwardPath
	}
	if len(in.ReturnPath) > 0 {
		details["return_path"] = in.ReturnPath
	}
	if in.Bitfield > 0 {
		details["bitfield"] = in.Bitfield
	}
	if in.InferredForwardPath {
		details["inferred_forward_path"] = true
	}
	if in.InferredReturnPath {
		details["inferred_return_path"] = true
	}
	if in.InferredDirect {
		details["inferred_direct"] = true
	}

	return details
}

func pkiLogDetails(in *meshtastic.PKIPayload) map[string]any {
	if in == nil {
		return nil
	}

	details := map[string]any{
		"sender_node_id":      in.SenderNodeID,
		"destination_node_id": in.DestinationNodeID,
		"gateway_id":          in.GatewayID,
		"topic_channel":       in.TopicChannel,
		"envelope_channel_id": in.EnvelopeChannelID,
		"packet_id":           in.PacketID,
		"encrypted":           in.Encrypted,
		"decrypted":           in.Decrypted,
		"pki_encrypted":       in.PKIEncrypted,
		"payload_size_bytes":  in.PayloadSizeBytes,
	}
	if in.HopStart > 0 {
		details["hop_start"] = in.HopStart
	}
	if in.HopLimit > 0 {
		details["hop_limit"] = in.HopLimit
	}
	if in.Priority != "" {
		details["priority"] = in.Priority
	}

	return details
}

func (s *Service) allowLogEvent(topicKind meshtastic.TopicKind, channel string, kind meshtastic.ParsedKind) bool {
	if kind == meshtastic.ParsedMapReport || topicKind == meshtastic.TopicKindMapReport {
		return s.cfg.MapReports.Enabled
	}
	if kind == meshtastic.ParsedPKI {
		return true
	}
	_, ok := s.cfg.Channels[channel]

	return ok
}

func (s *Service) allowEvent(channel string, kind meshtastic.ParsedKind) bool {
	if kind == meshtastic.ParsedPKI {
		return true
	}
	ch, ok := s.cfg.Channels[channel]
	if !ok && kind != meshtastic.ParsedMapReport {
		return false
	}
	if kind == meshtastic.ParsedChat {
		return true
	}
	if kind == meshtastic.ParsedMapReport {
		return s.cfg.MapReports.Enabled
	}

	return ch.Primary
}

func (s *Service) upsertNodeEvidenceSet(ctx context.Context, evt meshtastic.ParsedEvent, topicInfo meshtastic.TopicInfo, channel, mqttUploaderNodeID string, now time.Time) bool {
	evidences := collectNodeEvidence(evt, topicInfo, mqttUploaderNodeID, now)
	for _, evidence := range evidences {
		if !s.upsertNodeEvidence(ctx, evidence, channel, now) {
			return false
		}
	}

	return true
}

func mqttUploaderFromTopic(topicInfo meshtastic.TopicInfo) string {
	if topicInfo.Kind == meshtastic.TopicKindMapReport {
		return strings.TrimSpace(topicInfo.MapNodeID)
	}
	if topicInfo.Kind != meshtastic.TopicKindChannel || !topicInfo.IsFromMQTT {
		return ""
	}

	return strings.TrimSpace(topicInfo.GatewayID)
}

func collectNodeEvidence(evt meshtastic.ParsedEvent, topicInfo meshtastic.TopicInfo, mqttUploaderNodeID string, now time.Time) []nodeEvidence {
	byID := make(map[string]nodeEvidence)
	add := func(e nodeEvidence) {
		id := strings.TrimSpace(e.NodeID)
		if id == "" {
			return
		}
		current, ok := byID[id]
		if !ok {
			e.NodeID = id
			byID[id] = e

			return
		}
		current.MQTTConnected = current.MQTTConnected || e.MQTTConnected
		current.MQTTGatewayCapable = current.MQTTGatewayCapable || e.MQTTGatewayCapable
		if current.MQTTUploaderNodeID == "" {
			current.MQTTUploaderNodeID = e.MQTTUploaderNodeID
			current.MQTTUploaderAt = e.MQTTUploaderAt
		}
		current.EmitSystemEvent = current.EmitSystemEvent || e.EmitSystemEvent
		if current.Reason == "" {
			current.Reason = e.Reason
		}
		byID[id] = current
	}

	senderEvidence := nodeEvidence{NodeID: evt.NodeID, EmitSystemEvent: true, Reason: "packet_sender"}
	if mqttUploaderNodeID != "" {
		senderEvidence.MQTTUploaderNodeID = mqttUploaderNodeID
		senderEvidence.MQTTUploaderAt = &now
		if topicInfo.Kind == meshtastic.TopicKindMapReport && strings.TrimSpace(evt.NodeID) == mqttUploaderNodeID {
			senderEvidence.MQTTConnected = true
			senderEvidence.MQTTGatewayCapable = true
		}
	}
	add(senderEvidence)

	gatewayID := strings.TrimSpace(topicInfo.GatewayID)
	if topicInfo.IsFromMQTT && gatewayID != "" {
		add(nodeEvidence{
			NodeID:             gatewayID,
			MQTTConnected:      true,
			MQTTGatewayCapable: true,
			Reason:             "mqtt_gateway",
		})
	}

	for _, id := range indirectNodeIDs(evt) {
		add(nodeEvidence{NodeID: id, Reason: "indirect_reference"})
	}

	out := make([]nodeEvidence, 0, len(byID))
	for _, evidence := range byID {
		out = append(out, evidence)
	}
	slices.SortFunc(out, func(a, b nodeEvidence) int {
		return strings.Compare(a.NodeID, b.NodeID)
	})

	return out
}

func indirectNodeIDs(evt meshtastic.ParsedEvent) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	add := func(ids ...string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || id == evt.NodeID {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}

	switch evt.Kind {
	case meshtastic.ParsedNeighborInfo:
		if evt.Neighbor != nil {
			add(evt.Neighbor.NodeID)
			for _, neighbor := range evt.Neighbor.Neighbors {
				add(neighbor.NodeID)
			}
		}
	case meshtastic.ParsedTraceroute:
		if evt.Traceroute != nil {
			add(evt.Traceroute.FromNodeID, evt.Traceroute.ToNodeID)
			add(evt.Traceroute.Route...)
			add(evt.Traceroute.RouteBack...)
			add(evt.Traceroute.ForwardPath...)
			add(evt.Traceroute.ReturnPath...)
		}
	case meshtastic.ParsedRouting:
		if evt.Routing != nil {
			add(evt.Routing.FromNodeID, evt.Routing.ToNodeID)
			add(evt.Routing.Route...)
			add(evt.Routing.RouteBack...)
		}
	case meshtastic.ParsedPKI:
		if evt.PKI != nil {
			add(evt.PKI.DestinationNodeID)
		}
	}

	return out
}

func (s *Service) persistTopologyEdges(ctx context.Context, evt meshtastic.ParsedEvent, channel string, now time.Time) bool {
	edges := topologyEdgesFromParsed(evt, channel, now)
	if len(edges) == 0 {
		return true
	}
	if err := s.store.UpsertTopologyEdges(ctx, edges); err != nil {
		s.log.Error("upsert topology edges failed", "node_id", evt.NodeID, "kind", evt.Kind, "channel", channel, "err", err)

		return false
	}

	return true
}

func topologyEdgesFromParsed(evt meshtastic.ParsedEvent, channel string, now time.Time) []domain.TopologyEdge {
	seen := make(map[string]struct{})
	out := make([]domain.TopologyEdge, 0)
	add := func(edge domain.TopologyEdge) {
		edge.ChannelName = strings.TrimSpace(edge.ChannelName)
		edge.FromNodeID = strings.TrimSpace(edge.FromNodeID)
		edge.ToNodeID = strings.TrimSpace(edge.ToNodeID)
		edge.ReportedByNodeID = strings.TrimSpace(edge.ReportedByNodeID)
		if !edge.SourceKind.Valid() || edge.FromNodeID == "" || edge.ToNodeID == "" || edge.FromNodeID == edge.ToNodeID {
			return
		}
		key := strings.Join([]string{string(edge.SourceKind), edge.ChannelName, edge.FromNodeID, edge.ToNodeID}, "\x00")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, edge)
	}

	switch evt.Kind {
	case meshtastic.ParsedNeighborInfo:
		if evt.Neighbor == nil {
			return nil
		}
		reporterID := strings.TrimSpace(evt.Neighbor.NodeID)
		if reporterID == "" {
			reporterID = strings.TrimSpace(evt.NodeID)
		}
		reportedByID := strings.TrimSpace(evt.NodeID)
		for _, neighbor := range evt.Neighbor.Neighbors {
			edge := domain.TopologyEdge{
				SourceKind:       domain.TopologySourceNeighborInfo,
				ChannelName:      channel,
				FromNodeID:       reporterID,
				ToNodeID:         neighbor.NodeID,
				ReportedByNodeID: reportedByID,
				FirstObservedAt:  now,
				LastObservedAt:   now,
				LastReportedAt:   evt.Timestamp,
				UpdatedAt:        now,
			}
			snr := float64(neighbor.SNR)
			edge.SNR = &snr
			if neighbor.LastRxTime > 0 {
				t := time.Unix(int64(neighbor.LastRxTime), 0).UTC()
				edge.NeighborLastRXAt = &t
			}
			if neighbor.NodeBroadcastIntervalSecs > 0 {
				v := neighbor.NodeBroadcastIntervalSecs
				edge.NeighborBroadcastIntervalSec = &v
			}
			add(edge)
		}
	case meshtastic.ParsedTraceroute:
		if evt.Traceroute == nil {
			return nil
		}
		for _, edge := range topologyEdgesFromPath(domain.TopologySourceTracerouteForward, channel, evt.NodeID, evt.Traceroute.ForwardPath, evt.Traceroute.InferredForwardPath, evt.Timestamp, now) {
			add(edge)
		}
		for _, edge := range topologyEdgesFromPath(domain.TopologySourceTracerouteReturn, channel, evt.NodeID, evt.Traceroute.ReturnPath, evt.Traceroute.InferredReturnPath, evt.Timestamp, now) {
			add(edge)
		}
	case meshtastic.ParsedRouting:
		if evt.Routing == nil {
			return nil
		}
		for _, edge := range topologyEdgesFromPath(domain.TopologySourceRoutingForward, channel, evt.NodeID, routingForwardPath(evt.Routing), false, evt.Timestamp, now) {
			add(edge)
		}
		for _, edge := range topologyEdgesFromPath(domain.TopologySourceRoutingReturn, channel, evt.NodeID, routingReturnPath(evt.Routing), false, evt.Timestamp, now) {
			add(edge)
		}
	}

	return out
}

func topologyEdgesFromPath(sourceKind domain.TopologySourceKind, channel, reportedBy string, path []string, inferred bool, reportedAt *time.Time, now time.Time) []domain.TopologyEdge {
	if len(path) < 2 {
		return nil
	}

	out := make([]domain.TopologyEdge, 0, len(path)-1)
	for i := 0; i < len(path)-1; i++ {
		out = append(out, domain.TopologyEdge{
			SourceKind:       sourceKind,
			ChannelName:      channel,
			FromNodeID:       path[i],
			ToNodeID:         path[i+1],
			ReportedByNodeID: reportedBy,
			Inferred:         inferred,
			FirstObservedAt:  now,
			LastObservedAt:   now,
			LastReportedAt:   reportedAt,
			UpdatedAt:        now,
		})
	}

	return out
}

func routingForwardPath(in *meshtastic.RoutingPayload) []string {
	if in == nil {
		return nil
	}
	if in.Variant == meshtastic.RoutingVariantError && len(in.Route) == 0 {
		return nil
	}

	path := make([]string, 0, len(in.Route)+2)
	if in.FromNodeID != "" {
		path = append(path, in.FromNodeID)
	}
	path = append(path, in.Route...)
	if in.ToNodeID != "" {
		path = append(path, in.ToNodeID)
	}

	return path
}

func routingReturnPath(in *meshtastic.RoutingPayload) []string {
	if in == nil {
		return nil
	}
	if in.Variant == meshtastic.RoutingVariantError && len(in.RouteBack) == 0 {
		return nil
	}

	path := make([]string, 0, len(in.RouteBack)+2)
	if in.ToNodeID != "" {
		path = append(path, in.ToNodeID)
	}
	path = append(path, in.RouteBack...)
	if in.FromNodeID != "" {
		path = append(path, in.FromNodeID)
	}

	return path
}

func (s *Service) upsertNodeEvidence(ctx context.Context, evidence nodeEvidence, channel string, now time.Time) bool {
	node := domain.Node{
		NodeID:             evidence.NodeID,
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}
	if evidence.MQTTConnected {
		node.LastSeenMQTTGatewayAt = &now
	}
	if evidence.MQTTGatewayCapable {
		node.MQTTGatewayCapable = boolPtr(true)
	}
	if evidence.MQTTUploaderNodeID != "" {
		node.LastMQTTUploaderNodeID = evidence.MQTTUploaderNodeID
		node.LastMQTTUploaderAt = evidence.MQTTUploaderAt
	}

	created, err := s.store.UpsertNode(ctx, node)
	if err != nil {
		s.log.Error("upsert node failed", "node_id", evidence.NodeID, "reason", evidence.Reason, "err", err)

		return false
	}
	if evidence.MQTTUploaderNodeID != "" {
		node.LastMQTTUploaderDisplayName = s.resolveNodeDisplayName(ctx, evidence.MQTTUploaderNodeID)
		s.emitter.Emit(domain.RealtimeEvent{Type: "node.upsert", TS: now, Payload: node})
	}
	if !created {
		return true
	}

	if evidence.EmitSystemEvent {
		s.log.Info("new node discovered", "node_id", evidence.NodeID, "channel", channel)
		s.emitSystemNodeDiscovered(ctx, evidence.NodeID, channel, now)

		return true
	}

	s.log.Debug("discovered node from indirect evidence", "node_id", evidence.NodeID, "channel", channel, "reason", evidence.Reason)

	return true
}

func (s *Service) handleChat(ctx context.Context, evt meshtastic.ParsedEvent, channel, mqttUploaderNodeID string, now time.Time) bool {
	ce := domain.ChatEvent{EventType: domain.ChatEventMessage, ChannelName: channel, NodeID: evt.NodeID, MQTTUploaderNodeID: mqttUploaderNodeID, MessageText: evt.Chat.Text, MessageTime: now, ReportedAt: evt.Timestamp, ObservedAt: now, CreatedAt: now}
	if evt.PacketID > 0 {
		v := evt.PacketID
		ce.PacketID = &v
	}
	id, err := s.store.InsertChatEvent(ctx, ce)
	if err != nil {
		s.log.Error("insert chat failed", "err", err)

		return false
	}
	ce.ID = id
	s.populateChatDisplay(ctx, &ce)
	s.emitter.Emit(domain.RealtimeEvent{Type: "chat.message", TS: now, Payload: ce})

	return true
}

func (s *Service) handleNodeInfo(ctx context.Context, evt meshtastic.ParsedEvent, now time.Time) bool {
	in := evt.NodeInfo
	n := domain.Node{
		NodeID:                 evt.NodeID,
		LongName:               in.LongName,
		ShortName:              in.ShortName,
		Role:                   in.Role,
		BoardModel:             in.BoardModel,
		FirmwareVersion:        in.FirmwareVersion,
		LoRaRegion:             in.LoRaRegion,
		LoRaFrequencyDesc:      in.LoRaFrequencyDesc,
		ModemPreset:            in.ModemPreset,
		HasDefaultChannel:      in.HasDefaultChannel,
		HasOptedReportLocation: in.HasOptedReportLocation,
		NeighborNodesCount:     in.NeighborNodesCount,
		FirstSeenAt:            now,
		LastSeenAnyEventAt:     now,
		UpdatedAt:              now,
	}
	if _, err := s.store.UpsertNode(ctx, n); err != nil {
		s.log.Error("upsert nodeinfo failed", "node_id", evt.NodeID, "err", err)

		return false
	}
	s.emitter.Emit(domain.RealtimeEvent{Type: "node.upsert", TS: now, Payload: n})

	return true
}

func (s *Service) handlePosition(ctx context.Context, evt meshtastic.ParsedEvent, channel, mqttUploaderNodeID string, now time.Time, source domain.PositionSourceKind) bool {
	in := evt.Position
	p := domain.NodePosition{
		NodeID:                  evt.NodeID,
		Latitude:                in.Latitude,
		Longitude:               in.Longitude,
		AltitudeM:               in.AltitudeM,
		PositionPrecision:       in.PositionPrecision,
		SourceKind:              source,
		SourceChannel:           channel,
		MQTTUploaderNodeID:      mqttUploaderNodeID,
		MQTTUploaderDisplayName: s.resolveNodeDisplayName(ctx, mqttUploaderNodeID),
		ReportedAt:              evt.Timestamp,
		ObservedAt:              now,
		UpdatedAt:               now,
	}
	if err := s.store.UpsertPosition(ctx, p); err != nil {
		s.log.Error("upsert position failed", "node_id", evt.NodeID, "err", err)

		return false
	}
	s.emitter.Emit(domain.RealtimeEvent{Type: "node.position", TS: now, Payload: p})

	return true
}

func (s *Service) handleTelemetry(ctx context.Context, evt meshtastic.ParsedEvent, channel, mqttUploaderNodeID string, now time.Time) bool {
	in := evt.Telemetry
	t := domain.NodeTelemetrySnapshot{NodeID: evt.NodeID, SourceChannel: channel, MQTTUploaderNodeID: mqttUploaderNodeID, MQTTUploaderDisplayName: s.resolveNodeDisplayName(ctx, mqttUploaderNodeID), ReportedAt: evt.Timestamp, ObservedAt: now, UpdatedAt: now}
	t.Power.Voltage = in.Power.Voltage
	t.Power.BatteryLevel = in.Power.BatteryLevel
	t.Environment.TemperatureC = in.Environment.TemperatureC
	t.Environment.Humidity = in.Environment.Humidity
	t.Environment.PressureHpa = in.Environment.PressureHpa
	t.AirQuality.PM25 = in.AirQuality.PM25
	t.AirQuality.PM10 = in.AirQuality.PM10
	t.AirQuality.CO2 = in.AirQuality.CO2
	t.AirQuality.IAQ = in.AirQuality.IAQ
	merged, err := s.store.MergeTelemetry(ctx, t)
	if err != nil {
		s.log.Error("merge telemetry failed", "node_id", evt.NodeID, "err", err)

		return false
	}
	s.emitter.Emit(domain.RealtimeEvent{Type: "node.telemetry", TS: now, Payload: merged})

	return true
}

func (s *Service) handleMapReport(ctx context.Context, evt meshtastic.ParsedEvent, mqttUploaderNodeID string, now time.Time) bool {
	if evt.MapReport == nil {
		return false
	}
	ok := true
	ev := evt
	ev.NodeInfo = &meshtastic.NodeInfoPayload{
		LongName:               evt.MapReport.LongName,
		ShortName:              evt.MapReport.ShortName,
		Role:                   evt.MapReport.Role,
		BoardModel:             evt.MapReport.BoardModel,
		FirmwareVersion:        evt.MapReport.FirmwareVersion,
		LoRaRegion:             evt.MapReport.LoRaRegion,
		ModemPreset:            evt.MapReport.ModemPreset,
		HasDefaultChannel:      boolPtr(evt.MapReport.HasDefaultChannel),
		HasOptedReportLocation: boolPtr(evt.MapReport.HasOptedReportLocation),
		NeighborNodesCount:     evt.MapReport.NeighborNodesCount,
	}
	if !s.handleNodeInfo(ctx, ev, now) {
		ok = false
	}
	ev.Position = &meshtastic.PositionPayload{
		Latitude:          evt.MapReport.Latitude,
		Longitude:         evt.MapReport.Longitude,
		AltitudeM:         evt.MapReport.AltitudeM,
		PositionPrecision: evt.MapReport.PositionPrecision,
	}
	if !s.handlePosition(ctx, ev, "", mqttUploaderNodeID, now, domain.PositionSourceMapReport) {
		ok = false
	}

	return ok
}

func (s *Service) emitSystemNodeDiscovered(ctx context.Context, nodeID, channel string, now time.Time) {
	ce := domain.ChatEvent{
		EventType:   domain.ChatEventSystem,
		ChannelName: channel,
		NodeID:      nodeID,
		SystemCode:  domain.ChatSystemNodeDiscovered,
		MessageTime: now,
		ObservedAt:  now,
		CreatedAt:   now,
	}
	id, err := s.store.InsertChatEvent(ctx, ce)
	if err != nil {
		s.log.Error("insert system event failed", "node_id", nodeID, "err", err)

		return
	}
	ce.ID = id
	s.populateChatDisplay(ctx, &ce)
	s.log.Debug("emit chat system event", "node_id", nodeID, "channel", channel, "system_code", ce.SystemCode)
	s.emitter.Emit(domain.RealtimeEvent{Type: "chat.system", TS: now, Payload: ce})
}

func (s *Service) populateChatDisplay(ctx context.Context, ce *domain.ChatEvent) {
	if ce == nil || ce.NodeID == "" {
		return
	}

	name, err := s.store.ResolveNodeDisplay(ctx, ce.NodeID)
	if err != nil {
		s.log.Debug("resolve chat node display failed", "node_id", ce.NodeID, "err", err)
		ce.NodeDisplay = ce.NodeID

		return
	}
	ce.NodeDisplay = name
	if ce.MQTTUploaderNodeID == "" {
		return
	}
	name, err = s.store.ResolveNodeDisplay(ctx, ce.MQTTUploaderNodeID)
	if err != nil {
		s.log.Debug("resolve chat mqtt uploader display failed", "node_id", ce.MQTTUploaderNodeID, "err", err)
		ce.MQTTUploaderDisplayName = ce.MQTTUploaderNodeID

		return
	}
	ce.MQTTUploaderDisplayName = name
}

func boolPtr(v bool) *bool {
	return &v
}
