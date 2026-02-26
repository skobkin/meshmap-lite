package migrations

import (
	"context"
	"database/sql"
)

func migrateV3TelemetryIAQ(ctx context.Context, tx *sql.Tx) error {
	hasTable, err := tableExistsTx(ctx, tx, "node_telemetry_snapshots")
	if err != nil {
		return err
	}
	if !hasTable {
		return nil
	}

	hasIAQ, err := hasColumnTx(ctx, tx, "node_telemetry_snapshots", "air_iaq")
	if err != nil {
		return err
	}
	if hasIAQ {
		return nil
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE node_telemetry_snapshots ADD COLUMN air_iaq REAL`)
	if err != nil {
		return err
	}

	return nil
}
