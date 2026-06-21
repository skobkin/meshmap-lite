package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateV22MapReportSeenAt adds nodes.last_map_report_at, the timestamp
// of the most recent MapReport packet seen for each node. The firmware
// version stats use this column to exclude nodes that haven't reported
// in web.stats.software.map_report_max_age (default 14d) — see
// internal/persistence/sqlite/firmware.go (FirmwareVersionSnapshot,
// RecordFirmwareHistoryWeek).
//
// Why this column is MapReport-specific:
//   - The NODEINFO_APP decoder at internal/meshtastic/portnums.go:127
//     hard-codes the firmware string to "", so only MapReport packets
//     actually populate nodes.firmware_version_id today.
//   - If NODEINFO_APP firmware decoding is ever fixed, the natural
//     generalization is a "last nodeinfo" column bumped in
//     handleNodeInfo. Until then, MapReport is the only meaningful
//     source and the column is named for that source.
//
// Why the column is nullable (no NOT NULL, no backfill):
//   - Existing nodes have no MapReport on file. Treating them as
//     "never reported → not relevant for stats" is the correct
//     semantic and degrades gracefully.
//
// Why no index on last_map_report_at:
//   - Both FirmwareVersionSnapshot and RecordFirmwareHistoryWeek do
//     full table scans of `nodes` (the table is small — hundreds of
//     rows in practice). A scan that touches every row cannot benefit
//     from an index; the planner would have to walk the index and
//     the heap. Index added here would be premature optimization.
//
// Idempotent: guarded by hasColumnTx and the table's existence (some
// test fixtures migrate partial schemas where `nodes` is absent — the
// migration must be a no-op in that case, same as V14/V17/etc.).
func migrateV22MapReportSeenAt(ctx context.Context, tx *sql.Tx) error {
	hasNodes, err := tableExistsTx(ctx, tx, "nodes")
	if err != nil {
		return err
	}
	if !hasNodes {
		return nil
	}
	has, err := hasColumnTx(ctx, tx, "nodes", "last_map_report_at")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN last_map_report_at TEXT;`); err != nil {
		return fmt.Errorf("add nodes.last_map_report_at: %w", err)
	}

	return nil
}
