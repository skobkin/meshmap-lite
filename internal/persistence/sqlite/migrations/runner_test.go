package migrations

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApply_MigratesLegacyChatEvents(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY
);
CREATE TABLE chat_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type TEXT NOT NULL,
  channel_name TEXT,
  node_id TEXT,
  sender_display TEXT,
  message_text TEXT NOT NULL,
  message_time TEXT NOT NULL,
  reported_at TEXT,
  observed_at TEXT NOT NULL,
  packet_id INTEGER,
  created_at TEXT NOT NULL
);
INSERT INTO chat_events(event_type,channel_name,node_id,sender_display,message_text,message_time,observed_at,created_at)
VALUES
  ('system','longfast',NULL,'!abc12345','New node discovered!','2026-02-25T00:00:00Z','2026-02-25T00:00:00Z','2026-02-25T00:00:00Z'),
  ('message','longfast','!def67890','legacy-sender','hello mesh','2026-02-25T00:01:00Z','2026-02-25T00:01:00Z','2026-02-25T00:01:00Z');
`)
	if err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	hasSenderDisplay, err := tableHasColumn(ctx, db, "chat_events", "sender_display")
	if err != nil {
		t.Fatalf("check sender_display column: %v", err)
	}
	if hasSenderDisplay {
		t.Fatalf("sender_display column should be removed")
	}
	hasSystemCode, err := tableHasColumn(ctx, db, "chat_events", "system_code")
	if err != nil {
		t.Fatalf("check system_code column: %v", err)
	}
	if !hasSystemCode {
		t.Fatalf("system_code column should exist")
	}

	var nodeID, messageText, systemCode sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT node_id,message_text,system_code FROM chat_events WHERE event_type='system' LIMIT 1`).Scan(&nodeID, &messageText, &systemCode); err != nil {
		t.Fatalf("read migrated system event: %v", err)
	}
	if nodeID.String != "!abc12345" {
		t.Fatalf("expected migrated node_id from sender_display, got %q", nodeID.String)
	}
	if messageText.Valid {
		t.Fatalf("expected system message_text to be NULL, got %q", messageText.String)
	}
	if systemCode.String != "node_discovered" {
		t.Fatalf("expected system_code node_discovered, got %q", systemCode.String)
	}

	var msgText, msgCode sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT message_text,system_code FROM chat_events WHERE event_type='message' LIMIT 1`).Scan(&msgText, &msgCode); err != nil {
		t.Fatalf("read migrated message event: %v", err)
	}
	if msgText.String != "hello mesh" {
		t.Fatalf("expected chat text preserved, got %q", msgText.String)
	}
	if msgCode.Valid {
		t.Fatalf("expected message system_code to be NULL, got %q", msgCode.String)
	}
}

func TestApply_NormalizesDefaultChannelNames(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY
);
CREATE TABLE node_positions (
  node_id TEXT PRIMARY KEY REFERENCES nodes(node_id) ON DELETE CASCADE,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  source_kind TEXT NOT NULL,
  source_channel TEXT,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE node_telemetry_snapshots (
  node_id TEXT PRIMARY KEY REFERENCES nodes(node_id) ON DELETE CASCADE,
  source_channel TEXT,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE chat_events (
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
);
INSERT INTO nodes(node_id) VALUES ('!00000001');
INSERT INTO node_positions(node_id,latitude,longitude,source_kind,source_channel,observed_at,updated_at)
VALUES ('!00000001',64.5,40.6,'channel_position','longfast','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z');
INSERT INTO node_telemetry_snapshots(node_id,source_channel,observed_at,updated_at)
VALUES ('!00000001','mediumslow','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z');
INSERT INTO chat_events(event_type,channel_name,node_id,message_text,message_time,observed_at,created_at)
VALUES
  ('message','shortfast','!00000001','a','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z'),
  ('message','pingpong','!00000001','b','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var posChannel string
	if err := db.QueryRowContext(ctx, `SELECT source_channel FROM node_positions WHERE node_id='!00000001'`).Scan(&posChannel); err != nil {
		t.Fatalf("read node_positions: %v", err)
	}
	if posChannel != "LongFast" {
		t.Fatalf("expected LongFast, got %q", posChannel)
	}

	var telChannel string
	if err := db.QueryRowContext(ctx, `SELECT source_channel FROM node_telemetry_snapshots WHERE node_id='!00000001'`).Scan(&telChannel); err != nil {
		t.Fatalf("read node_telemetry_snapshots: %v", err)
	}
	if telChannel != "MediumSlow" {
		t.Fatalf("expected MediumSlow, got %q", telChannel)
	}

	var chatDefault string
	if err := db.QueryRowContext(ctx, `SELECT channel_name FROM chat_events WHERE message_text='a'`).Scan(&chatDefault); err != nil {
		t.Fatalf("read chat default: %v", err)
	}
	if chatDefault != "ShortFast" {
		t.Fatalf("expected ShortFast, got %q", chatDefault)
	}

	var chatCustom string
	if err := db.QueryRowContext(ctx, `SELECT channel_name FROM chat_events WHERE message_text='b'`).Scan(&chatCustom); err != nil {
		t.Fatalf("read chat custom: %v", err)
	}
	if chatCustom != "pingpong" {
		t.Fatalf("expected custom channel unchanged, got %q", chatCustom)
	}
}

func TestApply_AddsMapReportFlagsColumns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY,
  long_name TEXT
);`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	hasDefaultChannel, err := tableHasColumn(ctx, db, "nodes", "has_default_channel")
	if err != nil {
		t.Fatalf("check has_default_channel column: %v", err)
	}
	if !hasDefaultChannel {
		t.Fatalf("has_default_channel column should exist")
	}

	hasOptedReportLocation, err := tableHasColumn(ctx, db, "nodes", "has_opted_report_location")
	if err != nil {
		t.Fatalf("check has_opted_report_location column: %v", err)
	}
	if !hasOptedReportLocation {
		t.Fatalf("has_opted_report_location column should exist")
	}
}

