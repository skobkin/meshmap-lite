package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the root application configuration loaded from YAML and environment.
type Config struct {
	MQTT       MQTTConfig               `koanf:"mqtt" json:"mqtt"`
	Storage    StorageConfig            `koanf:"storage" json:"storage"`
	MapReports MapReportsConfig         `koanf:"map_reports" json:"map_reports"`
	Channels   map[string]ChannelConfig `koanf:"channels" json:"channels"`
	Web        WebConfig                `koanf:"web" json:"web"`
	Logging    LoggingConfig            `koanf:"logging" json:"logging"`
}

// MQTTConfig contains MQTT broker and connection settings.
type MQTTConfig struct {
	Host             string        `koanf:"host"`
	Port             int           `koanf:"port"`
	TLS              bool          `koanf:"tls"`
	Username         string        `koanf:"username"`
	Password         string        `koanf:"password"`
	RootTopic        string        `koanf:"root_topic"`
	ProtocolVersion  string        `koanf:"protocol_version"`
	SubscribeQoS     byte          `koanf:"subscribe_qos"`
	CleanSession     bool          `koanf:"clean_session"`
	ReconnectTimeout time.Duration `koanf:"reconnect_timeout"`
	ConnectTimeout   time.Duration `koanf:"connect_timeout"`
	Keepalive        time.Duration `koanf:"keepalive"`
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
	Driver      string `koanf:"driver"`
	URL         string `koanf:"url"`
	AutoMigrate bool   `koanf:"auto_migrate"`
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
}

