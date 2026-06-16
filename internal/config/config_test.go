package config

import (
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("MML_CHANNELS__LONGFAST__EVENTS", "text_message, telemetry")
	t.Setenv("MML_STORAGE__SQL__LOG_PRUNE_BATCH_ROWS", "1234")
	t.Setenv("MML_INGEST__TRACEROUTE__MAX_ENTRIES", "2222")
	t.Setenv("MML_INGEST__MAP_REPORTS__TOPIC_SUFFIX", "custom/map")
	t.Setenv("MML_WEB__WS__STATS_INTERVAL", "90s")
	t.Setenv("MML_WEB__MAP__TOPOLOGY_CACHE_TTL", "25m")
	t.Setenv("MML_WEB__MAP__TOPOLOGY_MAX_EDGES", "1500")
	t.Setenv("MML_WEB__RELEVANCE__TELEMETRY_MAX_AGE", "12h")
	t.Setenv("MML_WEB__RELEVANCE__TOPOLOGY_EVIDENCE_MAX_AGE", "48h")
	t.Setenv("MML_WEB__RELEVANCE__MAP_POSITION_MAX_AGE", "168h")
	t.Setenv("MML_WEB__MAP__PRECISION_CIRCLES_MODE", "always")
	t.Setenv("MML_WEB__CHAT__HISTORY_WINDOW", "24h")
	t.Setenv("MML_WEB__STATS__ACTIVITY__DAILY__BUCKET", "10m")
	t.Setenv("MML_WEB__INFO__FILE", "/srv/meshmap/info.md")
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
	if cfg.Channels["LongFast"].PSK != "AQ==" {
		t.Fatalf("expected existing channel psk to be preserved, got %q", cfg.Channels["LongFast"].PSK)
	}
	if got, want := cfg.Channels["LongFast"].Events, []string{"text_message", "telemetry"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("expected channel events env override, got %#v", got)
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
	if cfg.Web.Map.TopologyCacheTTL != 25*time.Minute {
		t.Fatalf("expected web.map.topology_cache_ttl env override, got %v", cfg.Web.Map.TopologyCacheTTL)
	}
	if cfg.Web.Map.TopologyMaxEdges != 1500 {
		t.Fatalf("expected web.map.topology_max_edges env override, got %d", cfg.Web.Map.TopologyMaxEdges)
	}
	if cfg.Web.Relevance.TelemetryMaxAge != 12*time.Hour {
		t.Fatalf("expected web.relevance.telemetry_max_age env override, got %v", cfg.Web.Relevance.TelemetryMaxAge)
	}
	if cfg.Web.Relevance.TopologyEvidenceMaxAge != 48*time.Hour {
		t.Fatalf("expected web.relevance.topology_evidence_max_age env override, got %v", cfg.Web.Relevance.TopologyEvidenceMaxAge)
	}
	if cfg.Web.Relevance.MapPositionMaxAge != 7*24*time.Hour {
		t.Fatalf("expected web.relevance.map_position_max_age env override, got %v", cfg.Web.Relevance.MapPositionMaxAge)
	}
	if cfg.Web.Map.PrecisionCirclesMode != MapPrecisionCirclesAlways {
		t.Fatalf("expected web.map.precision_circles_mode env override, got %q", cfg.Web.Map.PrecisionCirclesMode)
	}
	if cfg.Web.Chat.HistoryWindow != 24*time.Hour {
		t.Fatalf("expected web.chat.history_window env override, got %v", cfg.Web.Chat.HistoryWindow)
	}
	if cfg.Web.Stats.Activity.Daily.Bucket != 10*time.Minute {
		t.Fatalf("expected web.stats.activity.daily.bucket env override, got %v", cfg.Web.Stats.Activity.Daily.Bucket)
	}
	if cfg.Web.Info.File != "/srv/meshmap/info.md" {
		t.Fatalf("expected web.info.file env override, got %q", cfg.Web.Info.File)
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
	if cfg.Web.Map.TopologyCacheTTL != 10*time.Minute {
		t.Fatalf("expected web.map.topology_cache_ttl default to be 10 minutes, got %v", cfg.Web.Map.TopologyCacheTTL)
	}
	if cfg.Web.Map.TopologyMaxEdges != 2000 {
		t.Fatalf("expected web.map.topology_max_edges default to be 2000, got %d", cfg.Web.Map.TopologyMaxEdges)
	}
	if cfg.Web.Relevance.TelemetryMaxAge != 24*time.Hour {
		t.Fatalf("expected web.relevance.telemetry_max_age default to be 24 hours, got %v", cfg.Web.Relevance.TelemetryMaxAge)
	}
	if cfg.Web.Relevance.TopologyEvidenceMaxAge != 72*time.Hour {
		t.Fatalf("expected web.relevance.topology_evidence_max_age default to be 72 hours, got %v", cfg.Web.Relevance.TopologyEvidenceMaxAge)
	}
	if cfg.Web.Relevance.MapPositionMaxAge != 14*24*time.Hour {
		t.Fatalf("expected web.relevance.map_position_max_age default to be 14 days, got %v", cfg.Web.Relevance.MapPositionMaxAge)
	}
	if cfg.Web.Chat.HistoryWindow != 7*24*time.Hour {
		t.Fatalf("expected web.chat.history_window default to be 7 days, got %v", cfg.Web.Chat.HistoryWindow)
	}
}

func TestLoadReadsChatHistoryWindowFromYAML(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  chat:
    history_window: 48h
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
	if cfg.Web.Chat.HistoryWindow != 48*time.Hour {
		t.Fatalf("expected YAML web.chat.history_window, got %v", cfg.Web.Chat.HistoryWindow)
	}
}

func TestLoadReadsWebInfoFileFromYAML(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  info:
    file: ./site-info.md
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
	if cfg.Web.Info.File != "./site-info.md" {
		t.Fatalf("expected YAML web.info.file, got %q", cfg.Web.Info.File)
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

func TestValidateUpdateCheckAcceptsGitHubSource(t *testing.T) {
	err := validateUpdateCheck(UpdateCheckConfig{
		Enabled: true,
		Sources: []UpdateCheckSourceConfig{
			{
				Name:       "meshtastic-firmware",
				Type:       "github",
				Repository: "meshtastic/firmware",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected github update source to pass validation: %v", err)
	}
}

func TestValidateUpdateCheckRejectsUnknownSourceType(t *testing.T) {
	err := validateUpdateCheck(UpdateCheckConfig{
		Enabled: true,
		Sources: []UpdateCheckSourceConfig{
			{
				Name:       "unknown",
				Type:       "gitlab",
				Repository: "owner/repo",
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown update source type to fail validation")
	}
}

func TestLoadUpdateCheckSourceFromEnv(t *testing.T) {
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

	t.Setenv("MML_UPDATE_CHECK__ENABLED", "true")
	t.Setenv("MML_UPDATE_CHECK__INTERVAL", "6h")
	t.Setenv("MML_UPDATE_CHECK__TIMEOUT", "20s")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__NAME", "meshtastic-firmware")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__LABEL", "Meshtastic FW")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__TYPE", "github")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__REPOSITORY", "meshtastic/firmware")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__CURRENT_VERSION_SOURCE", "none")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__LIMIT", "25")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__PRE_RELEASES", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.UpdateCheck.Enabled {
		t.Fatal("expected update_check.enabled env override")
	}
	if cfg.UpdateCheck.Interval != 6*time.Hour {
		t.Fatalf("expected update_check.interval env override, got %v", cfg.UpdateCheck.Interval)
	}
	if cfg.UpdateCheck.Timeout != 20*time.Second {
		t.Fatalf("expected update_check.timeout env override, got %v", cfg.UpdateCheck.Timeout)
	}
	if len(cfg.UpdateCheck.Sources) != 1 {
		t.Fatalf("expected one update check source, got %d", len(cfg.UpdateCheck.Sources))
	}
	src := cfg.UpdateCheck.Sources[0]
	if src.Name != "meshtastic-firmware" ||
		src.Label != "Meshtastic FW" ||
		src.Type != "github" ||
		src.Repository != "meshtastic/firmware" ||
		src.CurrentVersionSource != "none" ||
		src.Limit != 25 ||
		!src.PreReleases {
		t.Fatalf("unexpected update check source from env: %#v", src)
	}
}

func TestLoadUpdateCheckSourcePreReleasesFromYAML(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  LongFast:
    psk: AQ==
update_check:
  enabled: true
  sources:
    - name: meshtastic-firmware
      type: github
      repository: meshtastic/firmware
      pre_releases: true
    - name: meshmap-lite
      type: forgejo
      base_url: https://git.example.invalid
      repository: skobkin/meshmap-lite
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected Load error: %v", err)
	}
	if len(cfg.UpdateCheck.Sources) != 2 {
		t.Fatalf("expected two update check sources, got %d", len(cfg.UpdateCheck.Sources))
	}
	if !cfg.UpdateCheck.Sources[0].PreReleases {
		t.Fatalf("expected github source to have pre-releases=true from YAML, got %#v", cfg.UpdateCheck.Sources[0])
	}
	if cfg.UpdateCheck.Sources[1].PreReleases {
		t.Fatalf("expected forgejo source to have pre-releases=false by default, got %#v", cfg.UpdateCheck.Sources[1])
	}
	if !cfg.UpdateCheck.Sources[1].PostProcessEnabled() {
		t.Fatalf("expected post_process to default enabled, got %#v", cfg.UpdateCheck.Sources[1])
	}
}

func TestLoadUpdateCheckSourcePostProcessFromYAML(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
channels:
  LongFast:
    psk: AQ==
update_check:
  enabled: true
  sources:
    - name: meshmap-lite
      type: forgejo
      base_url: https://git.example.invalid
      repository: skobkin/meshmap-lite
      post_process: false
    - name: meshtastic-firmware
      type: github
      repository: meshtastic/firmware
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected Load error: %v", err)
	}
	if len(cfg.UpdateCheck.Sources) != 2 {
		t.Fatalf("expected two update check sources, got %d", len(cfg.UpdateCheck.Sources))
	}
	if cfg.UpdateCheck.Sources[0].PostProcess == nil || cfg.UpdateCheck.Sources[0].PostProcessEnabled() {
		t.Fatalf("expected yaml post_process=false, got %#v", cfg.UpdateCheck.Sources[0])
	}
	if !cfg.UpdateCheck.Sources[1].PostProcessEnabled() {
		t.Fatalf("expected omitted yaml post_process to default enabled, got %#v", cfg.UpdateCheck.Sources[1])
	}
}

func TestLoadUpdateCheckSourcesFromEnvOnly(t *testing.T) {
	t.Setenv("MML_MQTT__ROOT_TOPIC", "msh/env")
	t.Setenv("MML_CHANNELS__LongFast__PSK", "AQ==")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__NAME", "meshmap-lite")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__LABEL", "Map")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__TYPE", "forgejo")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__BASE_URL", "https://git.example.invalid")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__REPOSITORY", "skobkin/meshmap-lite")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__0__CURRENT_VERSION_SOURCE", "buildinfo")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__NAME", "meshtastic-firmware")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__LABEL", "Meshtastic FW")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__TYPE", "github")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__REPOSITORY", "meshtastic/firmware")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__CURRENT_VERSION_SOURCE", "none")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__LIMIT", "50")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__POST_PROCESS", "false")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.UpdateCheck.Sources) != 2 {
		t.Fatalf("expected two update check sources, got %d", len(cfg.UpdateCheck.Sources))
	}
	if got := cfg.UpdateCheck.Sources[0]; got.Name != "meshmap-lite" || got.Type != "forgejo" || got.BaseURL != "https://git.example.invalid" {
		t.Fatalf("unexpected first update check source: %#v", got)
	}
	if got := cfg.UpdateCheck.Sources[1]; got.Name != "meshtastic-firmware" || got.Type != "github" || got.Repository != "meshtastic/firmware" || got.Limit != 50 {
		t.Fatalf("unexpected second update check source: %#v", got)
	}
	if cfg.UpdateCheck.Sources[1].PostProcess == nil || cfg.UpdateCheck.Sources[1].PostProcessEnabled() {
		t.Fatalf("expected env-only source post_process=false, got %#v", cfg.UpdateCheck.Sources[1])
	}
}

