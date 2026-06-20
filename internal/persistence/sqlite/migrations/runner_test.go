package migrations

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"reflect"
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
  (2, '2026-03-23T10:01:00Z', '!other000', 8, 0, 1, '{"portnum_value":32,"portnum_name":"SERIAL_APP"}');
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
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER,
  details_json TEXT,
  mqtt_uploader_node_id TEXT
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

func TestApply_AddsChatReactionColumns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 15;
CREATE TABLE chat_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type TEXT NOT NULL,
  message_text TEXT,
  message_time TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  created_at TEXT NOT NULL
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
		{"chat_events", "reaction_emoji"},
		{"chat_events", "reply_to_packet_id"},
	} {
		hasColumn, err := tableHasColumn(ctx, db, tc.table, tc.column)
		if err != nil {
			t.Fatalf("check column %s.%s: %v", tc.table, tc.column, err)
		}
		if !hasColumn {
			t.Fatalf("expected column %s.%s", tc.table, tc.column)
		}
	}

	// Re-applying migrations must be a no-op (idempotent on existing columns).
	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("re-apply migrations on already-current schema: %v", err)
	}
}

func TestApply_BackfillsLogVisibleNodes(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 16;
CREATE TABLE nodes (
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
  has_default_channel INTEGER,
  has_opted_report_location INTEGER,
  neighbor_nodes_count INTEGER,
  mqtt_gateway_capable INTEGER,
  first_seen_at TEXT NOT NULL,
  last_seen_any_event_at TEXT NOT NULL,
  last_seen_mqtt_gateway_at TEXT,
  last_mqtt_uploader_node_id TEXT,
  last_mqtt_uploader_at TEXT,
  last_seen_position_at TEXT,
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
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  reaction_emoji TEXT,
  reply_to_packet_id INTEGER,
  created_at TEXT NOT NULL
);
CREATE TABLE log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER,
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  details_json TEXT
);
INSERT INTO nodes(node_id,long_name,first_seen_at,last_seen_any_event_at,updated_at)
VALUES ('!existing', 'Existing node', '2026-06-01T00:00:00Z', '2026-06-01T00:00:00Z', '2026-06-01T00:00:00Z');
INSERT INTO chat_events(event_type,channel_name,node_id,message_text,message_time,observed_at,mqtt_uploader_node_id,created_at)
VALUES ('message','LongSlow','!chat001','hello','2026-06-16T10:00:00Z','2026-06-16T10:00:00Z','!chatgw1','2026-06-16T10:00:00Z');
INSERT INTO log_events(observed_at,node_id,event_kind,encrypted,mqtt_uploader_node_id,details_json)
VALUES
  ('2026-06-16T10:01:00Z','!log0001',7,0,'!loggw01','{"neighbor_node_id":"!nbrmain","neighbors":[{"node_id":"!nbr0001","snr":12.5}]}'),
  ('2026-06-16T10:02:00Z','!existing',6,0,NULL,'{"from":"!from001","to":"!to00001","route":["!route01"],"route_back":["!routeb1"],"forward_path":["!fwd0001"],"return_path":["!ret0001"]}'),
  ('2026-06-16T10:03:00Z',NULL,11,1,NULL,'{"sender_node_id":"!pki0001","destination_node_id":"!pki0002","gateway_id":"!pkigw01"}');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for _, nodeID := range []string{
		"!chat001", "!chatgw1", "!log0001", "!loggw01", "!nbrmain", "!nbr0001",
		"!from001", "!to00001", "!route01", "!routeb1", "!fwd0001", "!ret0001",
		"!pki0001", "!pki0002", "!pkigw01",
	} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes WHERE node_id = ?`, nodeID).Scan(&count); err != nil {
			t.Fatalf("count node %s: %v", nodeID, err)
		}
		if count != 1 {
			t.Fatalf("expected backfilled node %s, got count %d", nodeID, count)
		}
	}

	var longName, lastSeen string
	if err := db.QueryRowContext(ctx, `SELECT long_name,last_seen_any_event_at FROM nodes WHERE node_id='!existing'`).Scan(&longName, &lastSeen); err != nil {
		t.Fatalf("read existing node: %v", err)
	}
	if longName != "Existing node" {
		t.Fatalf("expected existing identity fields to be preserved, got %q", longName)
	}
	if lastSeen != "2026-06-16T10:02:00Z" {
		t.Fatalf("expected existing timestamp to be advanced, got %q", lastSeen)
	}
}

func TestApply_ReclassifiesStoreForwardLogEvents(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 17;
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
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  CHECK (event_kind BETWEEN 1 AND 11),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);
INSERT INTO log_channels(id, name) VALUES (1, 'LongFast');
INSERT INTO log_events(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json, mqtt_uploader_node_id, hop_start, hop_limit) VALUES
  (1, '2026-06-17T10:00:00Z', '!sf0001', 8, 0, 1, '{"portnum_value":65,"portnum_name":"STORE_FORWARD_APP"}', '!gateway1', 5, 3),
  (2, '2026-06-17T10:01:00Z', '!other0', 8, 0, 1, '{"portnum_value":32,"portnum_name":"SERIAL_APP"}', NULL, 0, 0),
  (3, '2026-06-17T10:02:00Z', '!sf0002', 8, 0, 1, '{"portnum_name":"STORE_FORWARD_APP","extra":"data"}', NULL, 0, 0);
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 20 {
		t.Fatalf("expected user_version=20, got %d", version)
	}

	var eventKind int
	var details sql.NullString
	var mqttUploader sql.NullString
	var hopStart, hopLimit sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT event_kind, details_json, mqtt_uploader_node_id, hop_start, hop_limit FROM log_events WHERE id = 1`).Scan(&eventKind, &details, &mqttUploader, &hopStart, &hopLimit); err != nil {
		t.Fatalf("read migrated store forward row: %v", err)
	}
	if eventKind != 12 {
		t.Fatalf("expected store forward kind 12, got %d", eventKind)
	}
	if details.Valid {
		t.Fatalf("expected migrated store forward details to be cleared, got %q", details.String)
	}
	if !mqttUploader.Valid || mqttUploader.String != "!gateway1" {
		t.Fatalf("expected mqtt_uploader_node_id preserved, got %#v", mqttUploader)
	}
	if !hopStart.Valid || hopStart.Int64 != 5 {
		t.Fatalf("expected hop_start preserved, got %#v", hopStart)
	}
	if !hopLimit.Valid || hopLimit.Int64 != 3 {
		t.Fatalf("expected hop_limit preserved, got %#v", hopLimit)
	}

	if err := db.QueryRowContext(ctx, `SELECT event_kind, details_json FROM log_events WHERE id = 2`).Scan(&eventKind, &details); err != nil {
		t.Fatalf("read preserved other-portnum row: %v", err)
	}
	if eventKind != 8 {
		t.Fatalf("expected other-portnum kind 8, got %d", eventKind)
	}
	if !details.Valid || details.String == "" {
		t.Fatalf("expected other-portnum details preserved")
	}

	if err := db.QueryRowContext(ctx, `SELECT event_kind FROM log_events WHERE id = 3`).Scan(&eventKind); err != nil {
		t.Fatalf("read second store forward row: %v", err)
	}
	if eventKind != 12 {
		t.Fatalf("expected store forward kind 12 for portnum_name match, got %d", eventKind)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO log_events(observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES (?, ?, ?, ?, ?, ?)`,
		"2026-06-17T10:03:00Z", "!sf0003", 12, 0, 1, `{"rr":"ROUTER_STATS","role":"router"}`); err != nil {
		t.Fatalf("expected event kind 12 to be accepted, got %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO log_events(observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES (?, ?, ?, ?, ?, ?)`,
		"2026-06-17T10:04:00Z", "!bogus01", 13, 0, 1, "NULL"); err == nil {
		t.Fatalf("expected event kind 13 to be rejected by CHECK")
	}
}

