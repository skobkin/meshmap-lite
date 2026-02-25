package migrations

import (
	"context"
	"database/sql"
)

func migrateV1BootstrapCoreSchema(ctx context.Context, tx *sql.Tx) error {
	return applyStatements(ctx, tx, "bootstrap_core_schema", []string{
		`CREATE TABLE IF NOT EXISTS nodes (
  node_id TEXT PRIMARY KEY,
  node_num INTEGER,
  long_name TEXT,
  short_name TEXT,
  role TEXT,
  board_model TEXT,
  firmware_version TEXT,
  lora_region TEXT,
  lora_frequency_desc TEXT,
  modem_preset TEXT,
  neighbor_nodes_count INTEGER,
  mqtt_gateway_capable INTEGER,
  first_seen_at TEXT NOT NULL,
  last_seen_any_event_at TEXT NOT NULL,
  last_seen_mqtt_gateway_at TEXT,
  last_seen_position_at TEXT,
  updated_at TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS node_positions (
  node_id TEXT PRIMARY KEY REFERENCES nodes(node_id) ON DELETE CASCADE,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  altitude_m REAL,
  position_precision INTEGER,
  source_kind TEXT NOT NULL,
  source_channel TEXT,
  reported_at TEXT,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS node_telemetry_snapshots (
  node_id TEXT PRIMARY KEY REFERENCES nodes(node_id) ON DELETE CASCADE,
  power_voltage REAL,
  power_battery_level REAL,
  env_temperature_c REAL,
  env_humidity REAL,
  env_pressure_hpa REAL,
  air_pm25 REAL,
  air_pm10 REAL,
  air_co2 REAL,
  source_channel TEXT,
  reported_at TEXT,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS chat_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type TEXT NOT NULL,
  channel_name TEXT,
  node_id TEXT,
  message_text TEXT,
  system_code TEXT,
  message_time TEXT NOT NULL,
  reported_at TEXT,
  observed_at TEXT NOT NULL,
  packet_id INTEGER,
  created_at TEXT NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_channel_id ON chat_events(channel_name, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_packet_id ON chat_events(packet_id);`,
	})
}
