package migrations

import (
	"context"
	"database/sql"
)

// migrateV20TelemetryPowerCurrent adds the power_current column to capture
// PowerMetrics.Ch1Current, which was previously dropped on the floor.
//
// Idempotent: the ALTER is guarded by hasColumnTx so the migration is safe
// to re-run on databases that were partially upgraded.
func migrateV20TelemetryPowerCurrent(ctx context.Context, tx *sql.Tx) error {
	hasTable, err := tableExistsTx(ctx, tx, "node_telemetry_snapshots")
	if err != nil {
		return err
	}
	if !hasTable {
		return nil
	}

	const col = "power_current"
	exists, err := hasColumnTx(ctx, tx, "node_telemetry_snapshots", col)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "ALTER TABLE node_telemetry_snapshots ADD COLUMN power_current REAL"); err != nil {
		return err
	}

	return nil
}
