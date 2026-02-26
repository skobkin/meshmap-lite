package migrations

import (
	"context"
	"database/sql"
)

func migrateV5MapReportFlags(ctx context.Context, tx *sql.Tx) error {
	hasNodes, err := tableExistsTx(ctx, tx, "nodes")
	if err != nil {
		return err
	}
	if !hasNodes {
		return nil
	}

	hasDefaultChannel, err := hasColumnTx(ctx, tx, "nodes", "has_default_channel")
	if err != nil {
		return err
	}
	if !hasDefaultChannel {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN has_default_channel INTEGER`); err != nil {
			return err
		}
	}

	hasOptedReportLocation, err := hasColumnTx(ctx, tx, "nodes", "has_opted_report_location")
	if err != nil {
		return err
	}
	if !hasOptedReportLocation {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE nodes ADD COLUMN has_opted_report_location INTEGER`); err != nil {
			return err
		}
	}

	return nil
}
