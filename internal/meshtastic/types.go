package meshtastic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	generated "meshmap-lite/internal/meshtasticpb"
)

const positionScale = 1e-7

// ParsedKind classifies decoded Meshtastic packet payload types.
type ParsedKind string

// Parsed Meshtastic payload families.
const (
	ParsedChat             ParsedKind = "chat"
	ParsedNodeInfo         ParsedKind = "node_info"
	ParsedPosition         ParsedKind = "position"
	ParsedTelemetry        ParsedKind = "telemetry"
	ParsedMapReport        ParsedKind = "map_report"
	ParsedTraceroute       ParsedKind = "traceroute"
	ParsedNeighborInfo     ParsedKind = "neighbor_info"
	ParsedRouting          ParsedKind = "routing"
	ParsedPKI              ParsedKind = "pki"
	ParsedStoreForward     ParsedKind = "store_forward"
	ParsedOtherPortnum     ParsedKind = "other_portnum"
	ParsedUnknownEncrypted ParsedKind = "unknown_encrypted"
)

// ParsedEvent is a normalized decoded payload produced by parser.
type ParsedEvent struct {
	Kind         ParsedKind
	NodeID       string
	PacketID     uint32
	Portnum      generated.PortNum
	Format       string
	Encrypted    bool
	Decrypted    bool
	Timestamp    *time.Time
	HopStart     uint32
	HopLimit     uint32
	RxSNR        *float64
	Chat         *ChatPayload
	NodeInfo     *NodeInfoPayload
	Position     *PositionPayload
	Telemetry    *TelemetryPayload
	MapReport    *MapReportPayload
	Traceroute   *TraceroutePayload
	Neighbor     *NeighborInfoPayload
	Routing      *RoutingPayload
	PKI          *PKIPayload
	StoreForward *StoreForwardPayload
	Other        *OtherPortnumPayload
}

// ChatPayload contains decoded text message fields. When Emoji is true the
// Text field carries a single emoji character and the message is a reaction
// to the chat packet whose id equals ReplyID.
type ChatPayload struct {
	Text    string `json:"text"`
	Sender  string `json:"sender"`
	Emoji   bool   `json:"emoji,omitempty"`
	ReplyID uint32 `json:"reply_id,omitempty"`
}

// NodeInfoPayload contains decoded node identity and capabilities fields.
type NodeInfoPayload struct {
	LongName               string `json:"long_name"`
	ShortName              string `json:"short_name"`
	Role                   string `json:"role"`
	BoardModel             string `json:"board_model"`
	FirmwareVersion        string `json:"firmware_version"`
	LoRaRegion             string `json:"lora_region"`
	LoRaFrequencyDesc      string `json:"lora_frequency_desc"`
	ModemPreset            string `json:"modem_preset"`
	HasDefaultChannel      *bool  `json:"has_default_channel,omitempty"`
	HasOptedReportLocation *bool  `json:"has_opted_report_location,omitempty"`
	NeighborNodesCount     *int   `json:"neighbor_nodes_count"`
}

// PositionPayload contains decoded geolocation fields.
type PositionPayload struct {
	Latitude          float64  `json:"latitude"`
	Longitude         float64  `json:"longitude"`
	AltitudeM         *float64 `json:"altitude_m"`
	PositionPrecision *uint32  `json:"position_precision,omitempty"`
}

// TelemetryPayload contains decoded telemetry sections.
type TelemetryPayload struct {
	Power struct {
		Voltage      *float64 `json:"voltage"`
		BatteryLevel *float64 `json:"battery_level"`
		Current      *float64 `json:"current"`
	} `json:"power"`
	Environment struct {
		TemperatureC *float64 `json:"temperature_c"`
		Humidity     *float64 `json:"humidity"`
		PressureHpa  *float64 `json:"pressure_hpa"`
	} `json:"environment"`
	AirQuality struct {
		PM25 *float64 `json:"pm25"`
		PM10 *float64 `json:"pm10"`
		CO2  *float64 `json:"co2"`
		IAQ  *float64 `json:"iaq"`
	} `json:"air_quality"`
	// Utilization carries radio load metrics (channel / air-time utilization).
	// Source: Meshtastic DeviceMetrics and LocalStats.
	Utilization struct {
		ChUtil    *float64 `json:"ch_util"`
		AirUtilTx *float64 `json:"air_util_tx"`
	} `json:"utilization"`
	// Device carries device-level runtime metrics.
	// Source: Meshtastic DeviceMetrics and LocalStats (UptimeSeconds).
	Device struct {
		UptimeSeconds *uint32 `json:"uptime_seconds"`
	} `json:"device"`
}

