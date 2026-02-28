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
	MQTT       MQTTConfig               `koanf:"mqtt" json:"mqtt"`
	Ingest     IngestConfig             `koanf:"ingest" json:"ingest"`
	Storage    StorageConfig            `koanf:"storage" json:"storage"`
	MapReports MapReportsConfig         `koanf:"map_reports" json:"map_reports"`
	Channels   map[string]ChannelConfig `koanf:"channels" json:"channels"`
	Web        WebConfig                `koanf:"web" json:"web"`
	Logging    LoggingConfig            `koanf:"logging" json:"logging"`
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
	ListenAddr string     `koanf:"listen_addr"`
	BasePath   string     `koanf:"base_path"`
	Chat       ChatConfig `koanf:"chat"`
	WS         WSConfig   `koanf:"ws"`
	Map        MapConfig  `koanf:"map"`
	Log        LogConfig  `koanf:"log"`
}

// ChatConfig controls chat API/UI behavior.
type ChatConfig struct {
	Enabled            bool   `koanf:"enabled"`
	DefaultChannel     string `koanf:"default_channel"`
	ShowRecentMessages int    `koanf:"show_recent_messages"`
}

// WSConfig configures websocket behavior.
type WSConfig struct {
	HeartbeatInterval time.Duration `koanf:"heartbeat_interval"`
	StatsInterval     time.Duration `koanf:"stats_interval"`
}

// MapConfig controls map rendering defaults and liveness thresholds.
type MapConfig struct {
	Clustering            bool              `koanf:"clustering"`
	DisconnectedThreshold time.Duration     `koanf:"disconnected_threshold"`
	DefaultView           DefaultViewConfig `koanf:"default_view"`
}

// LogConfig controls log tab behavior.
type LogConfig struct {
	LiveUpdates     bool `koanf:"live_updates"`
	PageSizeDefault int  `koanf:"page_size_default"`
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
