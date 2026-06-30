package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// migrateV23HardwareModelsNormalized splits nodes.board_model into a normalized
// lookup table (hardware_models) and a per-node FK (nodes.hardware_model_id),
// mirroring the V21 firmware normalization. The historical weekly snapshots
// live in node_hardware_history, written only by the scheduled job.
//
// Data migration steps:
//  1. Create hardware_models + node_hardware_history tables.
//  2. Add nodes.hardware_model_id INTEGER FK (must exist before backfill).
//  3. Backfill hardware_models from existing nodes.board_model text values
//     (one row per distinct model).
//  4. Backfill nodes.hardware_model_id by string-joining to hardware_models.
//  5. Drop nodes.board_model TEXT.
//
// node_hardware_history is intentionally NOT backfilled — the next scheduled
// weekly snapshot (Monday 00:00 UTC) populates it. This keeps the migration
// small and reduces failure modes (same approach as V21).
//
// Idempotent: each structural step is guarded by table/column existence so
// re-running on a partially-upgraded database is safe.
func migrateV23HardwareModelsNormalized(ctx context.Context, tx *sql.Tx) error {
	hasNodes, err := tableExistsTx(ctx, tx, "nodes")
	if err != nil {
		return err
	}
	if !hasNodes {
		return nil
	}

	if err := applyStatements(ctx, tx, "hardware_models", []string{
		`CREATE TABLE IF NOT EXISTS hardware_models (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  model_string TEXT NOT NULL UNIQUE,
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_hardware_models_string ON hardware_models(model_string);`,
		`CREATE TABLE IF NOT EXISTS node_hardware_history (
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  hardware_model_id INTEGER NOT NULL REFERENCES hardware_models(id) ON DELETE CASCADE,
  week_start TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (node_id, week_start)
);`,
		`CREATE INDEX IF NOT EXISTS idx_nhh_week_model ON node_hardware_history(week_start, hardware_model_id);`,
		`CREATE INDEX IF NOT EXISTS idx_nhh_model ON node_hardware_history(hardware_model_id, week_start);`,
	}); err != nil {
		return err
	}

	boardTextExists, err := hasColumnTx(ctx, tx, "nodes", "board_model")
	if err != nil {
		return err
	}
	hwIDExists, err := hasColumnTx(ctx, tx, "nodes", "hardware_model_id")
	if err != nil {
		return err
	}

	switch {
	case boardTextExists && !hwIDExists:
		// 1. Add the FK column first (it must exist before the backfill UPDATE
		//    can reference it).
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN hardware_model_id INTEGER REFERENCES hardware_models(id) ON DELETE SET NULL;`); err != nil {
			return fmt.Errorf("add nodes.hardware_model_id: %w", err)
		}
		// 2. Backfill hardware_models from nodes.board_model TEXT.
		if err := backfillHardwareModels(ctx, tx); err != nil {
			return fmt.Errorf("backfill hardware_models: %w", err)
		}
		// 3. Backfill nodes.hardware_model_id by string-joining to hardware_models.
		if err := backfillNodeHardwareModelID(ctx, tx); err != nil {
			return fmt.Errorf("backfill nodes.hardware_model_id: %w", err)
		}
		// 4. Drop the now-redundant TEXT column.
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes DROP COLUMN board_model;`); err != nil {
			return fmt.Errorf("drop nodes.board_model: %w", err)
		}
	case !boardTextExists && !hwIDExists:
		// Pre-existing schema on a fresh database (V23+ initialized empty) — still
		// need to add the FK column so the read-side JOIN works.
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN hardware_model_id INTEGER REFERENCES hardware_models(id) ON DELETE SET NULL;`); err != nil {
			return fmt.Errorf("add nodes.hardware_model_id: %w", err)
		}
	}

	return nil
}

// backfillHardwareModels populates the hardware_models lookup table from the
// existing nodes.board_model text column. One row per distinct model string,
// with first/last-seen aggregated from the nodes table.
func backfillHardwareModels(ctx context.Context, tx *sql.Tx) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `
INSERT INTO hardware_models (model_string, first_seen_at, last_seen_at, created_at)
SELECT board_model,
       MIN(first_seen_at),
       MAX(updated_at),
       MAX(updated_at)
FROM nodes
WHERE board_model IS NOT NULL AND board_model <> ''
GROUP BY board_model
ON CONFLICT(model_string) DO NOTHING;`)
	if err != nil {
		return err
	}
	_ = now // reserved for potential future ON CONFLICT fallback

	return nil
}

// backfillNodeHardwareModelID joins nodes to hardware_models by string and sets
// nodes.hardware_model_id for every node that previously had a board_model text
// value.
func backfillNodeHardwareModelID(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
UPDATE nodes
SET hardware_model_id = (SELECT id FROM hardware_models WHERE model_string = nodes.board_model)
WHERE board_model IS NOT NULL AND board_model <> ''
  AND hardware_model_id IS NULL;`)
	if err != nil {
		return err
	}

	return nil
}
