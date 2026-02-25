package migrations

import (
	"context"
	"database/sql"
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

	if err := Apply(ctx, db); err != nil {
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
