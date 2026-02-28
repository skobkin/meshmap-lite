package config

import (
	"os"
	"strings"
)

type envSetter func(cfg *Config, value string)

var envSetters = map[string]envSetter{
	"mqtt.host":             func(cfg *Config, value string) { cfg.MQTT.Host = value },
	"mqtt.port":             func(cfg *Config, value string) { cfg.MQTT.Port = mustInt(value, cfg.MQTT.Port) },
	"mqtt.tls":              func(cfg *Config, value string) { cfg.MQTT.TLS = mustBool(value, cfg.MQTT.TLS) },
	"mqtt.client_id":        func(cfg *Config, value string) { cfg.MQTT.ClientID = value },
	"mqtt.username":         func(cfg *Config, value string) { cfg.MQTT.Username = value },
	"mqtt.password":         func(cfg *Config, value string) { cfg.MQTT.Password = value },
	"mqtt.root_topic":       func(cfg *Config, value string) { cfg.MQTT.RootTopic = value },
	"mqtt.protocol_version": func(cfg *Config, value string) { cfg.MQTT.ProtocolVersion = value },
	"mqtt.subscribe_qos":    func(cfg *Config, value string) { cfg.MQTT.SubscribeQoS = mustByte(value, cfg.MQTT.SubscribeQoS) },
	"mqtt.clean_session":    func(cfg *Config, value string) { cfg.MQTT.CleanSession = mustBool(value, cfg.MQTT.CleanSession) },
	"mqtt.reconnect_timeout": func(cfg *Config, value string) {
		cfg.MQTT.ReconnectTimeout = mustDuration(value, cfg.MQTT.ReconnectTimeout)
	},
	"mqtt.connect_timeout": func(cfg *Config, value string) {
		cfg.MQTT.ConnectTimeout = mustDuration(value, cfg.MQTT.ConnectTimeout)
	},
	"mqtt.keepalive":     func(cfg *Config, value string) { cfg.MQTT.Keepalive = mustDuration(value, cfg.MQTT.Keepalive) },
	"storage.kv.driver":  func(cfg *Config, value string) { cfg.Storage.KV.Driver = value },
	"storage.kv.size":    func(cfg *Config, value string) { cfg.Storage.KV.Size = mustInt(value, cfg.Storage.KV.Size) },
	"storage.kv.ttl":     func(cfg *Config, value string) { cfg.Storage.KV.TTL = mustDuration(value, cfg.Storage.KV.TTL) },
	"storage.sql.driver": func(cfg *Config, value string) { cfg.Storage.SQL.Driver = value },
	"storage.sql.url":    func(cfg *Config, value string) { cfg.Storage.SQL.URL = value },
	"storage.sql.auto_migrate": func(cfg *Config, value string) {
		cfg.Storage.SQL.AutoMigrate = mustBool(value, cfg.Storage.SQL.AutoMigrate)
	},
	"storage.sql.log_max_rows": func(cfg *Config, value string) {
		cfg.Storage.SQL.LogMaxRows = mustInt(value, cfg.Storage.SQL.LogMaxRows)
	},
	"storage.sql.log_prune_batch_rows": func(cfg *Config, value string) {
		cfg.Storage.SQL.LogPruneBatchRows = mustInt(value, cfg.Storage.SQL.LogPruneBatchRows)
	},
	"ingest.traceroute.timeout": func(cfg *Config, value string) {
		cfg.Ingest.Traceroute.Timeout = mustDuration(value, cfg.Ingest.Traceroute.Timeout)
	},
	"ingest.traceroute.max_entries": func(cfg *Config, value string) {
		cfg.Ingest.Traceroute.MaxEntries = mustInt(value, cfg.Ingest.Traceroute.MaxEntries)
	},
	"ingest.traceroute.final_retention": func(cfg *Config, value string) {
		cfg.Ingest.Traceroute.FinalRetention = mustDuration(value, cfg.Ingest.Traceroute.FinalRetention)
	},
	"map_reports.enabled":      func(cfg *Config, value string) { cfg.MapReports.Enabled = mustBool(value, cfg.MapReports.Enabled) },
	"map_reports.topic_suffix": func(cfg *Config, value string) { cfg.MapReports.TopicSuffix = value },
	"web.listen_addr":          func(cfg *Config, value string) { cfg.Web.ListenAddr = value },
	"web.base_path":            func(cfg *Config, value string) { cfg.Web.BasePath = value },
	"web.chat.enabled":         func(cfg *Config, value string) { cfg.Web.Chat.Enabled = mustBool(value, cfg.Web.Chat.Enabled) },
	"web.chat.default_channel": func(cfg *Config, value string) { cfg.Web.Chat.DefaultChannel = value },
	"web.chat.show_recent_messages": func(cfg *Config, value string) {
		cfg.Web.Chat.ShowRecentMessages = mustInt(value, cfg.Web.Chat.ShowRecentMessages)
	},
	"web.ws.heartbeat_interval": func(cfg *Config, value string) {
		cfg.Web.WS.HeartbeatInterval = mustDuration(value, cfg.Web.WS.HeartbeatInterval)
	},
	"web.ws.stats_interval": func(cfg *Config, value string) {
		cfg.Web.WS.StatsInterval = mustDuration(value, cfg.Web.WS.StatsInterval)
	},
	"web.map.clustering": func(cfg *Config, value string) { cfg.Web.Map.Clustering = mustBool(value, cfg.Web.Map.Clustering) },
	"web.map.disconnected_threshold": func(cfg *Config, value string) {
		cfg.Web.Map.DisconnectedThreshold = mustDuration(value, cfg.Web.Map.DisconnectedThreshold)
	},
	"web.map.default_view.latitude": func(cfg *Config, value string) {
		cfg.Web.Map.DefaultView.Latitude = mustFloat(value, cfg.Web.Map.DefaultView.Latitude)
	},
	"web.map.default_view.longitude": func(cfg *Config, value string) {
		cfg.Web.Map.DefaultView.Longitude = mustFloat(value, cfg.Web.Map.DefaultView.Longitude)
	},
	"web.map.default_view.zoom": func(cfg *Config, value string) {
		cfg.Web.Map.DefaultView.Zoom = mustInt(value, cfg.Web.Map.DefaultView.Zoom)
	},
	"web.log.live_updates": func(cfg *Config, value string) { cfg.Web.Log.LiveUpdates = mustBool(value, cfg.Web.Log.LiveUpdates) },
	"web.log.page_size_default": func(cfg *Config, value string) {
		cfg.Web.Log.PageSizeDefault = mustInt(value, cfg.Web.Log.PageSizeDefault)
	},
	"logging.level": func(cfg *Config, value string) { cfg.Logging.Level = value },
}