func TestApply_CompactsStoreForwardDetails(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Seed a v17-stamped DB (PRAGMA user_version = 17) that already
	// has the v18-extended CHECK bound in place. This simulates an
	// operator who ran v18 once, hand-rolled-back the schema version
	// without dropping the table, and is now re-applying v18 — so
	// kind=12 rows from the previous v18 run are still in the table
	// alongside the kind=8 rows that v18 should reclassify. The
	// compact-shaping pass must then walk those surviving kind=12
	// rows and rewrite them in place.
	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 17;
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
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  CHECK (event_kind BETWEEN 1 AND 12),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);
INSERT INTO log_channels(id, name) VALUES (1, 'LongFast');
INSERT INTO log_events(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES
  (1, '2026-06-17T10:00:00Z', '!sf0001', 8,  0, 1, '{"portnum_value":65,"portnum_name":"STORE_FORWARD_APP"}'),
  (2, '2026-06-17T10:01:00Z', '!sf0002', 8,  0, 1, '{"portnum_name":"STORE_FORWARD_APP","extra":"data"}'),
  (3, '2026-06-17T10:02:00Z', '!sf0003', 12, 0, 1, '{"rr":"ROUTER_STATS","role":"router","from":"!aabbccdd","stats":{"messages_total":42}}'),
  (4, '2026-06-17T10:03:00Z', '!sf0004', 12, 0, 1, '{"rr":7,"from":"!aabbccdd","stats":{"messages_total":42}}'),
  (5, '2026-06-17T10:04:00Z', '!sf0005', 12, 0, 1, '{"rr":"CLIENT_HISTORY","role":"client","text":"hello world"}'),
  (6, '2026-06-17T10:05:00Z', '!sf0006', 12, 0, 1, NULL),
  (7, '2026-06-17T10:06:00Z', '!sf0007', 12, 0, 1, '{"rr":"ROUTER_PING_PONG_2027"}');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 20 {
		t.Fatalf("expected user_version=20, got %d", version)
	}

	// Row 1: kind 8 with portnum_name=STORE_FORWARD_APP — promoted
	// to kind 12, details nulled (the renderer re-emits the
	// structured details on the new code path).
	assertEventKindAndDetailsNull(t, ctx, db, 1, 12)

	// Row 2: kind 8 with portnum_name=STORE_FORWARD_APP and extra
	// keys — same treatment.
	assertEventKindAndDetailsNull(t, ctx, db, 2, 12)

	// Row 3: kind 12 with legacy S&F details. v18 compacts in place:
	// string `rr` becomes int, `role` removed, `from` preserved,
	// sub-payload `stats` preserved.
	assertStoreForwardDetails(t, ctx, db, 3, func(d map[string]any) {
		rr, ok := d["rr"].(float64)
		if !ok || rr != 7 {
			t.Errorf("row 3: expected rr=7, got %#v", d["rr"])
		}
		if _, present := d["role"]; present {
			t.Errorf("row 3: role should be removed, got %#v", d["role"])
		}
		if d["from"] != "!aabbccdd" {
			t.Errorf("row 3: from not preserved: %#v", d["from"])
		}
		stats, ok := d["stats"].(map[string]any)
		if !ok || stats["messages_total"].(float64) != 42 {
			t.Errorf("row 3: stats not preserved: %#v", d["stats"])
		}
	})

	// Row 4: kind 12 already in the compact shape. Migration must be
	// idempotent.
	var details sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT details_json FROM log_events WHERE id = 4`).Scan(&details); err != nil {
		t.Fatalf("read idempotent row: %v", err)
	}
	if !details.Valid {
		t.Fatalf("row 4: expected details to remain, got NULL")
	}
	original := map[string]any{
		"from":  "!aabbccdd",
		"rr":    float64(7),
		"stats": map[string]any{"messages_total": float64(42)},
	}
	assertDetailsEqual(t, details.String, original)

	// Row 5: kind 12 legacy with `text` — expect rr=65, no `role`,
	// text_bytes=11 ("hello world").
	assertStoreForwardDetails(t, ctx, db, 5, func(d map[string]any) {
		rr, ok := d["rr"].(float64)
		if !ok || rr != 65 {
			t.Errorf("row 5: expected rr=65, got %#v", d["rr"])
		}
		if _, present := d["role"]; present {
			t.Errorf("row 5: role should be removed, got %#v", d["role"])
		}
		tb, ok := d["text_bytes"].(float64)
		if !ok || tb != 11 {
			t.Errorf("row 5: expected text_bytes=11, got %#v", d["text_bytes"])
		}
		if _, present := d["text"]; present {
			t.Errorf("row 5: text should be removed, got %#v", d["text"])
		}
	})

	// Row 6: kind 12 with NULL details — must remain NULL.
	if err := db.QueryRowContext(ctx, `SELECT details_json FROM log_events WHERE id = 6`).Scan(&details); err != nil {
		t.Fatalf("read null row: %v", err)
	}
	if details.Valid {
		t.Errorf("row 6: expected details to remain NULL, got %q", details.String)
	}

	// Row 7: kind 12 with an RR value the pinned proto does not know
	// about — the migration preserves the original string in
	// `raw_rr` and writes `rr: -1` (the Go-side sentinel) so future
	// firmware codes never silently lose data.
	assertStoreForwardDetails(t, ctx, db, 7, func(d map[string]any) {
		rr, ok := d["rr"].(float64)
		if !ok || rr != -1 {
			t.Errorf("row 7: expected rr=-1, got %#v", d["rr"])
		}
		if d["raw_rr"] != "ROUTER_PING_PONG_2027" {
			t.Errorf("row 7: expected raw_rr=ROUTER_PING_PONG_2027, got %#v", d["raw_rr"])
		}
		if _, present := d["role"]; present {
			t.Errorf("row 7: role should be removed, got %#v", d["role"])
		}
	})
}

// TestApply_CompactsStoreForwardDetails_AdditionalShapes covers the
// migration's behaviour on shapes the main test does not exercise:
// compacting empty sub-payloads, the heartbeat sub-payload, the
// mixed legacy shape (int rr + role) and from/to preservation.
func TestApply_CompactsStoreForwardDetails_AdditionalShapes(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 17;
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
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  CHECK (event_kind BETWEEN 1 AND 12),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);
INSERT INTO log_channels(id, name) VALUES (1, 'LongFast');
INSERT INTO log_events(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES
  -- Row 1: all three sub-payloads empty/null — must all be dropped
  -- from the rewritten details (nonEmptyRaw filter).
  (1, '2026-06-17T10:00:00Z', '!sf0001', 12, 0, 1, '{"rr":"ROUTER_STATS","role":"router","stats":{},"history":null,"heartbeat":[]}'),
  -- Row 2: heartbeat sub-payload preserved verbatim.
  (2, '2026-06-17T10:01:00Z', '!sf0002', 12, 0, 1, '{"rr":"ROUTER_HEARTBEAT","role":"router","heartbeat":{"period":60,"secondary":false}}'),
  -- Row 3: mixed legacy — int rr AND role key. The role key alone
  -- is enough to flag the row as legacy; the int rr is preserved
  -- as-is (not re-mapped) and the role is stripped.
  (3, '2026-06-17T10:02:00Z', '!sf0003', 12, 0, 1, '{"rr":2,"role":"router","heartbeat":{"period":60}}'),
  -- Row 4: from and to both set — both must be preserved.
  (4, '2026-06-17T10:03:00Z', '!sf0004', 12, 0, 1, '{"rr":"ROUTER_STATS","role":"router","from":"!aabbccdd","to":"!11223344"}');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	if err := Apply(ctx, db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	// Row 1: empty sub-payloads are stripped from the rewritten JSON.
	assertStoreForwardDetails(t, ctx, db, 1, func(d map[string]any) {
		if _, present := d["stats"]; present {
			t.Errorf("row 1: empty stats object should be dropped, got %#v", d["stats"])
		}
		if _, present := d["history"]; present {
			t.Errorf("row 1: null history should be dropped, got %#v", d["history"])
		}
		if _, present := d["heartbeat"]; present {
			t.Errorf("row 1: empty heartbeat array should be dropped, got %#v", d["heartbeat"])
		}
		// rr=7 (ROUTER_STATS) and no role are still applied.
		rr, ok := d["rr"].(float64)
		if !ok || rr != 7 {
			t.Errorf("row 1: expected rr=7, got %#v", d["rr"])
		}
		if _, present := d["role"]; present {
			t.Errorf("row 1: role should be removed, got %#v", d["role"])
		}
	})

	// Row 2: heartbeat sub-payload round-trips through the rewrite.
	assertStoreForwardDetails(t, ctx, db, 2, func(d map[string]any) {
		rr, ok := d["rr"].(float64)
		if !ok || rr != 2 {
			t.Errorf("row 2: expected rr=2, got %#v", d["rr"])
		}
		hb, ok := d["heartbeat"].(map[string]any)
		if !ok {
			t.Fatalf("row 2: heartbeat sub-payload missing: %#v", d["heartbeat"])
		}
		if hb["period"].(float64) != 60 {
			t.Errorf("row 2: heartbeat.period not preserved: %#v", hb["period"])
		}
		if hb["secondary"].(bool) != false {
			t.Errorf("row 2: heartbeat.secondary not preserved: %#v", hb["secondary"])
		}
		if _, present := d["role"]; present {
			t.Errorf("row 2: role should be removed, got %#v", d["role"])
		}
	})

	// Row 3: mixed legacy shape — int rr preserved verbatim, role removed.
	assertStoreForwardDetails(t, ctx, db, 3, func(d map[string]any) {
		rr, ok := d["rr"].(float64)
		if !ok || rr != 2 {
			t.Errorf("row 3: int rr should be preserved (got %#v, want 2)", d["rr"])
		}
		if _, present := d["role"]; present {
			t.Errorf("row 3: role should be removed, got %#v", d["role"])
		}
	})

	// Row 4: both from and to preserved.
	assertStoreForwardDetails(t, ctx, db, 4, func(d map[string]any) {
		if d["from"] != "!aabbccdd" {
			t.Errorf("row 4: from not preserved: %#v", d["from"])
		}
		if d["to"] != "!11223344" {
			t.Errorf("row 4: to not preserved: %#v", d["to"])
		}
	})
}

// TestApply_CompactsStoreForwardDetails_MissingRRFailsLoudly covers
// the migration's promise that a legacy row with no `rr` (and no
// recoverable replacement) aborts the migration rather than silently
// writing a meaningless row. The legacy shape detector flags any row
// with a `role` key as legacy, even if `rr` is missing — that row
// must surface an error from mapLegacyRR.
func TestApply_CompactsStoreForwardDetails_MissingRRFailsLoudly(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, `
PRAGMA user_version = 17;
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
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  CHECK (event_kind BETWEEN 1 AND 12),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);
INSERT INTO log_channels(id, name) VALUES (1, 'LongFast');
INSERT INTO log_events(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json) VALUES
  (1, '2026-06-17T10:00:00Z', '!sf0001', 12, 0, 1, '{"role":"router","from":"!aabbccdd","stats":{}}');
