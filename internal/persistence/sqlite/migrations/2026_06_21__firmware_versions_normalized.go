package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// migrateV21FirmwareVersionsNormalized splits nodes.firmware_version into a
// normalized lookup table (firmware_versions) and a per-node FK
// (nodes.firmware_version_id). The historical weekly snapshots live in
// node_firmware_history, written only by the scheduled job.
//
// Data migration steps:
//  1. Create firmware_versions + node_firmware_history tables.
//  2. Backfill firmware_versions from existing nodes.firmware_version
//     text values (one row per distinct version).
//  3. Backfill nodes.firmware_version_id by string-joining to firmware_versions.
//  4. Drop nodes.firmware_version TEXT, add nodes.firmware_version_id INTEGER FK.
//
// node_firmware_history is intentionally NOT backfilled — the next scheduled
// weekly snapshot (Monday 00:00 UTC) populates it. This keeps the migration
// small and reduces failure modes.
//
// Idempotent: each structural step is guarded by table/column existence so
// re-running on a partially-upgraded database is safe.
func migrateV21FirmwareVersionsNormalized(ctx context.Context, tx *sql.Tx) error {
	hasNodes, err := tableExistsTx(ctx, tx, "nodes")
	if err != nil {
		return err
	}
	if !hasNodes {
		return nil
	}

	if err := applyStatements(ctx, tx, "firmware_versions", []string{
		`CREATE TABLE IF NOT EXISTS firmware_versions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  version_string TEXT NOT NULL UNIQUE,
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_firmware_versions_string ON firmware_versions(version_string);`,
		`CREATE TABLE IF NOT EXISTS node_firmware_history (
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  firmware_version_id INTEGER NOT NULL REFERENCES firmware_versions(id) ON DELETE CASCADE,
  week_start TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (node_id, week_start)
);`,
		`CREATE INDEX IF NOT EXISTS idx_nfh_week_version ON node_firmware_history(week_start, firmware_version_id);`,
		`CREATE INDEX IF NOT EXISTS idx_nfh_version ON node_firmware_history(firmware_version_id, week_start);`,
	}); err != nil {
		return err
	}

	fwTextExists, err := hasColumnTx(ctx, tx, "nodes", "firmware_version")
	if err != nil {
		return err
	}
	fwIDExists, err := hasColumnTx(ctx, tx, "nodes", "firmware_version_id")
	if err != nil {
		return err
	}

	switch {
	case fwTextExists && !fwIDExists:
		// 1. Add the FK column first (it must exist before backfill UPDATE
		//    can reference it).
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN firmware_version_id INTEGER REFERENCES firmware_versions(id) ON DELETE SET NULL;`); err != nil {
			return fmt.Errorf("add nodes.firmware_version_id: %w", err)
		}
		// 2. Backfill firmware_versions from nodes.firmware_version TEXT.
		if err := backfillFirmwareVersions(ctx, tx); err != nil {
			return fmt.Errorf("backfill firmware_versions: %w", err)
		}
		// 3. Backfill nodes.firmware_version_id by string-joining to firmware_versions.
		if err := backfillNodeFirmwareVersionID(ctx, tx); err != nil {
			return fmt.Errorf("backfill nodes.firmware_version_id: %w", err)
		}
		// 4. Drop the now-redundant TEXT column.
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes DROP COLUMN firmware_version;`); err != nil {
			return fmt.Errorf("drop nodes.firmware_version: %w", err)
		}
	case !fwTextExists && !fwIDExists:
		// Pre-existing schema on a fresh database (V21+ initialized empty) — still
		// need to add the FK column so the read-side JOIN works.
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN firmware_version_id INTEGER REFERENCES firmware_versions(id) ON DELETE SET NULL;`); err != nil {
			return fmt.Errorf("add nodes.firmware_version_id: %w", err)
		}
	}

	return nil
}

// backfillFirmwareVersions populates the firmware_versions lookup table from
// the existing nodes.firmware_version text column. One row per distinct
// version string, with first/last-seen aggregated from the nodes table.
func backfillFirmwareVersions(ctx context.Context, tx *sql.Tx) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `
INSERT INTO firmware_versions (version_string, first_seen_at, last_seen_at, created_at)
SELECT firmware_version,
       MIN(first_seen_at),
       MAX(updated_at),
       MAX(updated_at)
FROM nodes
WHERE firmware_version IS NOT NULL AND firmware_version <> ''
GROUP BY firmware_version
ON CONFLICT(version_string) DO NOTHING;`)
	if err != nil {
		return err
	}
	_ = now // reserved for potential future ON CONFLICT fallback

	return nil
}

// backfillNodeFirmwareVersionID joins nodes to firmware_versions by string
// and sets nodes.firmware_version_id for every node that previously had a
// firmware_version text value.
func backfillNodeFirmwareVersionID(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
UPDATE nodes
SET firmware_version_id = (SELECT id FROM firmware_versions WHERE version_string = nodes.firmware_version)
WHERE firmware_version IS NOT NULL AND firmware_version <> ''
  AND firmware_version_id IS NULL;`)
	if err != nil {
		return err
	}

	return nil
}
