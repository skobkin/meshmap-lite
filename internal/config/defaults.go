package config

import "time"

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
	maxStatsActivityBuckets   = 1000
)

var defaultChannelEvents = []string{"text_message", "node_info", "position", "telemetry"}

const (
	defaultMQTTReconnectTimeout     = 10 * time.Second
	defaultMQTTConnectTimeout       = 10 * time.Second
	defaultMQTTKeepalive            = 60 * time.Second
	defaultTracerouteTimeout        = 60 * time.Second
	defaultStorageKVTTL             = 6 * time.Hour
	defaultMapDisconnectedThreshold = 60 * time.Minute
	defaultMapHidePositionAfter     = 14 * 24 * time.Hour
	defaultMapTopologyCacheTTL      = 10 * time.Minute
	defaultTracerouteFinalRetention = defaultTracerouteTimeout
	defaultStorageKVSize            = 100000
	defaultStatsDailyWindow         = 24 * time.Hour
	defaultStatsDailyBucket         = 5 * time.Minute
	defaultStatsWeeklyWindow        = 168 * time.Hour
	defaultStatsWeeklyBucket        = time.Hour
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
			Chat:       ChatConfig{Enabled: true, ShowRecentMessages: defaultShowRecentMessages},
			WS: WSConfig{
				HeartbeatInterval: DefaultWSHeartbeatInterval,
				StatsInterval:     DefaultWSStatsInterval,
			},
			Map: MapConfig{
				Clustering:            false,
				DisconnectedThreshold: defaultMapDisconnectedThreshold,
				HidePositionAfter:     defaultMapHidePositionAfter,
				TopologyCacheTTL:      defaultMapTopologyCacheTTL,
				PrecisionCirclesMode:  MapPrecisionCirclesSelected,
				DefaultView:           DefaultViewConfig{Latitude: 64.5, Longitude: 40.6, Zoom: 13},
			},
			Log: LogConfig{
				LiveUpdates:     true,
				PageSizeDefault: defaultLogPageSize,
			},
			Stats: StatsConfig{
				Activity: StatsActivityConfig{
					Daily: StatsActivityPeriodConfig{
						Window: defaultStatsDailyWindow,
						Bucket: defaultStatsDailyBucket,
					},
					Weekly: StatsActivityPeriodConfig{
						Window: defaultStatsWeeklyWindow,
						Bucket: defaultStatsWeeklyBucket,
					},
				},
			},
		},
		Logging: LoggingConfig{Level: defaultLoggingLevel},
	}
}
