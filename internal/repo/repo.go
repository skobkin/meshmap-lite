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
	MergeTelemetry(ctx context.Context, snapshot domain.NodeTelemetrySnapshot) (domain.NodeTelemetrySnapshot, error)
	UpsertTopologyEdges(ctx context.Context, edges []domain.TopologyEdge) error
	InsertChatEvent(ctx context.Context, event domain.ChatEvent) (int64, error)
	InsertLogEvent(ctx context.Context, event domain.LogEvent) (int64, error)
	ResolveNodeDisplay(ctx context.Context, nodeID string) (string, error)
}

// ReadStore defines query operations used by HTTP and other read APIs.
type ReadStore interface {
	GetMapNodes(ctx context.Context, q MapNodeQuery) ([]MapNode, error)
	ListNodes(ctx context.Context, q NodeListQuery) ([]NodeSummary, error)
	GetNodeDetails(ctx context.Context, q NodeDetailsQuery) (NodeDetails, error)
	ListTopologyEdges(ctx context.Context, q TopologyEdgeQuery) ([]domain.TopologyEdge, error)
	ListChatEvents(ctx context.Context, q ChatEventQuery) ([]domain.ChatEvent, error)
	ListLogEvents(ctx context.Context, q domain.LogEventQuery) ([]domain.LogEventView, error)
	ActivityBuckets(ctx context.Context, q domain.ActivityQuery) ([]domain.ActivityBucket, error)
	Stats(ctx context.Context, disconnectedThreshold time.Duration) (domain.Stats, error)
}

// MapNodeQuery defines map snapshot visibility cutoffs.
type MapNodeQuery struct {
	PositionObservedSince  time.Time
	TelemetryObservedSince time.Time
}

// NodeListQuery defines node-list visibility cutoffs.
type NodeListQuery struct {
	PositionObservedSince time.Time
}

// NodeDetailsQuery defines node-detail visibility cutoffs.
type NodeDetailsQuery struct {
	NodeID                 string
	PositionObservedSince  time.Time
	TelemetryObservedSince time.Time
	TopologyUpdatedSince   time.Time
}

// Store is the full repository surface implemented by storage adapters.
type Store interface {
	WriteStore
	ReadStore
}

// ChatEventQuery defines chat history list parameters.
type ChatEventQuery struct {
	Channel         string
	Limit           int
	BeforeID        int64
	ObservedSinceAt time.Time
}

// MapNode combines node identity with optional position and telemetry.
type MapNode struct {
	Node      domain.Node                   `json:"node"`
	Position  *domain.NodePosition          `json:"position,omitempty"`
	Telemetry *domain.NodeTelemetrySnapshot `json:"telemetry,omitempty"`
}

// NodeSummary is a compact record for node list views.
type NodeSummary struct {
	NodeID                      string     `json:"node_id"`
	DisplayName                 string     `json:"display_name"`
	LongName                    string     `json:"long_name,omitempty"`
	ShortName                   string     `json:"short_name,omitempty"`
	LastSeenAnyEventAt          time.Time  `json:"last_seen_any_event_at"`
	LastSeenPositionAt          *time.Time `json:"last_seen_position_at,omitempty"`
	LastSeenMQTTAt              *time.Time `json:"last_seen_mqtt_gateway_at,omitempty"`
	LastMQTTUploaderNodeID      string     `json:"last_mqtt_uploader_node_id,omitempty"`
	LastMQTTUploaderDisplayName string     `json:"last_mqtt_uploader_display_name,omitempty"`
	LastMQTTUploaderAt          *time.Time `json:"last_mqtt_uploader_at,omitempty"`
	HasPosition                 bool       `json:"has_position"`
	Role                        string     `json:"role,omitempty"`
	BoardModel                  string     `json:"board_model,omitempty"`
}

// NodeDetails is the full node details payload.
type NodeDetails struct {
	Node          domain.Node                   `json:"node"`
	Position      *domain.NodePosition          `json:"position,omitempty"`
	Telemetry     *domain.NodeTelemetrySnapshot `json:"telemetry,omitempty"`
	Neighbors     []NodeNeighbor                `json:"neighbors,omitempty"`
	PreviousNames []NodeNameHistory             `json:"previous_names,omitempty"`
}

// NodeNameHistory records one effective node name change.
type NodeNameHistory struct {
	PreviousLongName  string    `json:"previous_long_name,omitempty"`
	PreviousShortName string    `json:"previous_short_name,omitempty"`
	NewLongName       string    `json:"new_long_name,omitempty"`
	NewShortName      string    `json:"new_short_name,omitempty"`
	ChangedAt         time.Time `json:"changed_at"`
}

// NodeNeighbor is a collapsed topology view for one peer of the selected node.
type NodeNeighbor struct {
	NodeID                       string     `json:"node_id"`
	DisplayName                  string     `json:"display_name"`
	LongName                     string     `json:"long_name,omitempty"`
	ShortName                    string     `json:"short_name,omitempty"`
	HasPosition                  bool       `json:"has_position"`
	EvidenceKind                 string     `json:"evidence_kind"`
	SNR                          *float64   `json:"snr,omitempty"`
	ChannelName                  string     `json:"channel_name,omitempty"`
	ReportedByNodeID             string     `json:"reported_by_node_id,omitempty"`
	NeighborLastRXAt             *time.Time `json:"neighbor_last_rx_at,omitempty"`
	NeighborBroadcastIntervalSec *uint32    `json:"neighbor_broadcast_interval_secs,omitempty"`
	LastObservedAt               time.Time  `json:"last_observed_at"`
	LastReportedAt               *time.Time `json:"last_reported_at,omitempty"`
	UpdatedAt                    time.Time  `json:"updated_at"`
}

// TopologyEdgeQuery defines topology-edge list filters.
type TopologyEdgeQuery struct {
	NodeID       string
	Channel      string
	SourceKinds  []domain.TopologySourceKind
	UpdatedSince time.Time
	// Limit caps the number of returned rows. Zero means "no cap" (legacy
	// callers). Adapters that support limiting should treat positive values as
	// an exclusive bound and negative values as "no cap".
	Limit int
}
