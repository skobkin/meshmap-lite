package migrations

import (
	"context"
	"database/sql"
)

func migrateV4NormalizeDefaultChannelNames(ctx context.Context, tx *sql.Tx) error {
	defaultNames := map[string]string{
		"longfast":   "LongFast",
		"longslow":   "LongSlow",
		"mediumslow": "MediumSlow",
		"shortfast":  "ShortFast",
	}
	targets := []struct {
		table  string
		column string
		stmt   string
	}{
		{table: "chat_events", column: "channel_name", stmt: "UPDATE chat_events SET channel_name = ? WHERE LOWER(channel_name) = ?"},
		{table: "node_positions", column: "source_channel", stmt: "UPDATE node_positions SET source_channel = ? WHERE LOWER(source_channel) = ?"},
		{table: "node_telemetry_snapshots", column: "source_channel", stmt: "UPDATE node_telemetry_snapshots SET source_channel = ? WHERE LOWER(source_channel) = ?"},
	}

	for _, target := range targets {
		hasTable, err := tableExistsTx(ctx, tx, target.table)
		if err != nil {
			return err
		}
		if !hasTable {
			continue
		}

		hasColumn, err := hasColumnTx(ctx, tx, target.table, target.column)
		if err != nil {
			return err
		}
		if !hasColumn {
			continue
		}

		for lower, canonical := range defaultNames {
			if _, err := tx.ExecContext(ctx, target.stmt, canonical, lower); err != nil {
				return err
			}
		}
	}

	return nil
}