// MapConfig controls map rendering defaults and liveness thresholds.
type MapConfig struct {
	Clustering            bool              `koanf:"clustering"`
	DisconnectedThreshold time.Duration     `koanf:"disconnected_threshold"`
	DefaultView           DefaultViewConfig `koanf:"default_view"`
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

func defaultConfig() Config {
	return Config{
		MQTT: MQTTConfig{
			Port:             1883,
			ProtocolVersion:  "3.1.1",
			SubscribeQoS:     1,
			CleanSession:     false,
			ReconnectTimeout: 10 * time.Second,
			ConnectTimeout:   10 * time.Second,
			Keepalive:        60 * time.Second,
		},
		Storage: StorageConfig{
			KV:  KVConfig{Driver: "memory", Size: 100000, TTL: 6 * time.Hour},
			SQL: SQLConfig{Driver: "sqlite", URL: "/data/db.sqlite", AutoMigrate: true},
		},
		MapReports: MapReportsConfig{Enabled: true, TopicSuffix: "2/map"},
		Channels:   map[string]ChannelConfig{},
		Web: WebConfig{
			ListenAddr: ":8080",
			BasePath:   "/",
			Chat:       ChatConfig{Enabled: true, ShowRecentMessages: 50},
			WS:         WSConfig{HeartbeatInterval: 30 * time.Second},
			Map: MapConfig{
				Clustering:            true,
				DisconnectedThreshold: 60 * time.Minute,
				DefaultView:           DefaultViewConfig{Latitude: 64.5, Longitude: 40.6, Zoom: 13},
			},
		},
		Logging: LoggingConfig{Level: "info"},
	}
}

// Load reads configuration from an optional YAML file and MML_* environment overrides.
func Load(path string) (Config, error) {
	cfg := defaultConfig()
	k := koanf.New(".")
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
				return Config{}, fmt.Errorf("load yaml: %w", err)
			}
		}
	}

	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("decode yaml: %w", err)
	}

	applyEnv(&cfg)
	normalize(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyEnv(cfg *Config) {
	for _, raw := range os.Environ() {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := parts[0], parts[1]
		if !strings.HasPrefix(k, "MML_") {
			continue
		}
		path := strings.TrimPrefix(k, "MML_")
		segments := strings.Split(path, "__")
		for i := range segments {
			segments[i] = strings.ToLower(segments[i])
		}
		setPath(cfg, segments, v)
	}
}

func setPath(cfg *Config, parts []string, value string) {
	if len(parts) == 0 {
		return
	}
	joined := strings.Join(parts, ".")
	switch joined {
	case "mqtt.host":
		cfg.MQTT.Host = value
	case "mqtt.port":
		cfg.MQTT.Port = mustInt(value, cfg.MQTT.Port)
	case "mqtt.tls":
		cfg.MQTT.TLS = mustBool(value, cfg.MQTT.TLS)
	case "mqtt.username":
		cfg.MQTT.Username = value
	case "mqtt.password":
		cfg.MQTT.Password = value
	case "mqtt.root_topic":
		cfg.MQTT.RootTopic = value
	case "mqtt.protocol_version":
		cfg.MQTT.ProtocolVersion = value
	case "mqtt.subscribe_qos":
		cfg.MQTT.SubscribeQoS = byte(mustInt(value, int(cfg.MQTT.SubscribeQoS)))
	case "mqtt.clean_session":
		cfg.MQTT.CleanSession = mustBool(value, cfg.MQTT.CleanSession)
	case "mqtt.reconnect_timeout":
		cfg.MQTT.ReconnectTimeout = mustDuration(value, cfg.MQTT.ReconnectTimeout)
	case "mqtt.connect_timeout":
		cfg.MQTT.ConnectTimeout = mustDuration(value, cfg.MQTT.ConnectTimeout)
	case "mqtt.keepalive":
		cfg.MQTT.Keepalive = mustDuration(value, cfg.MQTT.Keepalive)
	case "storage.kv.driver":
		cfg.Storage.KV.Driver = value
	case "storage.kv.size":
		cfg.Storage.KV.Size = mustInt(value, cfg.Storage.KV.Size)
	case "storage.kv.ttl":
		cfg.Storage.KV.TTL = mustDuration(value, cfg.Storage.KV.TTL)
	case "storage.sql.driver":
		cfg.Storage.SQL.Driver = value
	case "storage.sql.url":
		cfg.Storage.SQL.URL = value
	case "storage.sql.auto_migrate":
		cfg.Storage.SQL.AutoMigrate = mustBool(value, cfg.Storage.SQL.AutoMigrate)
	case "map_reports.enabled":
		cfg.MapReports.Enabled = mustBool(value, cfg.MapReports.Enabled)
	case "map_reports.topic_suffix":
		cfg.MapReports.TopicSuffix = value
	case "web.listen_addr":
		cfg.Web.ListenAddr = value
	case "web.base_path":
		cfg.Web.BasePath = value
	case "web.chat.enabled":
		cfg.Web.Chat.Enabled = mustBool(value, cfg.Web.Chat.Enabled)
	case "web.chat.default_channel":
		cfg.Web.Chat.DefaultChannel = value
	case "web.chat.show_recent_messages":
		cfg.Web.Chat.ShowRecentMessages = mustInt(value, cfg.Web.Chat.ShowRecentMessages)
	case "web.ws.heartbeat_interval":
		cfg.Web.WS.HeartbeatInterval = mustDuration(value, cfg.Web.WS.HeartbeatInterval)
	case "web.map.clustering":
		cfg.Web.Map.Clustering = mustBool(value, cfg.Web.Map.Clustering)
	case "web.map.disconnected_threshold":
		cfg.Web.Map.DisconnectedThreshold = mustDuration(value, cfg.Web.Map.DisconnectedThreshold)
	case "web.map.default_view.latitude":
		cfg.Web.Map.DefaultView.Latitude = mustFloat(value, cfg.Web.Map.DefaultView.Latitude)
	case "web.map.default_view.longitude":
		cfg.Web.Map.DefaultView.Longitude = mustFloat(value, cfg.Web.Map.DefaultView.Longitude)
	case "web.map.default_view.zoom":
		cfg.Web.Map.DefaultView.Zoom = mustInt(value, cfg.Web.Map.DefaultView.Zoom)
	case "logging.level":
		cfg.Logging.Level = value
	default:
		if len(parts) >= 3 && parts[0] == "channels" {
			name := resolveChannelKey(cfg.Channels, parts[1])
			ch := cfg.Channels[name]
			switch parts[2] {
			case "psk":
				ch.PSK = value
			case "events":
				ch.Events = splitCSV(value)
			case "primary":
				ch.Primary = mustBool(value, ch.Primary)
			}
			cfg.Channels[name] = ch
		}
	}
}

func resolveChannelKey(channels map[string]ChannelConfig, name string) string {
	for existing := range channels {
		if strings.EqualFold(existing, name) {
			return existing
		}
	}

	return name
}

func normalize(cfg *Config) {
	if cfg.Channels == nil {
		cfg.Channels = map[string]ChannelConfig{}
	}
	normalized := make(map[string]ChannelConfig, len(cfg.Channels))
	for k, v := range cfg.Channels {
		key := strings.ToLower(strings.TrimSpace(k))
		if v.PSK == "" {
			v.PSK = "AQ=="
		}
		if len(v.Events) == 0 {
			v.Events = []string{"text_message", "node_info", "position", "telemetry"}
		}
		normalized[key] = v
	}
	cfg.Channels = normalized
	if cfg.Web.Chat.DefaultChannel == "" {
		names := make([]string, 0, len(cfg.Channels))
		for n := range cfg.Channels {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) > 0 {
			cfg.Web.Chat.DefaultChannel = names[0]
		}
	} else {
		cfg.Web.Chat.DefaultChannel = strings.ToLower(cfg.Web.Chat.DefaultChannel)
	}
}

func validate(cfg Config) error {
	if cfg.MQTT.RootTopic == "" {
		return errors.New("mqtt.root_topic is required")
	}
	if cfg.Storage.SQL.Driver != "sqlite" {
		return fmt.Errorf("unsupported storage.sql.driver: %s", cfg.Storage.SQL.Driver)
	}
	if cfg.Storage.KV.Driver != "memory" {
		return fmt.Errorf("unsupported storage.kv.driver: %s", cfg.Storage.KV.Driver)
	}
	if len(cfg.Channels) == 0 {
		return errors.New("at least one channel must be configured")
	}
	primary := 0
	for _, ch := range cfg.Channels {
		if ch.Primary {
			primary++
		}
	}
	if primary > 1 {
		return errors.New("at most one channels.*.primary=true is allowed")
	}

	return nil
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}

	return out
}

func mustInt(v string, d int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return d
	}

	return n
}

func mustBool(v string, d bool) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return d
	}

	return b
}

func mustDuration(v string, d time.Duration) time.Duration {
	t, err := time.ParseDuration(v)
	if err != nil {
		return d
	}

	return t
}

func mustFloat(v string, d float64) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return d
	}

	return f
}