func TestApply_AddsLogTables(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY
);`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	hasLogChannels, err := tableExists(ctx, db, "log_channels")
	if err != nil {
		t.Fatalf("check log_channels table: %v", err)
	}
	if !hasLogChannels {
		t.Fatalf("log_channels table should exist")
	}

	hasLogEvents, err := tableExists(ctx, db, "log_events")
	if err != nil {
		t.Fatalf("check log_events table: %v", err)
	}
	if !hasLogEvents {
		t.Fatalf("log_events table should exist")
	}
}

func TestApply_AddsTopologyEdgesTable(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY
);`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	hasTopologyEdges, err := tableExists(ctx, db, "topology_edges")
	if err != nil {
		t.Fatalf("check topology_edges table: %v", err)
	}
	if !hasTopologyEdges {
		t.Fatalf("topology_edges table should exist")
	}
}

func TestApply_WidensTopologySourceKindConstraint(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 12;
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY
);
INSERT INTO nodes(node_id) VALUES ('!49b5976c'), ('!11223344');
CREATE TABLE topology_edges (
  source_kind INTEGER NOT NULL,
  channel_name TEXT NOT NULL DEFAULT '',
  from_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  to_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  reported_by_node_id TEXT REFERENCES nodes(node_id) ON DELETE SET NULL,
  inferred INTEGER NOT NULL DEFAULT 0,
  snr REAL,
  neighbor_last_rx_at TEXT,
  neighbor_broadcast_interval_secs INTEGER,
  first_observed_at TEXT NOT NULL,
  last_observed_at TEXT NOT NULL,
  last_reported_at TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (source_kind, channel_name, from_node_id, to_node_id),
  CHECK (source_kind BETWEEN 1 AND 5),
  CHECK (inferred IN (0, 1))
);
INSERT INTO topology_edges(source_kind,channel_name,from_node_id,to_node_id,first_observed_at,last_observed_at,updated_at)
VALUES (1, 'LongFast', '!49b5976c', '!11223344', '2026-05-10T10:00:00Z', '2026-05-10T10:00:00Z', '2026-05-10T10:00:00Z');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO topology_edges(source_kind,channel_name,from_node_id,to_node_id,first_observed_at,last_observed_at,updated_at) VALUES (6, 'LongFast', '!49b5976c', '!11223344', '2026-05-10T10:01:00Z', '2026-05-10T10:01:00Z', '2026-05-10T10:01:00Z')`); err != nil {
		t.Fatalf("expected source_kind 6 to be accepted: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM topology_edges`).Scan(&count); err != nil {
		t.Fatalf("count topology rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected preserved row and inserted source_kind 6 row, got %d", count)
	}
}

