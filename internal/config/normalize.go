package config

import (
	"sort"
	"strings"
	"time"
)

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
	channelKeys := make([]string, 0, len(cfg.Channels))
	for key := range cfg.Channels {
		channelKeys = append(channelKeys, key)
	}
	sort.Slice(channelKeys, func(i, j int) bool {
		iUpper := channelKeys[i] == strings.ToUpper(channelKeys[i])
		jUpper := channelKeys[j] == strings.ToUpper(channelKeys[j])
		if iUpper != jUpper {
			return !iUpper
		}

		return channelKeys[i] < channelKeys[j]
	})

	normalized := make(map[string]ChannelConfig, len(cfg.Channels))
	for _, key := range channelKeys {
		channel := cfg.Channels[key]
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if existing := resolveChannelKey(normalized, trimmedKey); existing != trimmedKey {
			normalized[existing] = mergeChannelConfig(normalized[existing], channel)

			continue
		}
		if channel.PSK == "" {
			channel.PSK = defaultChannelPSK
		}
		if len(channel.Events) == 0 {
			channel.Events = append([]string(nil), defaultChannelEvents...)
		}
		normalized[trimmedKey] = channel
	}
	cfg.Channels = normalized

	if cfg.Web.Chat.DefaultChannel == "" {
		names := make([]string, 0, len(cfg.Channels))
		for name := range cfg.Channels {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) > 0 {
			cfg.Web.Chat.DefaultChannel = names[0]
		}
	} else {
		cfg.Web.Chat.DefaultChannel = resolveChannelKey(cfg.Channels, strings.TrimSpace(cfg.Web.Chat.DefaultChannel))
	}

	if cfg.Web.Log.PageSizeDefault <= 0 {
		cfg.Web.Log.PageSizeDefault = defaultLogPageSize
	}
	if cfg.Web.Log.PageSizeDefault > maxLogPageSize {
		cfg.Web.Log.PageSizeDefault = maxLogPageSize
	}
	if cfg.Web.Chat.HistoryWindow <= 0 {
		cfg.Web.Chat.HistoryWindow = defaultChatHistoryWindow
	}
	if cfg.Storage.SQL.LogMaxRows < 0 {
		cfg.Storage.SQL.LogMaxRows = 0
	}
	if cfg.Storage.SQL.LogPruneBatchRows < 0 {
		cfg.Storage.SQL.LogPruneBatchRows = 0
	}
	if cfg.Ingest.Traceroute.Timeout <= 0 {
		cfg.Ingest.Traceroute.Timeout = defaultTracerouteTimeout
	}
	if cfg.Ingest.Traceroute.MaxEntries < 1 {
		cfg.Ingest.Traceroute.MaxEntries = defaultTracerouteEntries
	}
	if cfg.Ingest.Traceroute.FinalRetention <= 0 {
		cfg.Ingest.Traceroute.FinalRetention = cfg.Ingest.Traceroute.Timeout
	}
	if cfg.Web.WS.HeartbeatInterval <= 0 {
		cfg.Web.WS.HeartbeatInterval = DefaultWSHeartbeatInterval
	}
	if cfg.Web.WS.StatsInterval <= 0 {
		cfg.Web.WS.StatsInterval = DefaultWSStatsInterval
	}
	if cfg.Web.Map.TopologyCacheTTL <= 0 {
		cfg.Web.Map.TopologyCacheTTL = defaultMapTopologyCacheTTL
	}
	if cfg.Web.Map.TopologyMaxEdges <= 0 {
		cfg.Web.Map.TopologyMaxEdges = defaultMapTopologyMaxEdges
	}
	if cfg.Web.Relevance.TelemetryMaxAge <= 0 {
		cfg.Web.Relevance.TelemetryMaxAge = defaultRelevanceTelemetryMaxAge
	}
	if cfg.Web.Relevance.TopologyEvidenceMaxAge <= 0 {
		cfg.Web.Relevance.TopologyEvidenceMaxAge = defaultRelevanceTopologyEvidenceMaxAge
	}
	if cfg.Web.Relevance.MapPositionMaxAge <= 0 {
		cfg.Web.Relevance.MapPositionMaxAge = defaultRelevanceMapPositionMaxAge
	}
	normalizeStatsActivityBucket(&cfg.Web.Stats.Activity.Daily, defaultStatsDailyBucket)
	normalizeStatsActivityBucket(&cfg.Web.Stats.Activity.Weekly, defaultStatsWeeklyBucket)
}

func mergeChannelConfig(dst, src ChannelConfig) ChannelConfig {
	if src.PSK != "" {
		dst.PSK = src.PSK
	}
	if len(src.Events) > 0 {
		dst.Events = src.Events
	}
	if src.Primary {
		dst.Primary = true
	}

	return dst
}

func normalizeStatsActivityBucket(period *StatsActivityBucketConfig, defaultBucket time.Duration) {
	if period.Bucket <= 0 {
		period.Bucket = defaultBucket
	}
}
