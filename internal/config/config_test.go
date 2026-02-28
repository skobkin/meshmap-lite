package config

import (
	"os"
	"path/filepath"
	"testing"
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
