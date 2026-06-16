package config

import "time"

const (
	// DefaultWSHeartbeatInterval is the fallback websocket heartbeat interval.
	DefaultWSHeartbeatInterval = 30 * time.Second
	// DefaultWSStatsInterval is the fallback websocket stats emission interval.
	DefaultWSStatsInterval = 60 * time.Second
)

// Config is the root application configuration loaded from YAML and environment.
type Config struct {
	MQTT        MQTTConfig               `koanf:"mqtt" json:"mqtt"`
	Ingest      IngestConfig             `koanf:"ingest" json:"ingest"`
	Storage     StorageConfig            `koanf:"storage" json:"storage"`
	Channels    map[string]ChannelConfig `koanf:"channels" json:"channels"`
	Web         WebConfig                `koanf:"web" json:"web"`
	Logging     LoggingConfig            `koanf:"logging" json:"logging"`
	UpdateCheck UpdateCheckConfig        `koanf:"update_check" json:"update_check"`
}

// MQTTConfig contains MQTT broker and connection settings.
type MQTTConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	TLS      bool   `koanf:"tls"`
	ClientID string `koanf:"client_id"`
	Username string `koanf:"username"`
	//nolint:gosec // configuration object intentionally carries secret value.
	Password         string        `koanf:"password"`
	RootTopic        string        `koanf:"root_topic"`
	ProtocolVersion  string        `koanf:"protocol_version"`
	SubscribeQoS     byte          `koanf:"subscribe_qos"`
	CleanSession     bool          `koanf:"clean_session"`
	ReconnectTimeout time.Duration `koanf:"reconnect_timeout"`
	ConnectTimeout   time.Duration `koanf:"connect_timeout"`
	Keepalive        time.Duration `koanf:"keepalive"`
}

// IngestConfig controls ingest-side correlation and synthesis policies.
type IngestConfig struct {
	MapReports MapReportsConfig       `koanf:"map_reports" json:"map_reports"`
	Traceroute TracerouteIngestConfig `koanf:"traceroute" json:"traceroute"`
}

// TracerouteIngestConfig bounds ingest-side traceroute lifecycle tracking.
type TracerouteIngestConfig struct {
	Timeout        time.Duration `koanf:"timeout" json:"timeout"`
	MaxEntries     int           `koanf:"max_entries" json:"max_entries"`
	FinalRetention time.Duration `koanf:"final_retention" json:"final_retention"`
}

// StorageConfig configures KV and SQL backends.
type StorageConfig struct {
	KV  KVConfig  `koanf:"kv"`
	SQL SQLConfig `koanf:"sql"`
}

// KVConfig configures the in-memory dedup key-value store.
type KVConfig struct {
	Driver string        `koanf:"driver"`
	Size   int           `koanf:"size"`
	TTL    time.Duration `koanf:"ttl"`
}

// SQLConfig configures the relational storage backend.
type SQLConfig struct {
	Driver            string `koanf:"driver"`
	URL               string `koanf:"url"`
	AutoMigrate       bool   `koanf:"auto_migrate"`
	LogMaxRows        int    `koanf:"log_max_rows"`
	LogPruneBatchRows int    `koanf:"log_prune_batch_rows"`
}

// MapReportsConfig controls optional Meshtastic map report ingest.
type MapReportsConfig struct {
	Enabled     bool   `koanf:"enabled"`
	TopicSuffix string `koanf:"topic_suffix"`
}

// ChannelConfig defines per-channel PSK and enabled event families.
type ChannelConfig struct {
	PSK     string   `koanf:"psk" json:"psk"`
	Events  []string `koanf:"events" json:"events"`
	Primary bool     `koanf:"primary" json:"primary"`
}

// WebConfig contains HTTP/websocket and UI-related settings.
type WebConfig struct {
	ListenAddr string          `koanf:"listen_addr"`
	BasePath   string          `koanf:"base_path"`
	Chat       ChatConfig      `koanf:"chat"`
	WS         WSConfig        `koanf:"ws"`
	Map        MapConfig       `koanf:"map"`
	Relevance  RelevanceConfig `koanf:"relevance"`
	Log        LogConfig       `koanf:"log"`
	Stats      StatsConfig     `koanf:"stats"`
	Info       InfoConfig      `koanf:"info"`
}

// InfoConfig controls optional startup-loaded site information content.
type InfoConfig struct {
	File string `koanf:"file"`
}

// MapPrecisionCirclesMode controls how node precision circles are rendered on the web map.
type MapPrecisionCirclesMode string

