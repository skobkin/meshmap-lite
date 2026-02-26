package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"meshmap-lite/internal/config"
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
	cfg     config.Config
	store   repo.Store
	dedup   *dedup.Store
	emitter RealtimeEmitter
	log     *slog.Logger
}

// New constructs ingest service and configures parser channel keys.
func New(cfg config.Config, store repo.Store, dedupStore *dedup.Store, emitter RealtimeEmitter, log *slog.Logger) *Service {
	keys := make(map[string]string, len(cfg.Channels))
	for name, ch := range cfg.Channels {
		keys[name] = ch.PSK
	}
	meshtastic.ConfigureChannelKeys(keys)

	return &Service{cfg: cfg, store: store, dedup: dedupStore, emitter: emitter, log: log}
}

// HandleMessage processes one MQTT message through topic classification and ingest pipeline.
func (s *Service) HandleMessage(ctx context.Context, topic string, payload []byte) {
	now := time.Now().UTC()
	s.log.Debug("ingest mqtt payload", "topic", topic, "bytes", len(payload))
	topicInfo := meshtastic.ClassifyTopic(s.cfg.MQTT.RootTopic, s.cfg.MapReports.TopicSuffix, topic)
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
		s.log.Debug("drop payload without node_id", "topic", topic)

		return
	}
	if evt.PacketID > 0 {
		if s.dedup.Seen(fmt.Sprintf("%s:%d", evt.NodeID, evt.PacketID), now) {
			s.log.Debug("skip duplicated packet", "node_id", evt.NodeID, "packet_id", evt.PacketID)

			return
		}
	}
	channel := strings.TrimSpace(topicInfo.Channel)
	if !s.allowEvent(channel, evt.Kind) {
		s.log.Debug("skip packet by channel policy", "channel", channel, "kind", evt.Kind, "node_id", evt.NodeID)

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
	n := domain.Node{NodeID: evt.NodeID, LongName: in.LongName, ShortName: in.ShortName, Role: in.Role, BoardModel: in.BoardModel, FirmwareVersion: in.FirmwareVersion, LoRaRegion: in.LoRaRegion, LoRaFrequencyDesc: in.LoRaFrequencyDesc, ModemPreset: in.ModemPreset, NeighborNodesCount: in.NeighborNodesCount, FirstSeenAt: now, LastSeenAnyEventAt: now, UpdatedAt: now}
	if _, err := s.store.UpsertNode(ctx, n); err != nil {
		s.log.Error("upsert nodeinfo failed", "node_id", evt.NodeID, "err", err)

		return false
	}
	s.emitter.Emit(domain.RealtimeEvent{Type: "node.upsert", TS: now, Payload: n})

	return true
}

func (s *Service) handlePosition(ctx context.Context, evt meshtastic.ParsedEvent, channel string, now time.Time, source domain.PositionSourceKind) bool {
	in := evt.Position
	p := domain.NodePosition{NodeID: evt.NodeID, Latitude: in.Latitude, Longitude: in.Longitude, AltitudeM: in.AltitudeM, SourceKind: source, SourceChannel: channel, ReportedAt: evt.Timestamp, ObservedAt: now, UpdatedAt: now}
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
	ev := evt
	ev.Position = &meshtastic.PositionPayload{Latitude: evt.MapReport.Latitude, Longitude: evt.MapReport.Longitude, AltitudeM: evt.MapReport.AltitudeM}

	return s.handlePosition(ctx, ev, "", now, domain.PositionSourceMapReport)
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
