package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadWithEnvOverrides(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MML_MQTT__ROOT_TOPIC", "msh/override")
	t.Setenv("MML_MQTT__TLS", "true")
	t.Setenv("MML_MQTT__PROTOCOL_VERSION", "5")
	t.Setenv("MML_CHANNELS__LONGFAST__PRIMARY", "true")
	t.Setenv("MML_STORAGE__SQL__LOG_PRUNE_BATCH_ROWS", "1234")
	t.Setenv("MML_INGEST__TRACEROUTE__MAX_ENTRIES", "2222")
	t.Setenv("MML_INGEST__MAP_REPORTS__TOPIC_SUFFIX", "custom/map")
	t.Setenv("MML_WEB__WS__STATS_INTERVAL", "90s")
	t.Setenv("MML_WEB__MAP__PRECISION_CIRCLES_MODE", "always")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.RootTopic != "msh/override" {
		t.Fatalf("expected root_topic env override")
	}
	if !cfg.MQTT.TLS {
		t.Fatalf("expected tls env override")
	}
	if cfg.MQTT.ProtocolVersion != "5" {
		t.Fatalf("expected protocol_version env override")
	}
	if !cfg.Channels["LongFast"].Primary {
		t.Fatalf("expected channel env override")
	}
	if cfg.Storage.SQL.LogPruneBatchRows != 1234 {
		t.Fatalf("expected log_prune_batch_rows env override")
	}
	if cfg.Ingest.Traceroute.MaxEntries != 2222 {
		t.Fatalf("expected ingest traceroute max_entries env override")
	}
	if cfg.Ingest.MapReports.TopicSuffix != "custom/map" {
		t.Fatalf("expected ingest map_reports topic_suffix env override")
	}
	if cfg.Web.WS.StatsInterval != 90*time.Second {
		t.Fatalf("expected web.ws.stats_interval env override")
	}
	if cfg.Web.Map.PrecisionCirclesMode != MapPrecisionCirclesAlways {
		t.Fatalf("expected web.map.precision_circles_mode env override, got %q", cfg.Web.Map.PrecisionCirclesMode)
	}
}

func TestLoadNormalizesNegativeLogPruneBatchRows(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
storage:
  sql:
    log_prune_batch_rows: -100
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.SQL.LogPruneBatchRows != 0 {
		t.Fatalf("expected negative log_prune_batch_rows to normalize to 0, got %d", cfg.Storage.SQL.LogPruneBatchRows)
	}
}

func TestLoadDefaultsDisableMapClustering(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Map.Clustering {
		t.Fatalf("expected web.map.clustering default to be false")
	}
	if cfg.Web.Map.PrecisionCirclesMode != MapPrecisionCirclesSelected {
		t.Fatalf("expected web.map.precision_circles_mode default to be %q, got %q", MapPrecisionCirclesSelected, cfg.Web.Map.PrecisionCirclesMode)
	}
}

func TestLoadRejectsInvalidPrecisionCirclesMode(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  LongFast:
    psk: AQ==
web:
  map:
    precision_circles_mode: hover
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected invalid precision circles mode to fail validation")
	}
}

func TestLoadNormalizesInvalidTracerouteTrackerBounds(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
ingest:
  traceroute:
    timeout: 0s
    max_entries: 0
    final_retention: 0s
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ingest.Traceroute.Timeout <= 0 {
		t.Fatalf("expected positive traceroute timeout, got %v", cfg.Ingest.Traceroute.Timeout)
	}
	if cfg.Ingest.Traceroute.MaxEntries != 1000 {
		t.Fatalf("expected default traceroute max_entries, got %d", cfg.Ingest.Traceroute.MaxEntries)
	}
	if cfg.Ingest.Traceroute.FinalRetention != cfg.Ingest.Traceroute.Timeout {
		t.Fatalf("expected final retention to normalize to timeout, got %v want %v", cfg.Ingest.Traceroute.FinalRetention, cfg.Ingest.Traceroute.Timeout)
	}
}

func TestLoadNormalizesInvalidWSIntervals(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  ws:
    heartbeat_interval: 0s
    stats_interval: 0s
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.WS.HeartbeatInterval != DefaultWSHeartbeatInterval {
		t.Fatalf("expected default heartbeat interval, got %v", cfg.Web.WS.HeartbeatInterval)
	}
	if cfg.Web.WS.StatsInterval != DefaultWSStatsInterval {
		t.Fatalf("expected default stats interval, got %v", cfg.Web.WS.StatsInterval)
	}
}

