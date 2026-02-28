package meshtastic

import (
	"encoding/json"
	"fmt"
	"time"

	generated "meshmap-lite/internal/meshtasticpb"
)

func parseJSONFallback(kind TopicKind, payload []byte) (ParsedEvent, error) {
	var raw struct {
		Type       string              `json:"type"`
		NodeID     string              `json:"node_id"`
		PacketID   uint32              `json:"packet_id"`
		Portnum    int32               `json:"portnum"`
		Timestamp  *time.Time          `json:"timestamp"`
		Chat       ChatPayload         `json:"chat"`
		NodeInfo   NodeInfoPayload     `json:"node_info"`
		Position   PositionPayload     `json:"position"`
		Telemetry  TelemetryPayload    `json:"telemetry"`
		MapReport  MapReportPayload    `json:"map_report"`
		Traceroute TraceroutePayload   `json:"traceroute"`
		Neighbor   NeighborInfoPayload `json:"neighbor_info"`
		Routing    RoutingPayload      `json:"routing"`
		Other      OtherPortnumPayload `json:"other"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ParsedEvent{}, fmt.Errorf("decode payload: %w", err)
	}

	out := ParsedEvent{
		NodeID:    normalizeNodeID(raw.NodeID),
		PacketID:  raw.PacketID,
		Portnum:   generated.PortNum(raw.Portnum),
		Timestamp: raw.Timestamp,
		Format:    "json_fallback",
	}
	if kind == TopicKindMapReport || raw.Type == "map_report" {
		out.Kind = ParsedMapReport
		out.MapReport = &raw.MapReport
		if out.MapReport.NodeID == "" {
			out.MapReport.NodeID = out.NodeID
		}

		return out, nil
	}

	switch raw.Type {
	case "chat", "text_message":
		out.Kind = ParsedChat
		out.Chat = &raw.Chat
	case "node_info":
		out.Kind = ParsedNodeInfo
		out.NodeInfo = &raw.NodeInfo
	case "position":
		out.Kind = ParsedPosition
		out.Position = &raw.Position
	case "telemetry":
		out.Kind = ParsedTelemetry
		out.Telemetry = &raw.Telemetry
	case "traceroute":
		out.Kind = ParsedTraceroute
		out.Traceroute = &raw.Traceroute
	case "neighbor_info":
		out.Kind = ParsedNeighborInfo
		out.Neighbor = &raw.Neighbor
	case "routing":
		out.Kind = ParsedRouting
		out.Routing = &raw.Routing
	case "other_portnum":
		out.Kind = ParsedOtherPortnum
		out.Other = &raw.Other
	case "unknown_encrypted":
		out.Kind = ParsedUnknownEncrypted
		out.Encrypted = true
	default:
		return ParsedEvent{}, fmt.Errorf("unsupported event type %q", raw.Type)
	}

	return out, nil
}
