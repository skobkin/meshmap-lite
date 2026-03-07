package ingest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"meshmap-lite/internal/dedup"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
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
	logAllowed := s.allowLogEvent(topicInfo.Kind, channel, evt.Kind)
	tracerouteDecision := tracerouteLogDecision{}
	if logAllowed {
		tracerouteDecision = s.tracerouteLogDecision(evt, channel, now)
	}
	if logEvent, ok := s.logEventFromParsed(evt, channel, now); ok && logAllowed && !tracerouteDecision.suppressPacketLog {
		s.persistLogEvent(ctx, logEvent)
	}
	if logAllowed {
		for _, lifecycleEvent := range tracerouteDecision.lifecycleEvents {
			s.persistLogEvent(ctx, lifecycleEvent)
		}
	}
	if !s.allowEvent(channel, evt.Kind) {
		s.log.Debug("skip packet by channel policy", "channel", channel, "kind", evt.Kind, "node_id", evt.NodeID)

		return
	}
	if evt.NodeID == "" {
		return
	}

	node := domain.Node{NodeID: evt.NodeID, FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now}
	if topicInfo.IsFromMQTT {
		node.LastSeenMQTTGatewayAt = &now
	}
	if created, err := s.store.UpsertNode(ctx, node); err != nil {
		s.log.Error("upsert node failed", "node_id", evt.NodeID, "err", err)

		return
	} else if created {
		s.log.Info("new node discovered", "node_id", evt.NodeID, "channel", channel)
		s.emitSystemNodeDiscovered(ctx, evt.NodeID, channel, now)
	}

	switch evt.Kind {
	case meshtastic.ParsedChat:
		if s.handleChat(ctx, evt, channel, now) {
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
		if s.handlePosition(ctx, evt, channel, now, domain.PositionSourceChannel) {
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
		if s.handleTelemetry(ctx, evt, channel, now) {
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
		if s.handleMapReport(ctx, evt, now) {
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
		ID:             id,
		ObservedAt:     logEvent.ObservedAt,
		NodeID:         logEvent.NodeID,
		NodeDisplay:    s.resolveNodeDisplayName(ctx, logEvent.NodeID),
		EventKindValue: logEvent.EventKind,
		EventKindTitle: domain.LogEventKindTitle(logEvent.EventKind),
		Encrypted:      logEvent.Encrypted,
		Details:        logEvent.Details,
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

func (s *Service) logEventFromParsed(evt meshtastic.ParsedEvent, channel string, now time.Time) (domain.LogEvent, bool) {
	e := domain.LogEvent{
		ObservedAt: now,
		NodeID:     evt.NodeID,
		Encrypted:  evt.Encrypted,
		Channel:    channel,
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
	case meshtastic.ParsedOtherPortnum:
		e.EventKind = domain.LogEventKindOtherPortnumValue
		if evt.Other != nil {
			e.Details = map[string]any{
				"portnum_value": evt.Other.PortnumValue,
				"portnum_name":  evt.Other.PortnumName,
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

func (s *Service) allowLogEvent(topicKind meshtastic.TopicKind, channel string, kind meshtastic.ParsedKind) bool {
	if kind == meshtastic.ParsedMapReport || topicKind == meshtastic.TopicKindMapReport {
		return s.cfg.MapReports.Enabled
	}
	_, ok := s.cfg.Channels[channel]

	return ok
}

func (s *Service) allowEvent(channel string, kind meshtastic.ParsedKind) bool {
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

func (s *Service) handleChat(ctx context.Context, evt meshtastic.ParsedEvent, channel string, now time.Time) bool {
	ce := domain.ChatEvent{EventType: domain.ChatEventMessage, ChannelName: channel, NodeID: evt.NodeID, MessageText: evt.Chat.Text, MessageTime: now, ReportedAt: evt.Timestamp, ObservedAt: now, CreatedAt: now}
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

func (s *Service) handlePosition(ctx context.Context, evt meshtastic.ParsedEvent, channel string, now time.Time, source domain.PositionSourceKind) bool {
	in := evt.Position
	p := domain.NodePosition{
		NodeID:            evt.NodeID,
		Latitude:          in.Latitude,
		Longitude:         in.Longitude,
		AltitudeM:         in.AltitudeM,
		PositionPrecision: in.PositionPrecision,
		SourceKind:        source,
		SourceChannel:     channel,
		ReportedAt:        evt.Timestamp,
		ObservedAt:        now,
		UpdatedAt:         now,
	}
	if err := s.store.UpsertPosition(ctx, p); err != nil {
		s.log.Error("upsert position failed", "node_id", evt.NodeID, "err", err)

		return false
	}
	s.emitter.Emit(domain.RealtimeEvent{Type: "node.position", TS: now, Payload: p})

	return true
}

func (s *Service) handleTelemetry(ctx context.Context, evt meshtastic.ParsedEvent, channel string, now time.Time) bool {
	in := evt.Telemetry
	t := domain.NodeTelemetrySnapshot{NodeID: evt.NodeID, SourceChannel: channel, ReportedAt: evt.Timestamp, ObservedAt: now, UpdatedAt: now}
	t.Power.Voltage = in.Power.Voltage
	t.Power.BatteryLevel = in.Power.BatteryLevel
	t.Environment.TemperatureC = in.Environment.TemperatureC
	t.Environment.Humidity = in.Environment.Humidity
	t.Environment.PressureHpa = in.Environment.PressureHpa
	t.AirQuality.PM25 = in.AirQuality.PM25
	t.AirQuality.PM10 = in.AirQuality.PM10
	t.AirQuality.CO2 = in.AirQuality.CO2
	t.AirQuality.IAQ = in.AirQuality.IAQ
	if err := s.store.MergeTelemetry(ctx, t); err != nil {
		s.log.Error("merge telemetry failed", "node_id", evt.NodeID, "err", err)

		return false
	}
	s.emitter.Emit(domain.RealtimeEvent{Type: "node.telemetry", TS: now, Payload: t})

	return true
}

func (s *Service) handleMapReport(ctx context.Context, evt meshtastic.ParsedEvent, now time.Time) bool {
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
	if !s.handlePosition(ctx, ev, "", now, domain.PositionSourceMapReport) {
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
	s.log.Debug("emit chat system event", "node_id", nodeID, "channel", channel, "system_code", ce.SystemCode)
	s.emitter.Emit(domain.RealtimeEvent{Type: "chat.system", TS: now, Payload: ce})
}

func boolPtr(v bool) *bool {
	return &v
}