// MapReportPayload contains decoded map report content.
type MapReportPayload struct {
	NodeID                 string   `json:"node_id"`
	LongName               string   `json:"long_name"`
	ShortName              string   `json:"short_name"`
	Role                   string   `json:"role"`
	BoardModel             string   `json:"board_model"`
	FirmwareVersion        string   `json:"firmware_version"`
	LoRaRegion             string   `json:"lora_region"`
	ModemPreset            string   `json:"modem_preset"`
	HasDefaultChannel      bool     `json:"has_default_channel"`
	HasOptedReportLocation bool     `json:"has_opted_report_location"`
	NeighborNodesCount     *int     `json:"neighbor_nodes_count"`
	Latitude               float64  `json:"latitude"`
	Longitude              float64  `json:"longitude"`
	AltitudeM              *float64 `json:"altitude_m"`
	PositionPrecision      *uint32  `json:"position_precision"`
}

// TraceroutePayload contains compact TRACEROUTE_APP details.
type TraceroutePayload struct {
	Role                string   `json:"role"`
	Status              string   `json:"status,omitempty"`
	RequestID           uint32   `json:"request_id,omitempty"`
	ReplyID             uint32   `json:"reply_id,omitempty"`
	FromNodeID          string   `json:"from,omitempty"`
	ToNodeID            string   `json:"to,omitempty"`
	Route               []string `json:"route,omitempty"`
	SnrTowards          []int32  `json:"snr_towards,omitempty"`
	RouteBack           []string `json:"route_back,omitempty"`
	SnrBack             []int32  `json:"snr_back,omitempty"`
	ForwardPath         []string `json:"forward_path,omitempty"`
	ReturnPath          []string `json:"return_path,omitempty"`
	InferredForwardPath bool     `json:"inferred_forward_path,omitempty"`
	InferredReturnPath  bool     `json:"inferred_return_path,omitempty"`
	InferredDirect      bool     `json:"inferred_direct,omitempty"`
	WantResponse        bool     `json:"want_response,omitempty"`
	HopStart            uint32   `json:"hop_start,omitempty"`
	HopLimit            uint32   `json:"hop_limit,omitempty"`
	Bitfield            uint32   `json:"bitfield,omitempty"`
}

// NeighborInfoPayload contains compact NEIGHBORINFO_APP details.
type NeighborInfoPayload struct {
	NodeID            string                 `json:"node_id,omitempty"`
	NeighborsCount    int                    `json:"neighbors_count"`
	BroadcastInterval uint32                 `json:"broadcast_interval_secs,omitempty"`
	Neighbors         []NeighborInfoNeighbor `json:"neighbors,omitempty"`
}

// NeighborInfoNeighbor stores one neighbor entry reported by a node.
type NeighborInfoNeighbor struct {
	NodeID                    string  `json:"node_id,omitempty"`
	SNR                       float32 `json:"snr"`
	LastRxTime                uint32  `json:"last_rx_time,omitempty"`
	NodeBroadcastIntervalSecs uint32  `json:"node_broadcast_interval_secs,omitempty"`
}

// RoutingPayload contains compact ROUTING_APP details.
type RoutingPayload struct {
	Variant       string   `json:"variant"`
	RequestID     uint32   `json:"request_id,omitempty"`
	FromNodeID    string   `json:"from,omitempty"`
	ToNodeID      string   `json:"to,omitempty"`
	Route         []string `json:"route,omitempty"`
	RouteBack     []string `json:"route_back,omitempty"`
	ErrorReason   string   `json:"error_reason,omitempty"`
	TracerouteRef bool     `json:"traceroute_ref,omitempty"`
}

