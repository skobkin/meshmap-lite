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
VALUES ('!00000001',0,0,'channel_position','longfast','2026-02-26T00:00:00Z','2026-02-26T00:00:00Z');
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
