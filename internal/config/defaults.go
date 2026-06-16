package config

import (
	"time"

	"meshmap-lite/internal/updatecheck"
)

const (
	envPrefix                 = "MML_"
	envNestingSeparator       = "__"
	defaultChannelPSK         = "AQ=="
	defaultSQLiteDBPath       = "/data/db.sqlite"
	defaultLogPageSize        = 100
	maxLogPageSize            = 500
	defaultMQTTPort           = 1883
	defaultMQTTProtocol       = "3.1.1"
	defaultMQTTSubscribeQoS   = 1
	defaultTracerouteEntries  = 1000
	defaultLogMaxRows         = 50000
	defaultLogPruneBatchRows  = 1000
	defaultWebListenAddr      = ":8080"
	defaultWebBasePath        = "/"
	defaultShowRecentMessages = 50
	defaultLoggingLevel       = "info"
)

var defaultChannelEvents = []string{"text_message", "node_info", "position", "telemetry"}

const (
	defaultMQTTReconnectTimeout            = 10 * time.Second
	defaultMQTTConnectTimeout              = 10 * time.Second
	defaultMQTTKeepalive                   = 60 * time.Second
	defaultTracerouteTimeout               = 60 * time.Second
	defaultStorageKVTTL                    = 6 * time.Hour
	defaultMapDisconnectedThreshold        = 60 * time.Minute
	defaultMapTopologyCacheTTL             = 10 * time.Minute
	defaultMapTopologyMaxEdges             = 2000
	defaultRelevanceTelemetryMaxAge        = 24 * time.Hour
	defaultRelevanceTopologyEvidenceMaxAge = 72 * time.Hour
	defaultRelevanceMapPositionMaxAge      = 14 * 24 * time.Hour
	defaultTracerouteFinalRetention        = defaultTracerouteTimeout
	defaultStorageKVSize                   = 100000
	defaultStatsDailyBucket                = 5 * time.Minute
	defaultStatsWeeklyBucket               = time.Hour
	defaultChatHistoryWindow               = 7 * 24 * time.Hour
	defaultUpdateCheckInterval             = 12 * time.Hour
	defaultUpdateCheckTimeout              = 15 * time.Second
	defaultUpdateCheckLimit                = 15
)

func defaultConfig() Config {
	return Config{
		MQTT: MQTTConfig{
			Port:             defaultMQTTPort,
			ProtocolVersion:  defaultMQTTProtocol,
			SubscribeQoS:     defaultMQTTSubscribeQoS,
			CleanSession:     false,
			ReconnectTimeout: defaultMQTTReconnectTimeout,
			ConnectTimeout:   defaultMQTTConnectTimeout,
			Keepalive:        defaultMQTTKeepalive,
		},
		Ingest: IngestConfig{
			MapReports: MapReportsConfig{Enabled: true, TopicSuffix: "2/map"},
			Traceroute: TracerouteIngestConfig{
				Timeout:        defaultTracerouteTimeout,
				MaxEntries:     defaultTracerouteEntries,
				FinalRetention: defaultTracerouteFinalRetention,
			},
		},
		Storage: StorageConfig{
			KV: KVConfig{Driver: "memory", Size: defaultStorageKVSize, TTL: defaultStorageKVTTL},
			SQL: SQLConfig{
				Driver:            "sqlite",
				URL:               defaultSQLiteDBPath,
				AutoMigrate:       true,
				LogMaxRows:        defaultLogMaxRows,
				LogPruneBatchRows: defaultLogPruneBatchRows,
			},
		},
		Channels: map[string]ChannelConfig{},
		Web: WebConfig{
			ListenAddr: defaultWebListenAddr,
			BasePath:   defaultWebBasePath,
			Chat: ChatConfig{
				Enabled:            true,
				ShowRecentMessages: defaultShowRecentMessages,
				HistoryWindow:      defaultChatHistoryWindow,
			},
			WS: WSConfig{
				HeartbeatInterval: DefaultWSHeartbeatInterval,
				StatsInterval:     DefaultWSStatsInterval,
			},
			Map: MapConfig{
				Clustering:            false,
				DisconnectedThreshold: defaultMapDisconnectedThreshold,
				TopologyCacheTTL:      defaultMapTopologyCacheTTL,
				TopologyMaxEdges:      defaultMapTopologyMaxEdges,
				PrecisionCirclesMode:  MapPrecisionCirclesSelected,
				DefaultView:           DefaultViewConfig{Latitude: 64.5, Longitude: 40.6, Zoom: 13},
			},
			Relevance: RelevanceConfig{
				TelemetryMaxAge:        defaultRelevanceTelemetryMaxAge,
				TopologyEvidenceMaxAge: defaultRelevanceTopologyEvidenceMaxAge,
				MapPositionMaxAge:      defaultRelevanceMapPositionMaxAge,
			},
			Log: LogConfig{
				LiveUpdates:     true,
				PageSizeDefault: defaultLogPageSize,
			},
			Stats: StatsConfig{
				Activity: StatsActivityConfig{
					Daily: StatsActivityBucketConfig{
						Bucket: defaultStatsDailyBucket,
					},
					Weekly: StatsActivityBucketConfig{
						Bucket: defaultStatsWeeklyBucket,
					},
				},
			},
		},
		Logging: LoggingConfig{Level: defaultLoggingLevel},
		UpdateCheck: UpdateCheckConfig{
			Enabled:  true,
			Interval: defaultUpdateCheckInterval,
			Timeout:  defaultUpdateCheckTimeout,
		},
	}
}

// DefaultUpdateCheckSource is the canonical meshmap-lite release source
// used when no explicit source is declared. The wiring layer registers
// it at startup if the user has not provided their own entry.
var DefaultUpdateCheckSource = UpdateCheckSourceConfig{
	Name:                 "meshmap-lite",
	Label:                "Map",
	Type:                 updatecheck.SourceTypeForgejo,
	BaseURL:              "https://git.skobk.in",
	Repository:           "skobkin/meshmap-lite",
	CurrentVersionSource: "buildinfo",
	Limit:                defaultUpdateCheckLimit,
}
