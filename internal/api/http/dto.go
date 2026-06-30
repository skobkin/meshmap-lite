package httpapi

import (
	"time"

	"meshmap-lite/internal/mqttclient"
)

type healthStatusPayload struct {
	Status string `json:"status"`
}

type errorPayload struct {
	Error string `json:"error"`
}

type metaPayload struct {
	AppName               string                 `json:"app_name"`
	Version               string                 `json:"version"`
	WebsocketPath         string                 `json:"websocket_path"`
	ChatEnabled           bool                   `json:"chat_enabled"`
	DefaultChatChannel    string                 `json:"default_chat_channel"`
	ShowRecentMessages    int                    `json:"show_recent_messages"`
	LogLiveUpdates        bool                   `json:"log_live_updates"`
	LogPageSizeDefault    int                    `json:"log_page_size_default"`
	DisconnectedThreshold string                 `json:"disconnected_threshold"`
	InfoAvailable         bool                   `json:"info_available"`
	InfoSourceHash        string                 `json:"info_source_hash,omitempty"`
	UpdateCheckAvailable  bool                   `json:"update_check_available"`
	UpdateCheckSources    []*updateSourceSummary `json:"update_check_sources,omitempty"`
	Map                   metaMapPayload         `json:"map"`
	Relevance             metaRelevancePayload   `json:"relevance"`
}

type infoPayload struct {
	Format     string `json:"format"`
	SourceHash string `json:"source_hash"`
	Content    string `json:"content"`
}

type updatesPayload struct {
	Format     string               `json:"format"`
	Source     string               `json:"source"`
	SourceHash string               `json:"source_hash"`
	Releases   []updateReleaseEntry `json:"releases"`
}

// updateSourceSummary is the per-source summary embedded in the meta
// response. It carries only release metadata (no markdown bodies) so
// the page-load payload stays light; full bodies are fetched on demand
// from /api/v1/updates.
type updateSourceSummary struct {
	Name            string                       `json:"name"`
	Label           string                       `json:"label"`
	SourceHash      string                       `json:"source_hash,omitempty"`
	CurrentVersion  string                       `json:"current_version,omitempty"`
	LatestVersion   string                       `json:"latest_version,omitempty"`
	UpdateAvailable bool                         `json:"update_available"`
	ReleasesPageURL string                       `json:"releases_page_url,omitempty"`
	Releases        []updateReleaseMetadataEntry `json:"releases"`
}

type updateReleaseMetadataEntry struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
}

type updateReleaseEntry struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url,omitempty"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
}

type metaMapPayload struct {
	Clustering           bool                   `json:"clustering"`
	TopologyCacheTTL     string                 `json:"topology_cache_ttl"`
	PrecisionCirclesMode string                 `json:"precision_circles_mode"`
	DefaultView          metaDefaultViewPayload `json:"default_view"`
}

type metaRelevancePayload struct {
	TelemetryMaxAge        string `json:"telemetry_max_age"`
	TopologyEvidenceMaxAge string `json:"topology_evidence_max_age"`
	MapPositionMaxAge      string `json:"map_position_max_age"`
}

type metaDefaultViewPayload struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zoom      int     `json:"zoom"`
}

type channelPayload struct {
	Name        string `json:"name"`
	ChatEnabled bool   `json:"chat_enabled"`
	IsPrimary   bool   `json:"is_primary"`
}

type heartbeatPayload struct {
	Status               string                      `json:"status"`
	MQTTConnectionStatus mqttclient.ConnectionStatus `json:"mqtt_connection_status"`
}

type activityPayload struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Periods     []activityPeriodPayload `json:"periods"`
}

type activityPeriodPayload struct {
	Key     string                  `json:"key"`
	Title   string                  `json:"title"`
	Window  string                  `json:"window"`
	Bucket  string                  `json:"bucket"`
	Buckets []activityBucketPayload `json:"buckets"`
}

type activityBucketPayload struct {
	BucketStart  time.Time `json:"bucket_start"`
	TextMessages int       `json:"text_messages"`
	PKI          int       `json:"pki"`
	NodeInfo     int       `json:"node_info"`
	Telemetry    int       `json:"telemetry"`
	NeighborInfo int       `json:"neighbor_info"`
	RangeTest    int       `json:"range_test"`
	Traceroute   int       `json:"traceroute"`
}

