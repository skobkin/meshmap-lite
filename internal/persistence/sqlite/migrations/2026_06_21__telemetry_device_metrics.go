package migrations

import (
	"context"
	"database/sql"
)

// migrateV19TelemetryDeviceMetrics adds the columns needed to persist the
// Meshtastic DeviceMetrics fields that were previously dropped on the floor:
// channel_utilization, air_util_tx, and uptime_seconds.
//
// Idempotent: each ALTER is guarded by hasColumnTx so the migration is safe
// to re-run on databases that were partially upgraded.
func migrateV19TelemetryDeviceMetrics(ctx context.Context, tx *sql.Tx) error {
	hasTable, err := tableExistsTx(ctx, tx, "node_telemetry_snapshots")
	if err != nil {
		return err
	}
	if !hasTable {
		return nil
	}

	for _, col := range []struct {
		name string
		ddl  string
	}{
		{"util_ch_util", "ALTER TABLE node_telemetry_snapshots ADD COLUMN util_ch_util REAL"},
		{"util_air_util_tx", "ALTER TABLE node_telemetry_snapshots ADD COLUMN util_air_util_tx REAL"},
		{"dev_uptime_seconds", "ALTER TABLE node_telemetry_snapshots ADD COLUMN dev_uptime_seconds INTEGER"},
	} {
		exists, err := hasColumnTx(ctx, tx, "node_telemetry_snapshots", col.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := tx.ExecContext(ctx, col.ddl); err != nil {
			return err
		}
	}

	return nil
}
