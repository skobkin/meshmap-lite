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
}
