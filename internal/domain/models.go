package domain

import "time"

// Node stores merged identity and liveness details for a Meshtastic node.
type Node struct {
	NodeID                 string     `json:"node_id"`
	NodeNum                *uint32    `json:"node_num,omitempty"`
	LongName               string     `json:"long_name,omitempty"`
	ShortName              string     `json:"short_name,omitempty"`
	Role                   string     `json:"role,omitempty"`
	BoardModel             string     `json:"board_model,omitempty"`
	FirmwareVersion        string     `json:"firmware_version,omitempty"`
	LoRaRegion             string     `json:"lora_region,omitempty"`
	LoRaFrequencyDesc      string     `json:"lora_frequency_desc,omitempty"`
	ModemPreset            string     `json:"modem_preset,omitempty"`
	HasDefaultChannel      *bool      `json:"has_default_channel,omitempty"`
	HasOptedReportLocation *bool      `json:"has_opted_report_location,omitempty"`
	NeighborNodesCount     *int       `json:"neighbor_nodes_count,omitempty"`
	MQTTGatewayCapable     *bool      `json:"mqtt_gateway_capable,omitempty"`
	FirstSeenAt            time.Time  `json:"first_seen_at"`
	LastSeenAnyEventAt     time.Time  `json:"last_seen_any_event_at"`
	LastSeenMQTTGatewayAt  *time.Time `json:"last_seen_mqtt_gateway_at,omitempty"`
	LastSeenPositionAt     *time.Time `json:"last_seen_position_at,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// PositionSourceKind identifies which ingest source produced a position update.
type PositionSourceKind string

// Position source values persisted with node positions.
const (
	PositionSourceChannel   PositionSourceKind = "channel_position"
	PositionSourceMapReport PositionSourceKind = "map_report"
)

// NodePosition stores the latest known position for a node.
type NodePosition struct {
	NodeID            string             `json:"node_id"`
	Latitude          float64            `json:"latitude"`
	Longitude         float64            `json:"longitude"`
	AltitudeM         *float64           `json:"altitude_m,omitempty"`
	PositionPrecision *uint32            `json:"position_precision,omitempty"`
	SourceKind        PositionSourceKind `json:"source_kind"`
	SourceChannel     string             `json:"source_channel,omitempty"`
	ReportedAt        *time.Time         `json:"reported_at,omitempty"`
	ObservedAt        time.Time          `json:"observed_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// TelemetrySectionPower stores power-related telemetry values.
type TelemetrySectionPower struct {
	Voltage      *float64 `json:"voltage,omitempty"`
	BatteryLevel *float64 `json:"battery_level,omitempty"`
}

// TelemetrySectionEnvironment stores environment sensor values.
type TelemetrySectionEnvironment struct {
	TemperatureC *float64 `json:"temperature_c,omitempty"`
	Humidity     *float64 `json:"humidity,omitempty"`
	PressureHpa  *float64 `json:"pressure_hpa,omitempty"`
}

// TelemetrySectionAirQuality stores air quality sensor values.
type TelemetrySectionAirQuality struct {
	PM25 *float64 `json:"pm25,omitempty"`
	PM10 *float64 `json:"pm10,omitempty"`
	CO2  *float64 `json:"co2,omitempty"`
	IAQ  *float64 `json:"iaq,omitempty"`
}

// NodeTelemetrySnapshot stores merged telemetry readings for a node.
type NodeTelemetrySnapshot struct {
	NodeID        string                      `json:"node_id"`
	Power         TelemetrySectionPower       `json:"power"`
	Environment   TelemetrySectionEnvironment `json:"environment"`
	AirQuality    TelemetrySectionAirQuality  `json:"air_quality"`
	SourceChannel string                      `json:"source_channel,omitempty"`
	ReportedAt    *time.Time                  `json:"reported_at,omitempty"`
	ObservedAt    time.Time                   `json:"observed_at"`
	UpdatedAt     time.Time                   `json:"updated_at"`
}

// ChatEventType classifies chat entries as user message or system event.
type ChatEventType string

// Chat event types stored in chat history.
const (
	ChatEventMessage ChatEventType = "message"
	ChatEventSystem  ChatEventType = "system"
)

// ChatSystemCode identifies the system message subtype.
type ChatSystemCode string

// System chat event codes emitted by ingest.
const (
	ChatSystemNodeDiscovered ChatSystemCode = "node_discovered"
)

// ChatEvent stores message and system events in a unified timeline.
type ChatEvent struct {
	ID          int64          `json:"id"`
	EventType   ChatEventType  `json:"event_type"`
	ChannelName string         `json:"channel_name,omitempty"`
	NodeID      string         `json:"node_id,omitempty"`
	NodeDisplay string         `json:"node_display_name,omitempty"`
	SystemCode  ChatSystemCode `json:"system_code,omitempty"`
	MessageText string         `json:"message_text,omitempty"`
	MessageTime time.Time      `json:"message_time"`
	ReportedAt  *time.Time     `json:"reported_at,omitempty"`
	ObservedAt  time.Time      `json:"observed_at"`
	PacketID    *uint32        `json:"packet_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Stats is an operational snapshot exposed over API and websocket.
type Stats struct {
	KnownNodesCount  int       `json:"known_nodes_count"`
	OnlineNodesCount int       `json:"online_nodes_count"`
	WSClientsCount   int       `json:"ws_clients_count"`
	LastIngestAt     time.Time `json:"last_ingest_at,omitempty"`
}

// TopologySourceKind identifies the evidence source of a persisted mesh edge.
type TopologySourceKind string

// Topology edge source values persisted from mesh evidence.
const (
	TopologySourceNeighborInfo      TopologySourceKind = "neighbor_info"
	TopologySourceRoutingForward    TopologySourceKind = "routing_forward"
	TopologySourceRoutingReturn     TopologySourceKind = "routing_return"
	TopologySourceTracerouteForward TopologySourceKind = "traceroute_forward"
	TopologySourceTracerouteReturn  TopologySourceKind = "traceroute_return"
)

// Compact persisted topology source values.
const (
	TopologySourceNeighborInfoValue      = 1
	TopologySourceRoutingForwardValue    = 2
	TopologySourceRoutingReturnValue     = 3
	TopologySourceTracerouteForwardValue = 4
	TopologySourceTracerouteReturnValue  = 5
)

// Valid reports whether the topology source kind is supported.
func (k TopologySourceKind) Valid() bool {
	switch k {
	case TopologySourceNeighborInfo,
		TopologySourceRoutingForward,
		TopologySourceRoutingReturn,
		TopologySourceTracerouteForward,
		TopologySourceTracerouteReturn:
		return true
	default:
		return false
	}
}

// TopologySourceKindValue returns the compact integer value used in storage.
func TopologySourceKindValue(kind TopologySourceKind) (int, bool) {
	switch kind {
	case TopologySourceNeighborInfo:
		return TopologySourceNeighborInfoValue, true
	case TopologySourceRoutingForward:
		return TopologySourceRoutingForwardValue, true
	case TopologySourceRoutingReturn:
		return TopologySourceRoutingReturnValue, true
	case TopologySourceTracerouteForward:
		return TopologySourceTracerouteForwardValue, true
	case TopologySourceTracerouteReturn:
		return TopologySourceTracerouteReturnValue, true
	default:
		return 0, false
	}
}

// TopologySourceKindFromInt parses a persisted integer value to a known topology source.
func TopologySourceKindFromInt(v int) (TopologySourceKind, bool) {
	switch v {
	case TopologySourceNeighborInfoValue:
		return TopologySourceNeighborInfo, true
	case TopologySourceRoutingForwardValue:
		return TopologySourceRoutingForward, true
	case TopologySourceRoutingReturnValue:
		return TopologySourceRoutingReturn, true
	case TopologySourceTracerouteForwardValue:
		return TopologySourceTracerouteForward, true
	case TopologySourceTracerouteReturnValue:
		return TopologySourceTracerouteReturn, true
	default:
		return "", false
	}
}

// TopologyEdge stores the latest observed directional link between two nodes.
type TopologyEdge struct {
	SourceKind                   TopologySourceKind `json:"source_kind"`
	ChannelName                  string             `json:"channel_name,omitempty"`
	FromNodeID                   string             `json:"from_node_id"`
	ToNodeID                     string             `json:"to_node_id"`
	ReportedByNodeID             string             `json:"reported_by_node_id,omitempty"`
	Inferred                     bool               `json:"inferred,omitempty"`
	SNR                          *float64           `json:"snr,omitempty"`
	NeighborLastRXAt             *time.Time         `json:"neighbor_last_rx_at,omitempty"`
	NeighborBroadcastIntervalSec *uint32            `json:"neighbor_broadcast_interval_secs,omitempty"`
	FirstObservedAt              time.Time          `json:"first_observed_at"`
	LastObservedAt               time.Time          `json:"last_observed_at"`
	LastReportedAt               *time.Time         `json:"last_reported_at,omitempty"`
	UpdatedAt                    time.Time          `json:"updated_at"`
}

// LogEventKind is a compact numeric identifier of a non-chat mesh activity event.
type LogEventKind uint8

// Log event kind values and UI titles.
const (
	LogEventKindMapReportValue        LogEventKind = 1
	LogEventKindNodeInfoValue         LogEventKind = 2
	LogEventKindPositionValue         LogEventKind = 3
	LogEventKindTelemetryValue        LogEventKind = 4
	LogEventKindTracerouteValue       LogEventKind = 5
	LogEventKindNeighborInfoValue     LogEventKind = 6
	LogEventKindRoutingValue          LogEventKind = 7
	LogEventKindOtherPortnumValue     LogEventKind = 8
	LogEventKindUnknownEncryptedValue LogEventKind = 9
	LogEventKindRangeTestValue        LogEventKind = 10
	LogEventKindPKIValue              LogEventKind = 11
)

// Human-friendly titles for compact log event kind values.
const (
	LogEventKindMapReportTitle        = "Map report"
	LogEventKindNodeInfoTitle         = "Node info"
	LogEventKindPositionTitle         = "Position"
	LogEventKindTelemetryTitle        = "Telemetry"
	LogEventKindTracerouteTitle       = "Traceroute"
	LogEventKindNeighborInfoTitle     = "Neighbor info"
	LogEventKindRoutingTitle          = "Routing"
	LogEventKindOtherPortnumTitle     = "Other app packet"
	LogEventKindUnknownEncryptedTitle = "Encrypted (undecryptable)"
	LogEventKindRangeTestTitle        = "Range test"
	LogEventKindPKITitle              = "PKI"
)

// LogEventKindTitle returns human-ready event kind title.
func LogEventKindTitle(kind LogEventKind) string {
	switch kind {
	case LogEventKindMapReportValue:
		return LogEventKindMapReportTitle
	case LogEventKindNodeInfoValue:
		return LogEventKindNodeInfoTitle
	case LogEventKindPositionValue:
		return LogEventKindPositionTitle
	case LogEventKindTelemetryValue:
		return LogEventKindTelemetryTitle
	case LogEventKindTracerouteValue:
		return LogEventKindTracerouteTitle
	case LogEventKindNeighborInfoValue:
		return LogEventKindNeighborInfoTitle
	case LogEventKindRoutingValue:
		return LogEventKindRoutingTitle
	case LogEventKindOtherPortnumValue:
		return LogEventKindOtherPortnumTitle
	case LogEventKindUnknownEncryptedValue:
		return LogEventKindUnknownEncryptedTitle
	case LogEventKindRangeTestValue:
		return LogEventKindRangeTestTitle
	case LogEventKindPKIValue:
		return LogEventKindPKITitle
	default:
		return "Unknown"
	}
}

// LogEventKindFromInt parses a persisted integer value to a known log kind.
func LogEventKindFromInt(v int) (LogEventKind, bool) {
	switch v {
	case int(LogEventKindMapReportValue):
		return LogEventKindMapReportValue, true
	case int(LogEventKindNodeInfoValue):
		return LogEventKindNodeInfoValue, true
	case int(LogEventKindPositionValue):
		return LogEventKindPositionValue, true
	case int(LogEventKindTelemetryValue):
		return LogEventKindTelemetryValue, true
	case int(LogEventKindTracerouteValue):
		return LogEventKindTracerouteValue, true
	case int(LogEventKindNeighborInfoValue):
		return LogEventKindNeighborInfoValue, true
	case int(LogEventKindRoutingValue):
		return LogEventKindRoutingValue, true
	case int(LogEventKindOtherPortnumValue):
		return LogEventKindOtherPortnumValue, true
	case int(LogEventKindUnknownEncryptedValue):
		return LogEventKindUnknownEncryptedValue, true
	case int(LogEventKindRangeTestValue):
		return LogEventKindRangeTestValue, true
	case int(LogEventKindPKIValue):
		return LogEventKindPKIValue, true
	default:
		return 0, false
	}
}

// LogEvent is a compact persisted row of mesh activity for Log tab.
type LogEvent struct {
	ID         int64          `json:"id"`
	ObservedAt time.Time      `json:"observed_at"`
	NodeID     string         `json:"node_id,omitempty"`
	EventKind  LogEventKind   `json:"event_kind_value"`
	Encrypted  bool           `json:"encrypted"`
	Channel    string         `json:"channel_name,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// LogEventView is API-ready row enriched with titles and display name fallback.
type LogEventView struct {
	ID             int64          `json:"id"`
	ObservedAt     time.Time      `json:"observed_at"`
	NodeID         string         `json:"node_id,omitempty"`
	NodeDisplay    string         `json:"node_display_name,omitempty"`
	EventKindValue LogEventKind   `json:"event_kind_value"`
	EventKindTitle string         `json:"event_kind_title"`
	Encrypted      bool           `json:"encrypted"`
	ChannelName    *string        `json:"channel_name"`
	Details        map[string]any `json:"details,omitempty"`
}

// LogEventQuery controls log-list pagination and filtering.
type LogEventQuery struct {
	Limit      int
	BeforeID   int64
	EventKinds []LogEventKind
	Channel    string
	NodeID     string
}

// ActivityBucket contains packet counts for one complete activity time bucket.
type ActivityBucket struct {
	BucketStart  time.Time `json:"bucket_start"`
	TextMessages int       `json:"text_messages"`
	PKI          int       `json:"pki"`
	NodeInfo     int       `json:"node_info"`
	Telemetry    int       `json:"telemetry"`
	NeighborInfo int       `json:"neighbor_info"`
	RangeTest    int       `json:"range_test"`
	Traceroute   int       `json:"traceroute"`
}

// ActivityQuery defines an aligned, complete-bucket activity query.
type ActivityQuery struct {
	Start  time.Time
	End    time.Time
	Bucket time.Duration
}
