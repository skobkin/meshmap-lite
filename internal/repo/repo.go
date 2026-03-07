package repo

import (
	"context"
	"time"

	"meshmap-lite/internal/domain"
)

// WriteStore defines persistence operations used by ingest.
type WriteStore interface {
	UpsertNode(ctx context.Context, node domain.Node) (created bool, err error)
	UpsertPosition(ctx context.Context, pos domain.NodePosition) error
	MergeTelemetry(ctx context.Context, snapshot domain.NodeTelemetrySnapshot) error
	InsertChatEvent(ctx context.Context, event domain.ChatEvent) (int64, error)
	InsertLogEvent(ctx context.Context, event domain.LogEvent) (int64, error)
}

// ReadStore defines query operations used by HTTP and other read APIs.
type ReadStore interface {
	GetMapNodes(ctx context.Context, hidePositionAfter time.Duration) ([]MapNode, error)
	ListNodes(ctx context.Context) ([]NodeSummary, error)
	GetNodeDetails(ctx context.Context, nodeID string) (NodeDetails, error)
	ListChatEvents(ctx context.Context, q ChatEventQuery) ([]domain.ChatEvent, error)
	ListLogEvents(ctx context.Context, q domain.LogEventQuery) ([]domain.LogEventView, error)
	Stats(ctx context.Context, disconnectedThreshold time.Duration) (domain.Stats, error)
}

// Store is the full repository surface implemented by storage adapters.
type Store interface {
	WriteStore
	ReadStore
}

// ChatEventQuery defines chat history list parameters.
type ChatEventQuery struct {
	Channel  string
	Limit    int
	BeforeID int64
}

// MapNode combines node identity with an optional latest position.
type MapNode struct {
	Node     domain.Node          `json:"node"`
	Position *domain.NodePosition `json:"position,omitempty"`
}

// NodeSummary is a compact record for node list views.
type NodeSummary struct {
	NodeID             string     `json:"node_id"`
	DisplayName        string     `json:"display_name"`
	LongName           string     `json:"long_name,omitempty"`
	ShortName          string     `json:"short_name,omitempty"`
	LastSeenAnyEventAt time.Time  `json:"last_seen_any_event_at"`
	LastSeenPositionAt *time.Time `json:"last_seen_position_at,omitempty"`
	LastSeenMQTTAt     *time.Time `json:"last_seen_mqtt_gateway_at,omitempty"`
	HasPosition        bool       `json:"has_position"`
	Role               string     `json:"role,omitempty"`
	BoardModel         string     `json:"board_model,omitempty"`
}

// NodeDetails is the full node details payload.
type NodeDetails struct {
	Node      domain.Node                   `json:"node"`
	Position  *domain.NodePosition          `json:"position,omitempty"`
	Telemetry *domain.NodeTelemetrySnapshot `json:"telemetry,omitempty"`
}