// firmwareSnapshotPayload is the response body of
// GET /api/v1/stats/firmware.
type firmwareSnapshotPayload struct {
	GeneratedAt time.Time `json:"generated_at"`
	// CacheTtlSeconds echoes the server's resolved cache TTL for this
	// endpoint (the configured `web.stats.software.snapshot_cache_ttl`,
	// normalized to a positive integer). Clients use it as the polling
	// cadence so an operator shortening the server-side TTL actually
	// picks up fresher data, instead of the client remaining stale on
	// its hard-coded interval.
	CacheTtlSeconds       int                      `json:"cache_ttl_seconds"`
	TotalNodesWithVersion int                      `json:"total_nodes_with_version"`
	Versions              []firmwareVersionPayload `json:"versions"`
}

type firmwareVersionPayload struct {
	Version    string    `json:"version"`
	Count      int       `json:"count"`
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`
}

// firmwareHistoryPayload is the response body of
// GET /api/v1/stats/firmware/history.
//
// VersionsByWeek[i][j] is the count of devices on Versions[i] at week
// index j (j=0 is the oldest week, j=Weeks-1 is the newest). Missing
// weeks are zero-filled so the chart's x-axis stays contiguous.
//
// WeekStarts is the same length as VersionsByWeek's inner axis. Each
// entry is the Monday 00:00 UTC of that column, RFC3339-encoded, in
// oldest-first order. The server resolves these from its own week
// boundary math; the client MUST use them for week-aligned display
// (tooltip labels, axis ticks) instead of re-deriving from
// `generated_at` and `weeks` — re-derivation drifts across week
// boundaries when the response is cached and the browser's clock has
// crossed a Monday.
type firmwareHistoryPayload struct {
	GeneratedAt time.Time `json:"generated_at"`
	// CacheTtlSeconds echoes the server's resolved cache TTL for this
	// endpoint (the configured `web.stats.software.history_cache_ttl`,
	// normalized to a positive integer). Clients use it as the polling
	// cadence so an operator shortening the server-side TTL actually
	// picks up fresher data, instead of the client remaining stale on
	// its hard-coded interval.
	CacheTtlSeconds int         `json:"cache_ttl_seconds"`
	Weeks           int         `json:"weeks"`
	Top             int         `json:"top"`
	Versions        []string    `json:"versions"`
	VersionsByWeek  [][]int     `json:"versions_by_week"`
	WeekStarts      []time.Time `json:"week_starts"`
}

// hardwareSnapshotPayload is the response body of GET /api/v1/stats/hardware.
// It mirrors firmwareSnapshotPayload; see that type's doc for the TTL contract.
type hardwareSnapshotPayload struct {
	GeneratedAt time.Time `json:"generated_at"`
	// CacheTtlSeconds echoes the server's resolved cache TTL for this endpoint
	// (the configured `web.stats.hardware.snapshot_cache_ttl`, normalized to a
	// positive integer). Clients use it as the polling cadence.
	CacheTtlSeconds     int                    `json:"cache_ttl_seconds"`
	TotalNodesWithModel int                    `json:"total_nodes_with_model"`
	Models              []hardwareModelPayload `json:"models"`
}

type hardwareModelPayload struct {
	Model      string    `json:"model"`
	Count      int       `json:"count"`
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`
}

// hardwareHistoryPayload is the response body of
// GET /api/v1/stats/hardware/history. It mirrors firmwareHistoryPayload; see
// that type's doc for the axis/ordering and CacheTtlSeconds contracts.
// ModelsByWeek[i][j] is the count of devices on Models[i] at week index j
// (j=0 is the oldest week, j=Weeks-1 is the newest). Missing weeks are
// zero-filled so the chart's x-axis stays contiguous.
type hardwareHistoryPayload struct {
	GeneratedAt     time.Time   `json:"generated_at"`
	CacheTtlSeconds int         `json:"cache_ttl_seconds"`
	Weeks           int         `json:"weeks"`
	Top             int         `json:"top"`
	Models          []string    `json:"models"`
	ModelsByWeek    [][]int     `json:"models_by_week"`
	WeekStarts      []time.Time `json:"week_starts"`
}