const (
	// MapPrecisionCirclesNone disables node precision circles on the web map.
	MapPrecisionCirclesNone MapPrecisionCirclesMode = "none"
	// MapPrecisionCirclesSelected shows node precision circles only for the selected node.
	MapPrecisionCirclesSelected MapPrecisionCirclesMode = "selected"
	// MapPrecisionCirclesAlways shows node precision circles for every eligible node.
	MapPrecisionCirclesAlways MapPrecisionCirclesMode = "always"
)

// ChatConfig controls chat API/UI behavior.
type ChatConfig struct {
	Enabled            bool          `koanf:"enabled"`
	DefaultChannel     string        `koanf:"default_channel"`
	ShowRecentMessages int           `koanf:"show_recent_messages"`
	HistoryWindow      time.Duration `koanf:"history_window"`
}

// WSConfig configures websocket behavior.
type WSConfig struct {
	HeartbeatInterval time.Duration `koanf:"heartbeat_interval"`
	StatsInterval     time.Duration `koanf:"stats_interval"`
}

// MapConfig controls map rendering defaults and liveness thresholds.
type MapConfig struct {
	Clustering            bool                    `koanf:"clustering"`
	DisconnectedThreshold time.Duration           `koanf:"disconnected_threshold"`
	TopologyCacheTTL      time.Duration           `koanf:"topology_cache_ttl"`
	TopologyMaxEdges      int                     `koanf:"topology_max_edges"`
	PrecisionCirclesMode  MapPrecisionCirclesMode `koanf:"precision_circles_mode"`
	DefaultView           DefaultViewConfig       `koanf:"default_view"`
}

// RelevanceConfig controls API/UI visibility cutoffs for stale data.
type RelevanceConfig struct {
	TelemetryMaxAge        time.Duration `koanf:"telemetry_max_age"`
	TopologyEvidenceMaxAge time.Duration `koanf:"topology_evidence_max_age"`
	MapPositionMaxAge      time.Duration `koanf:"map_position_max_age"`
}

// LogConfig controls log tab behavior.
type LogConfig struct {
	LiveUpdates     bool `koanf:"live_updates"`
	PageSizeDefault int  `koanf:"page_size_default"`
}

// StatsConfig controls charts and aggregate activity APIs.
type StatsConfig struct {
	Activity StatsActivityConfig `koanf:"activity"`
}

// StatsActivityConfig configures fixed activity chart periods.
type StatsActivityConfig struct {
	Daily  StatsActivityBucketConfig `koanf:"daily"`
	Weekly StatsActivityBucketConfig `koanf:"weekly"`
}

// StatsActivityBucketConfig defines a bucket size for activity charts.
type StatsActivityBucketConfig struct {
	Bucket time.Duration `koanf:"bucket"`
}

// DefaultViewConfig defines initial map center and zoom.
type DefaultViewConfig struct {
	Latitude  float64 `koanf:"latitude"`
	Longitude float64 `koanf:"longitude"`
	Zoom      int     `koanf:"zoom"`
}

// LoggingConfig controls process log verbosity.
type LoggingConfig struct {
	Level string `koanf:"level"`
}

// UpdateCheckConfig configures the multi-source release-checker.
type UpdateCheckConfig struct {
	Enabled  bool                      `koanf:"enabled"`
	Interval time.Duration             `koanf:"interval"`
	Timeout  time.Duration             `koanf:"timeout"`
	Sources  []UpdateCheckSourceConfig `koanf:"sources"`
}

// UpdateCheckSourceConfig declares a single release-check source. The
// Manager constructs the upstream API URL from Type, BaseURL, and
// Repository at runtime — users only supply high-level fields.
type UpdateCheckSourceConfig struct {
	Name                 string `koanf:"name"`
	Label                string `koanf:"label"`
	Type                 string `koanf:"type"`
	BaseURL              string `koanf:"base_url"`
	Repository           string `koanf:"repository"`
	CurrentVersionSource string `koanf:"current_version_source"`
	Limit                int    `koanf:"limit"`
	// PreReleases, when true, includes pre-release (alpha/beta/rc) tags
	// alongside stable releases for sources that support the distinction.
	// Defaults to false to preserve the previous stable-only behaviour.
	PreReleases bool `koanf:"pre_releases"`
	// PostProcess controls release Markdown post-processing. Defaults to
	// true; set post_process: false to leave upstream bodies untouched.
	PostProcess *bool `koanf:"post_process"`
}

// PostProcessEnabled reports whether release Markdown should be normalized
// for this source. Nil means the field was omitted and the default applies.
func (c UpdateCheckSourceConfig) PostProcessEnabled() bool {
	return c.PostProcess == nil || *c.PostProcess
}