func TestLoadUpdateCheckSourceFromEnvRejectsSparseIndexes(t *testing.T) {
	t.Setenv("MML_MQTT__ROOT_TOPIC", "msh/env")
	t.Setenv("MML_CHANNELS__LongFast__PSK", "AQ==")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__NAME", "meshtastic-firmware")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__TYPE", "github")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__1__REPOSITORY", "meshtastic/firmware")

	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || !strings.Contains(err.Error(), "update_check.sources[0].name is required") {
		t.Fatalf("expected sparse update source indexes to fail validation, got %v", err)
	}
}

func TestLoadUpdateCheckSourceFromEnvRejectsNonNumericIndex(t *testing.T) {
	t.Setenv("MML_MQTT__ROOT_TOPIC", "msh/env")
	t.Setenv("MML_CHANNELS__LongFast__PSK", "AQ==")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__foo__NAME", "meshtastic-firmware")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__foo__TYPE", "github")
	t.Setenv("MML_UPDATE_CHECK__SOURCES__foo__REPOSITORY", "meshtastic/firmware")

	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || !strings.Contains(err.Error(), `update_check.sources index "foo" must be a non-negative integer`) {
		t.Fatalf("expected non-numeric update source index to fail env loading, got %v", err)
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

func TestLoadNormalizesStatsActivityBuckets(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  stats:
    activity:
      daily:
        bucket: 0s
      weekly:
        bucket: 0s
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
	if cfg.Web.Stats.Activity.Daily.Bucket != 5*time.Minute {
		t.Fatalf("expected default daily bucket, got %v", cfg.Web.Stats.Activity.Daily.Bucket)
	}
	if cfg.Web.Stats.Activity.Weekly.Bucket != time.Hour {
		t.Fatalf("expected default weekly bucket, got %v", cfg.Web.Stats.Activity.Weekly.Bucket)
	}
}

func TestLoadReadsRelevanceFromYAML(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  relevance:
    telemetry_max_age: 12h
    topology_evidence_max_age: 48h
    map_position_max_age: 168h
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
	if cfg.Web.Relevance.TelemetryMaxAge != 12*time.Hour {
		t.Fatalf("expected YAML telemetry max age, got %v", cfg.Web.Relevance.TelemetryMaxAge)
	}
	if cfg.Web.Relevance.TopologyEvidenceMaxAge != 48*time.Hour {
		t.Fatalf("expected YAML topology evidence max age, got %v", cfg.Web.Relevance.TopologyEvidenceMaxAge)
	}
	if cfg.Web.Relevance.MapPositionMaxAge != 7*24*time.Hour {
		t.Fatalf("expected YAML map position max age, got %v", cfg.Web.Relevance.MapPositionMaxAge)
	}
}

func TestLoadNormalizesInvalidRelevanceDurations(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  relevance:
    telemetry_max_age: 0s
    topology_evidence_max_age: 0s
    map_position_max_age: 0s
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
	if cfg.Web.Relevance.TelemetryMaxAge != 24*time.Hour {
		t.Fatalf("expected default telemetry max age, got %v", cfg.Web.Relevance.TelemetryMaxAge)
	}
	if cfg.Web.Relevance.TopologyEvidenceMaxAge != 72*time.Hour {
		t.Fatalf("expected default topology evidence max age, got %v", cfg.Web.Relevance.TopologyEvidenceMaxAge)
	}
	if cfg.Web.Relevance.MapPositionMaxAge != 14*24*time.Hour {
		t.Fatalf("expected default map position max age, got %v", cfg.Web.Relevance.MapPositionMaxAge)
	}
}

func TestLoadNormalizesInvalidTopologyCacheTTL(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  map:
    topology_cache_ttl: 0s
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
	if cfg.Web.Map.TopologyCacheTTL != 10*time.Minute {
		t.Fatalf("expected default topology cache ttl, got %v", cfg.Web.Map.TopologyCacheTTL)
	}
}

func TestLoadNormalizesInvalidTopologyMaxEdges(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  map:
    topology_max_edges: 0
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
	if cfg.Web.Map.TopologyMaxEdges != 2000 {
		t.Fatalf("expected default topology max edges, got %d", cfg.Web.Map.TopologyMaxEdges)
	}
}

func TestLoadNormalizesInvalidChatHistoryWindow(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		d := t.TempDir()
		path := filepath.Join(d, "cfg.yaml")
		if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  chat:
    history_window: 0s
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
		if cfg.Web.Chat.HistoryWindow != 7*24*time.Hour {
			t.Fatalf("expected default chat history window, got %v", cfg.Web.Chat.HistoryWindow)
		}
	})

	t.Run("env", func(t *testing.T) {
		d := t.TempDir()
		path := filepath.Join(d, "cfg.yaml")
		if err := os.WriteFile(path, []byte(`
mqtt:
  root_topic: msh/test
web:
  chat:
    history_window: 48h
channels:
  LongFast:
    psk: AQ==
`), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MML_WEB__CHAT__HISTORY_WINDOW", "not-a-duration")

		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "decode config") {
			t.Fatalf("expected invalid env chat history window to fail config decode, got %v", err)
		}
	})
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

