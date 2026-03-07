package config

import (
	"sort"
	"strings"
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
	normalized := make(map[string]ChannelConfig, len(cfg.Channels))
	for key, channel := range cfg.Channels {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
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
	if cfg.Web.Map.HidePositionAfter <= 0 {
		cfg.Web.Map.HidePositionAfter = defaultMapHidePositionAfter
	}
}