// PKIPayload carries outer-header evidence for PKI-encrypted packets.
type PKIPayload struct {
	SenderNodeID      string `json:"sender_node_id,omitempty"`
	DestinationNodeID string `json:"destination_node_id,omitempty"`
	GatewayID         string `json:"gateway_id,omitempty"`
	TopicChannel      string `json:"topic_channel,omitempty"`
	EnvelopeChannelID string `json:"envelope_channel_id,omitempty"`
	PacketID          uint32 `json:"packet_id,omitempty"`
	Encrypted         bool   `json:"encrypted"`
	Decrypted         bool   `json:"decrypted"`
	PKIEncrypted      bool   `json:"pki_encrypted"`
	PayloadSizeBytes  int    `json:"payload_size_bytes,omitempty"`
	HopStart          uint32 `json:"hop_start,omitempty"`
	HopLimit          uint32 `json:"hop_limit,omitempty"`
	Priority          string `json:"priority,omitempty"`
}

// Routing payload variants.
const (
	RoutingVariantRequest = "route_request"
	RoutingVariantReply   = "route_reply"
	RoutingVariantError   = "error"
)

// OtherPortnumPayload carries fallback details for known-but-unhandled app packets.
type OtherPortnumPayload struct {
	PortnumValue int32  `json:"portnum_value"`
	PortnumName  string `json:"portnum_name"`
}

// StoreForwardRole is the typed role of a Store-and-Forward packet.
// Only two roles are defined by the meshtastic proto today ("router" /
// "client"); Unknown covers values published by newer firmware that we
// have not seen yet, so the unmarshaller can still ingest them without
// crashing and stash the original string in RawRole for later
// promotion.
type StoreForwardRole string

// Known StoreForwardRole values. The strings match the wire form so the
// enum can round-trip through JSON without a separate marshaller.
const (
	StoreForwardRoleRouter  StoreForwardRole = "router"
	StoreForwardRoleClient  StoreForwardRole = "client"
	StoreForwardRoleUnknown StoreForwardRole = "unknown"
)

// StoreForwardRRUnknown is the sentinel RR value used when a legacy
// JSON payload contains a RequestResponse name that is not in the
// pinned proto enum. We use -1 because proto3 enums are non-negative,
// so the value can never collide with a real enum entry.
const StoreForwardRRUnknown int32 = -1

// StoreForwardPayload contains decoded STORE_FORWARD_APP details. RR is the
// numeric value of the proto RequestResponse enum (see
// generated.StoreAndForward_*) — when a publisher ships a value the
// pinned proto does not know about, RR is set to StoreForwardRRUnknown
// (-1) and the original name is preserved in RawRR so the data is not
// lost. Role is derived from RR: "router" for values < 64, "client" for
// values >= 64, "unknown" for StoreForwardRRUnknown. Exactly one of
// Stats, History, Heartbeat, or Text is populated, matching the proto
// oneof.
type StoreForwardPayload struct {
	RR         int32                  `json:"rr"`
	Role       StoreForwardRole       `json:"-"`
	RawRR      string                 `json:"raw_rr,omitempty"`
	RawRole    StoreForwardRole       `json:"raw_role,omitempty"`
	FromNodeID string                 `json:"from,omitempty"`
	ToNodeID   string                 `json:"to,omitempty"`
	Stats      *StoreForwardStats     `json:"stats,omitempty"`
	History    *StoreForwardHistory   `json:"history,omitempty"`
	Heartbeat  *StoreForwardHeartbeat `json:"heartbeat,omitempty"`
	Text       string                 `json:"text,omitempty"`
}

// StoreForwardStats mirrors the ROUTER_STATS sub-payload.
type StoreForwardStats struct {
	MessagesTotal    uint32 `json:"messages_total"`
	MessagesSaved    uint32 `json:"messages_saved"`
	MessagesMax      uint32 `json:"messages_max"`
	UpTimeSeconds    uint32 `json:"up_time"`
	Requests         uint32 `json:"requests"`
	RequestsHistory  uint32 `json:"requests_history"`
	HeartbeatEnabled bool   `json:"heartbeat"`
	ReturnMax        uint32 `json:"return_max"`
	ReturnWindow     uint32 `json:"return_window"`
}

