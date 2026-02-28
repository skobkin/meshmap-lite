package meshtastic

import (
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
	ParsedOtherPortnum     ParsedKind = "other_portnum"
	ParsedUnknownEncrypted ParsedKind = "unknown_encrypted"
)

// ParsedEvent is a normalized decoded payload produced by parser.
type ParsedEvent struct {
	Kind       ParsedKind
	NodeID     string
	PacketID   uint32
	Portnum    generated.PortNum
	Format     string
	Encrypted  bool
	Decrypted  bool
	Timestamp  *time.Time
	Chat       *ChatPayload
	NodeInfo   *NodeInfoPayload
	Position   *PositionPayload
	Telemetry  *TelemetryPayload
	MapReport  *MapReportPayload
	Traceroute *TraceroutePayload
	Neighbor   *NeighborInfoPayload
	Routing    *RoutingPayload
	Other      *OtherPortnumPayload
}

// ChatPayload contains decoded text message fields.
type ChatPayload struct {
	Text   string `json:"text"`
	Sender string `json:"sender"`
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
	NodeID            string `json:"node_id,omitempty"`
	NeighborsCount    int    `json:"neighbors_count"`
	BroadcastInterval uint32 `json:"broadcast_interval_secs,omitempty"`
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

// OtherPortnumPayload carries fallback details for known-but-unhandled app packets.
type OtherPortnumPayload struct {
	PortnumValue int32  `json:"portnum_value"`
	PortnumName  string `json:"portnum_name"`
}