func TestApply_ReclassifiesRangeTestLogEvents(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 7;
CREATE TABLE log_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER REFERENCES log_channels(id) ON DELETE SET NULL,
  details_json TEXT,
  CHECK (event_kind BETWEEN 1 AND 9),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);
INSERT INTO log_channels(id, name) VALUES (1, 'LongFast');
INSERT INTO log_events(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES
  (1, '2026-03-23T10:00:00Z', '!range000', 8, 0, 1, '{"portnum_value":66,"portnum_name":"RANGE_TEST_APP"}'),
  (2, '2026-03-23T10:01:00Z', '!other000', 8, 0, 1, '{"portnum_value":65,"portnum_name":"SERIAL_APP"}');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var eventKind int
	var details sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT event_kind, details_json FROM log_events WHERE id = 1`).Scan(&eventKind, &details); err != nil {
		t.Fatalf("read migrated range test row: %v", err)
	}
	if eventKind != 10 {
		t.Fatalf("expected range test kind 10, got %d", eventKind)
	}
	if details.Valid {
		t.Fatalf("expected migrated range test details to be cleared, got %q", details.String)
	}

	if err := db.QueryRowContext(ctx, `SELECT event_kind, details_json FROM log_events WHERE id = 2`).Scan(&eventKind, &details); err != nil {
		t.Fatalf("read preserved other row: %v", err)
	}
	if eventKind != 8 {
		t.Fatalf("expected other-portnum kind 8, got %d", eventKind)
	}
	if !details.Valid || details.String == "" {
		t.Fatalf("expected other-portnum details preserved")
	}
}

func TestApply_LogsEachMigrationAtInfo(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY
);`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := Apply(ctx, db, logger); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "msg=\"applying sqlite migration\"") {
		t.Fatalf("expected migration log message, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "version=6") || !strings.Contains(logOutput, "name=log_events") {
		t.Fatalf("expected log entry for log_events migration, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "version=8") || !strings.Contains(logOutput, "name=log_event_range_test_kind") {
		t.Fatalf("expected log entry for range test migration, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "version=9") || !strings.Contains(logOutput, "name=log_event_pki_kind") {
		t.Fatalf("expected log entry for PKI migration, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "version=10") || !strings.Contains(logOutput, "name=activity_indexes") {
		t.Fatalf("expected log entry for activity indexes migration, got %q", logOutput)
	}
}

func TestApply_AllowsPKILogEventKind(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 8;
CREATE TABLE log_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER REFERENCES log_channels(id) ON DELETE SET NULL,
  details_json TEXT,
  CHECK (event_kind BETWEEN 1 AND 10),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);
INSERT INTO log_channels(id, name) VALUES (1, 'PKI');
INSERT INTO log_events(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES
  (1, '2026-03-23T10:00:00Z', '!opaque001', 9, 1, 1, NULL);
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO log_events(observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES (?, ?, ?, ?, ?, ?)`,
		"2026-03-23T10:01:00Z", "!opaque002", 11, 1, 1, `{"pki_encrypted":true}`); err != nil {
		t.Fatalf("expected event kind 11 to be accepted, got %v", err)
	}
}

func TestApply_AddsActivityIndexes(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 9;
CREATE TABLE chat_events (
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
);
CREATE TABLE log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER,
  details_json TEXT
);
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for _, name := range []string{"idx_chat_events_type_observed_at", "idx_log_events_kind_observed_at"} {
		exists, err := indexExists(ctx, db, name)
		if err != nil {
			t.Fatalf("check index %s: %v", name, err)
		}
		if !exists {
			t.Fatalf("expected index %s to exist", name)
		}
	}
}

func TestApply_AddsMQTTUploaderProvenanceColumns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 10;
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY,
  first_seen_at TEXT NOT NULL,
  last_seen_any_event_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE node_positions (
  node_id TEXT PRIMARY KEY,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  source_kind TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE node_telemetry_snapshots (
  node_id TEXT PRIMARY KEY,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE chat_events (
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
);
CREATE TABLE log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER,
  details_json TEXT
);
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for _, tc := range []struct {
		table  string
		column string
	}{
		{"nodes", "last_mqtt_uploader_node_id"},
		{"nodes", "last_mqtt_uploader_at"},
		{"node_positions", "mqtt_uploader_node_id"},
		{"node_telemetry_snapshots", "mqtt_uploader_node_id"},
		{"chat_events", "mqtt_uploader_node_id"},
		{"log_events", "mqtt_uploader_node_id"},
	} {
		hasColumn, err := tableHasColumn(ctx, db, tc.table, tc.column)
		if err != nil {
			t.Fatalf("check column %s.%s: %v", tc.table, tc.column, err)
		}
		if !hasColumn {
			t.Fatalf("expected column %s.%s", tc.table, tc.column)
		}
	}
}

func TestApply_DiscardsZeroPositions(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 13;
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY,
  first_seen_at TEXT NOT NULL,
  last_seen_any_event_at TEXT NOT NULL,
  last_seen_position_at TEXT,
  updated_at TEXT NOT NULL
);
CREATE TABLE node_positions (
  node_id TEXT PRIMARY KEY,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  source_kind TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
INSERT INTO nodes(node_id,first_seen_at,last_seen_any_event_at,last_seen_position_at,updated_at)
VALUES
  ('!zero0001','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!lat00002','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!lon00003','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!valid004','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!stale005','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z');
INSERT INTO node_positions(node_id,latitude,longitude,source_kind,observed_at,updated_at)
VALUES
  ('!zero0001',0,0,'channel_position','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!lat00002',0,40.6,'channel_position','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!lon00003',64.5,0,'channel_position','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z'),
  ('!valid004',64.5,40.6,'channel_position','2026-05-10T00:00:00Z','2026-05-10T00:00:00Z');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var zeroCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_positions WHERE latitude = 0 AND longitude = 0`).Scan(&zeroCount); err != nil {
		t.Fatalf("count zero positions: %v", err)
	}
	if zeroCount != 0 {
		t.Fatalf("expected zero positions to be removed, got %d", zeroCount)
	}

	for _, nodeID := range []string{"!lat00002", "!lon00003", "!valid004"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_positions WHERE node_id = ?`, nodeID).Scan(&count); err != nil {
			t.Fatalf("count position for %s: %v", nodeID, err)
		}
		if count != 1 {
			t.Fatalf("expected position for %s to be preserved, got %d", nodeID, count)
		}
	}

	var zeroLastSeen sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT last_seen_position_at FROM nodes WHERE node_id = '!zero0001'`).Scan(&zeroLastSeen); err != nil {
		t.Fatalf("read zero node timestamp: %v", err)
	}
	if zeroLastSeen.Valid {
		t.Fatalf("expected zero node last_seen_position_at to be cleared, got %q", zeroLastSeen.String)
	}

	var staleLastSeen sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT last_seen_position_at FROM nodes WHERE node_id = '!stale005'`).Scan(&staleLastSeen); err != nil {
		t.Fatalf("read stale node timestamp: %v", err)
	}
	if !staleLastSeen.Valid {
		t.Fatalf("expected unaffected stale timestamp to remain set")
	}
}

func tableHasColumn(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}

	return false, rows.Err()
}

func indexExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func TestApply_AddsHopTrackingColumns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 14;
CREATE TABLE chat_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type TEXT NOT NULL,
  message_text TEXT,
  message_time TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL
);
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for _, tc := range []struct {
		table  string
		column string
	}{
		{"chat_events", "hop_start"},
		{"chat_events", "hop_limit"},
		{"log_events", "hop_start"},
		{"log_events", "hop_limit"},
	} {
		hasColumn, err := tableHasColumn(ctx, db, tc.table, tc.column)
		if err != nil {
			t.Fatalf("check column %s.%s: %v", tc.table, tc.column, err)
		}
		if !hasColumn {
			t.Fatalf("expected column %s.%s", tc.table, tc.column)
		}
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("re-apply migrations on already-current schema: %v", err)
	}
}
