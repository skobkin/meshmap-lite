package migrations

import (
	"context"
	"database/sql"
)

func migrateV8LogEventRangeTestKind(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExistsTx(ctx, tx, "log_events")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	return applyStatements(ctx, tx, "log_event_range_test_kind", []string{
		`CREATE TABLE log_events_v8 (
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
  -- 10 range_test
  CHECK (event_kind BETWEEN 1 AND 10),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);`,
		`INSERT INTO log_events_v8(id, observed_at, node_id, event_kind, encrypted, channel_id, details_json)
SELECT
  id,
  observed_at,
  node_id,
  CASE
    WHEN event_kind = 8 AND (
      json_extract(details_json, '$.portnum_name') = 'RANGE_TEST_APP' OR
      json_extract(details_json, '$.portnum_value') = 66
    ) THEN 10
    ELSE event_kind
  END,
  encrypted,
  channel_id,
  CASE
    WHEN event_kind = 8 AND (
      json_extract(details_json, '$.portnum_name') = 'RANGE_TEST_APP' OR
      json_extract(details_json, '$.portnum_value') = 66
    ) THEN NULL
    ELSE details_json
  END
FROM log_events;`,
		`DROP TABLE log_events;`,
		`ALTER TABLE log_events_v8 RENAME TO log_events;`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_id ON log_events(id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_kind_id ON log_events(event_kind, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_channel_id ON log_events(channel_id, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_node_id ON log_events(node_id, id DESC);`,
		`UPDATE sqlite_sequence SET seq = COALESCE((SELECT MAX(id) FROM log_events), 0) WHERE name = 'log_events';`,
	})
}
