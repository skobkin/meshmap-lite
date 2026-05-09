package migrations

import (
	"context"
	"database/sql"
)

func migrateV12NodeNameHistory(ctx context.Context, tx *sql.Tx) error {
	return applyStatements(ctx, tx, "node_name_history", []string{
		`CREATE TABLE IF NOT EXISTS node_name_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  previous_long_name TEXT,
  previous_short_name TEXT,
  new_long_name TEXT,
  new_short_name TEXT,
  changed_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_node_name_history_node_changed ON node_name_history(node_id, changed_at DESC, id DESC);`,
	})
}
