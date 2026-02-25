package migrations

import (
	"context"
	"database/sql"
)

func migrateV2ChatSystemCode(ctx context.Context, tx *sql.Tx) error {
	var hasTable int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='chat_events'`).Scan(&hasTable); err != nil {
		return err
	}
	if hasTable == 0 {
		return nil
	}

	hasSenderDisplay, err := hasColumnTx(ctx, tx, "chat_events", "sender_display")
	if err != nil {
		return err
	}

	if hasSenderDisplay {
		return applyStatements(ctx, tx, "chat_system_code_rebuild", []string{
			`ALTER TABLE chat_events RENAME TO chat_events_old;`,
			`CREATE TABLE chat_events (
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
			`INSERT INTO chat_events(id,event_type,channel_name,node_id,message_text,system_code,message_time,reported_at,observed_at,packet_id,created_at)
SELECT
  id,
  event_type,
  channel_name,
  CASE
    WHEN (node_id IS NULL OR node_id='') AND sender_display IS NOT NULL AND sender_display<>'' THEN sender_display
    ELSE node_id
  END AS node_id,
  CASE
    WHEN event_type='system' THEN NULL
    ELSE message_text
  END AS message_text,
  CASE
    WHEN event_type='system' THEN 'node_discovered'
    ELSE NULL
  END AS system_code,
  message_time,
  reported_at,
  observed_at,
  packet_id,
  created_at
FROM chat_events_old;`,
			`DROP TABLE chat_events_old;`,
			`CREATE INDEX IF NOT EXISTS idx_chat_channel_id ON chat_events(channel_name, id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_chat_packet_id ON chat_events(packet_id);`,
		})
	}

	hasSystemCode, err := hasColumnTx(ctx, tx, "chat_events", "system_code")
	if err != nil {
		return err
	}
	if !hasSystemCode {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE chat_events ADD COLUMN system_code TEXT`); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chat_events SET system_code='node_discovered' WHERE event_type='system' AND (system_code IS NULL OR system_code='')`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chat_events SET message_text=NULL WHERE event_type='system' AND message_text='New node discovered!'`); err != nil {
		return err
	}

	return nil
}