func TestLoadDefaultMapViewFromEnvOnly(t *testing.T) {
	t.Setenv("MML_MQTT__ROOT_TOPIC", "msh/env")
	t.Setenv("MML_CHANNELS__LongFast__PSK", "AQ==")
	t.Setenv("MML_WEB__MAP__DEFAULT_VIEW__LATITUDE", "64.54678")
	t.Setenv("MML_WEB__MAP__DEFAULT_VIEW__LONGITUDE", "40.52470")
	t.Setenv("MML_WEB__MAP__DEFAULT_VIEW__ZOOM", "13")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Map.DefaultView.Latitude != 64.54678 {
		t.Fatalf("expected default view latitude from env, got %f", cfg.Web.Map.DefaultView.Latitude)
	}
	if cfg.Web.Map.DefaultView.Longitude != 40.52470 {
		t.Fatalf("expected default view longitude from env, got %f", cfg.Web.Map.DefaultView.Longitude)
	}
	if cfg.Web.Map.DefaultView.Zoom != 13 {
		t.Fatalf("expected default view zoom from env, got %d", cfg.Web.Map.DefaultView.Zoom)
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

func TestLoadInvalidEnvValuesFailDecode(t *testing.T) {
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

	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "decode config") {
		t.Fatalf("expected invalid env values to fail config decode, got %v", err)
	}
}

func TestLoadRejectsOutOfRangeSubscribeQoSFromEnv(t *testing.T) {
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
	t.Setenv("MML_MQTT__SUBSCRIBE_QOS", "300")

	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "decode config") {
		t.Fatalf("expected out-of-range subscribe qos env value to fail config decode, got %v", err)
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
