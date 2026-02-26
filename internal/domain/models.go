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
	PositionSourceNodeInfo  PositionSourceKind = "nodeinfo_position"
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
