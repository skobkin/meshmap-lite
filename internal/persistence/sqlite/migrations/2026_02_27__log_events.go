package migrations

import (
	"context"
	"database/sql"
)

func migrateV6LogEvents(ctx context.Context, tx *sql.Tx) error {
	return applyStatements(ctx, tx, "log_events", []string{
		`CREATE TABLE IF NOT EXISTS log_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);`,
		`CREATE TABLE IF NOT EXISTS log_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER REFERENCES log_channels(id) ON DELETE SET NULL,
  details_json TEXT,
  -- event_kind values:
  -- 1 map_report
  -- 2 node_info
  -- 3 position
  -- 4 telemetry
  -- 5 traceroute
  -- 6 neighbor_info
  -- 7 routing
  -- 8 other_portnum
  -- 9 unknown_encrypted
  CHECK (event_kind BETWEEN 1 AND 9),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_id ON log_events(id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_kind_id ON log_events(event_kind, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_channel_id ON log_events(channel_id, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_node_id ON log_events(node_id, id DESC);`,
	})
}