func TestLoadMissingFileUsesDefaultsAndEnv(t *testing.T) {
	t.Setenv("MML_MQTT__ROOT_TOPIC", "msh/env")
	t.Setenv("MML_CHANNELS__LongFast__PSK", "AQ==")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.RootTopic != "msh/env" {
		t.Fatalf("expected env root topic, got %q", cfg.MQTT.RootTopic)
	}
	if _, ok := cfg.Channels["LongFast"]; !ok {
		t.Fatalf("expected channel from env override")
	}
	if cfg.Storage.SQL.URL != defaultSQLiteDBPath {
		t.Fatalf("expected default sqlite path, got %q", cfg.Storage.SQL.URL)
	}
}

func TestLoadResolvesChannelNamesCaseInsensitively(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  LongFast:
    psk: AQ==
web:
  chat:
    default_channel: longfast
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Chat.DefaultChannel != "LongFast" {
		t.Fatalf("expected case-insensitive channel resolution, got %q", cfg.Web.Chat.DefaultChannel)
	}
}

func TestLoadSelectsAlphabeticalDefaultChannel(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  Zebra:
    psk: AQ==
  Alpha:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Chat.DefaultChannel != "Alpha" {
		t.Fatalf("expected alphabetical default channel, got %q", cfg.Web.Chat.DefaultChannel)
	}
}

func TestLoadNormalizesLogPageSizeBounds(t *testing.T) {
	t.Run("lower bound", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Channels = map[string]ChannelConfig{"LongFast": {PSK: "AQ=="}}
		cfg.MQTT.RootTopic = "msh/test"
		cfg.Web.Log.PageSizeDefault = 0

		normalize(&cfg)
		if cfg.Web.Log.PageSizeDefault != defaultLogPageSize {
			t.Fatalf("expected default page size, got %d", cfg.Web.Log.PageSizeDefault)
		}
	})

	t.Run("upper bound", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Channels = map[string]ChannelConfig{"LongFast": {PSK: "AQ=="}}
		cfg.MQTT.RootTopic = "msh/test"
		cfg.Web.Log.PageSizeDefault = 999

		normalize(&cfg)
		if cfg.Web.Log.PageSizeDefault != maxLogPageSize {
			t.Fatalf("expected capped page size, got %d", cfg.Web.Log.PageSizeDefault)
		}
	})
}

func TestMustByteBounds(t *testing.T) {
	if got := mustByte("-1", 7); got != 0 {
		t.Fatalf("expected negative byte to clamp to 0, got %d", got)
	}
	if got := mustByte("999", 7); got != 255 {
		t.Fatalf("expected oversized byte to clamp to 255, got %d", got)
	}
	if got := mustByte("invalid", 7); got != 7 {
		t.Fatalf("expected invalid byte to keep default, got %d", got)
	}
}

func TestLoadInvalidEnvValuesFallBackToDecodedValues(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
  port: 1884
web:
  log:
    page_size_default: 123
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MML_MQTT__PORT", "not-a-number")
	t.Setenv("MML_WEB__LOG__PAGE_SIZE_DEFAULT", "wat")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.Port != 1884 {
		t.Fatalf("expected invalid env port to keep YAML value, got %d", cfg.MQTT.Port)
	}
	if cfg.Web.Log.PageSizeDefault != 123 {
		t.Fatalf("expected invalid env page size to keep YAML value, got %d", cfg.Web.Log.PageSizeDefault)
	}
}

func TestValidateFailures(t *testing.T) {
	t.Run("missing root topic", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Channels = map[string]ChannelConfig{"LongFast": {PSK: "AQ=="}}
		cfg.MQTT.RootTopic = ""

		err := validate(cfg)
		if err == nil || err.Error() != "mqtt.root_topic is required" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("multiple primary channels", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.MQTT.RootTopic = "msh/test"
		cfg.Channels = map[string]ChannelConfig{
			"LongFast": {PSK: "AQ==", Primary: true},
			"Slow":     {PSK: "AQ==", Primary: true},
		}

		err := validate(cfg)
		if err == nil || err.Error() != "at most one channels.*.primary=true is allowed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