func applyEnv(cfg *Config) {
	for _, raw := range os.Environ() {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		if !strings.HasPrefix(key, envPrefix) {
			continue
		}

		segments := envSegments(strings.TrimPrefix(key, envPrefix))
		if len(segments) == 0 {
			continue
		}

		joined := strings.Join(segments, ".")
		if setter, ok := envSetters[joined]; ok {
			setter(cfg, value)

			continue
		}

		applyChannelEnv(cfg, segments, value)
	}
}

func envSegments(path string) []string {
	rawSegments := strings.Split(path, envNestingSeparator)
	segments := make([]string, len(rawSegments))
	copy(segments, rawSegments)
	for i := range segments {
		segments[i] = strings.ToLower(segments[i])
	}
	if len(rawSegments) >= 3 && strings.EqualFold(rawSegments[0], "channels") {
		segments[0] = "channels"
		segments[1] = strings.TrimSpace(rawSegments[1])
	}

	return segments
}

func applyChannelEnv(cfg *Config, parts []string, value string) {
	if len(parts) < 3 || parts[0] != "channels" {
		return
	}

	name := resolveChannelKey(cfg.Channels, parts[1])
	ch := cfg.Channels[name]
	switch parts[2] {
	case "psk":
		ch.PSK = value
	case "events":
		ch.Events = splitCSV(value)
	case "primary":
		ch.Primary = mustBool(value, ch.Primary)
	default:
		return
	}
	cfg.Channels[name] = ch
}