`)
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	err = Apply(ctx, db, nil)
	if err == nil {
		t.Fatalf("expected Apply to fail on legacy row with missing rr, got nil")
	}
	if !strings.Contains(err.Error(), "missing rr") {
		t.Errorf("expected error to mention missing rr, got: %v", err)
	}

	// The failed migration must roll back: schema version stays at
	// 17 and the offending row must not be partially rewritten.
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 17 {
		t.Errorf("expected user_version=17 after failed migration, got %d", version)
	}
	var details sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT details_json FROM log_events WHERE id = 1`).Scan(&details); err != nil {
		t.Fatalf("read row 1: %v", err)
	}
	if !details.Valid {
		t.Fatalf("row 1: expected details_json to remain set after rollback, got NULL")
	}
	if details.String != `{"role":"router","from":"!aabbccdd","stats":{}}` {
		t.Errorf("row 1: expected details_json unchanged after rollback, got %q", details.String)
	}
}

// assertEventKindAndDetailsNull reads log_events row id, asserts the
// event_kind, and asserts the details_json column is NULL.
func assertEventKindAndDetailsNull(t *testing.T, ctx context.Context, db *sql.DB, id int64, wantKind int) {
	t.Helper()
	var eventKind int
	var details sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT event_kind, details_json FROM log_events WHERE id = ?`, id).Scan(&eventKind, &details); err != nil {
		t.Fatalf("read row %d: %v", id, err)
	}
	if eventKind != wantKind {
		t.Errorf("row %d: expected event_kind=%d, got %d", id, wantKind, eventKind)
	}
	if details.Valid {
		t.Errorf("row %d: expected details to be NULL, got %q", id, details.String)
	}
}

func assertStoreForwardDetails(t *testing.T, ctx context.Context, db *sql.DB, id int64, check func(map[string]any)) {
	t.Helper()
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT details_json FROM log_events WHERE id = ?`, id).Scan(&raw); err != nil {
		t.Fatalf("read row %d: %v", id, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("decode row %d details: %v", id, err)
	}
	check(decoded)
}

func assertDetailsEqual(t *testing.T, raw string, expected map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode details %q: %v", raw, err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("details mismatch\n  got:  %s\n  want: %s", raw, marshalCanonical(t, expected))
	}
}

func marshalCanonical(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal canonical: %v", err)
	}

	return string(b)
}
