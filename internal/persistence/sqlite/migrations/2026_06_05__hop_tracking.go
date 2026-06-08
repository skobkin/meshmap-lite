package migrations

import (
	"context"
	"database/sql"
)

func migrateV15HopTracking(ctx context.Context, tx *sql.Tx) error {
	// Add hop tracking columns to chat_events.
	for _, column := range []struct {
		table string
		name  string
		sql   string
	}{
		{"chat_events", "hop_start", `ALTER TABLE chat_events ADD COLUMN hop_start INTEGER;`},
		{"chat_events", "hop_limit", `ALTER TABLE chat_events ADD COLUMN hop_limit INTEGER;`},
		{"log_events", "hop_start", `ALTER TABLE log_events ADD COLUMN hop_start INTEGER;`},
		{"log_events", "hop_limit", `ALTER TABLE log_events ADD COLUMN hop_limit INTEGER;`},
	} {
		hasTable, err := tableExistsTx(ctx, tx, column.table)
		if err != nil {
			return err
		}
		if !hasTable {
			continue
		}
		hasColumn, err := hasColumnTx(ctx, tx, column.table, column.name)
		if err != nil {
			return err
		}
		if hasColumn {
			continue
		}
		if _, err := tx.ExecContext(ctx, column.sql); err != nil {
			return err
		}
	}

	return nil
}