// StoreForwardHistory mirrors the ROUTER_HISTORY sub-payload header.
type StoreForwardHistory struct {
	HistoryMessages uint32 `json:"history_messages"`
	WindowMinutes   uint32 `json:"window"`
	LastRequest     uint32 `json:"last_request"`
}

// StoreForwardHeartbeat mirrors the ROUTER_HEARTBEAT sub-payload.
type StoreForwardHeartbeat struct {
	PeriodSeconds uint32 `json:"period"`
	Secondary     uint32 `json:"secondary"`
}

// UnmarshalJSON accepts RR as either the canonical integer value of the
// StoreAndForward_RequestResponse proto enum or a case-sensitive string
// form (e.g. "ROUTER_STATS", "CLIENT_HISTORY") for backward compatibility
// with publishers that still emit the legacy string form. Unknown enum
// names are preserved in RawRR and RR is set to StoreForwardRRUnknown
// (-1) so newer firmware does not crash older decoders. Role is derived
// from RR; an explicit `role` field on the wire is accepted but any
// value other than "router" or "client" lands in RawRole and the
// typed Role becomes StoreForwardRoleUnknown.
func (s *StoreForwardPayload) UnmarshalJSON(data []byte) error {
	type alias StoreForwardPayload
	var aux struct {
		alias
		RR        json.RawMessage `json:"rr"`
		Role      *string         `json:"role"`
		Text      json.RawMessage `json:"text"`
		TextBytes *uint32         `json:"text_bytes"`
	}
	aux.alias = alias(*s)
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("decode store_forward: %w", err)
	}
	*s = StoreForwardPayload(aux.alias)

	// Reset fields populated by the custom logic below so the alias
	// writeback does not leave stale values from a prior decode.
	s.RawRR = ""
	s.Role = ""
	s.RawRole = ""

	switch len(aux.RR) {
	case 0, 4:
		// Empty or "null" — leave RR at the zero value.
	default:
		var asInt int32
		if err := json.Unmarshal(aux.RR, &asInt); err == nil {
			s.RR = asInt
		} else {
			var asString string
			if err := json.Unmarshal(aux.RR, &asString); err != nil {
				return fmt.Errorf("decode store_forward.rr: %w", err)
			}
			value, ok := generated.StoreAndForward_RequestResponse_value[asString]
			if !ok {
				s.RR = StoreForwardRRUnknown
				s.RawRR = asString
			} else {
				s.RR = value
			}
		}
	}

	// Role: prefer an explicit wire value if present (legacy S&F
	// publishers may have set role independently of RR), otherwise
	// derive it. Anything other than the two known roles is preserved
	// in RawRole and surfaced as Unknown so we don't silently drop a
	// value a newer firmware has started sending.
	if aux.Role != nil {
		switch StoreForwardRole(*aux.Role) {
		case StoreForwardRoleRouter, StoreForwardRoleClient:
			s.Role = StoreForwardRole(*aux.Role)
		default:
			s.Role = StoreForwardRoleUnknown
			s.RawRole = StoreForwardRole(*aux.Role)
		}
	} else {
		s.Role = deriveRoleFromRR(s.RR)
	}

	switch {
	case aux.TextBytes != nil:
		s.Text = ""
	case len(aux.Text) > 0:
		var asString string
		if err := json.Unmarshal(aux.Text, &asString); err == nil {
			s.Text = asString
		} else {
			s.Text = strings.Trim(string(aux.Text), "\"")
		}
	}

	return nil
}

// deriveRoleFromRR maps the proto enum value to a StoreForwardRole.
// The StoreForwardRRUnknown sentinel maps to Unknown rather than the
// historical "default to router" behaviour so the renderer can show a
// distinct "unknown" label.
func deriveRoleFromRR(rr int32) StoreForwardRole {
	if rr == StoreForwardRRUnknown {
		return StoreForwardRoleUnknown
	}
	if rr >= 64 {
		return StoreForwardRoleClient
	}

	return StoreForwardRoleRouter
}
