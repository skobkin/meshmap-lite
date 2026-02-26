package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

const targetSchemaVersion = 4

type migrationStep struct {
	version int
	name    string
	apply   func(context.Context, *sql.Tx) error
}

var schemaMigrations = []migrationStep{
	{version: 1, name: "bootstrap_core_schema", apply: migrateV1BootstrapCoreSchema},
	{version: 2, name: "chat_system_code", apply: migrateV2ChatSystemCode},
	{version: 3, name: "telemetry_iaq", apply: migrateV3TelemetryIAQ},
	{version: 4, name: "normalize_default_channel_names", apply: migrateV4NormalizeDefaultChannelNames},
}

// Apply upgrades the SQLite schema to the latest supported version.
func Apply(ctx context.Context, db *sql.DB) error {
	version, err := readSchemaVersion(ctx, db)
	if err != nil {
		return err
	}

	if version == 0 {
		hasNodes, err := tableExists(ctx, db, "nodes")
		if err != nil {
			return err
		}
		if hasNodes {
			if err := setSchemaVersion(ctx, db, 1); err != nil {
				return err
			}
			version = 1
		}
	}

	if version >= targetSchemaVersion {
		return nil
	}

	for _, migration := range schemaMigrations {
		if version >= migration.version {
			continue
		}
		if err := applyMigration(ctx, db, migration); err != nil {
			return fmt.Errorf("apply migration %s (%d): %w", migration.name, migration.version, err)
		}
		version = migration.version
	}

	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration migrationStep) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := migration.apply(ctx, tx); err != nil {
		return err
	}
	if err := setSchemaVersionTx(ctx, tx, migration.version); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}

	return nil
}

func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version;`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}

	return version, nil
}

func setSchemaVersion(ctx context.Context, db *sql.DB, version int) error {
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d;`, version)); err != nil {
		return fmt.Errorf("set schema version %d: %w", version, err)
	}

	return nil
}

func setSchemaVersionTx(ctx context.Context, tx *sql.Tx, version int) error {
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d;`, version)); err != nil {
		return fmt.Errorf("set schema version %d: %w", version, err)
	}

	return nil
}

func tableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
		return false, fmt.Errorf("table exists %s: %w", table, err)
	}

	return count > 0, nil
}

func tableExistsTx(ctx context.Context, tx *sql.Tx, table string) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
		return false, fmt.Errorf("table exists %s: %w", table, err)
	}

	return count > 0, nil
}

func hasColumnTx(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, fmt.Errorf("table_info %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan table_info %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("read table_info %s: %w", table, err)
	}

	return false, nil
}

func applyStatements(ctx context.Context, tx *sql.Tx, migrationName string, statements []string) error {
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %s statement: %w", migrationName, err)
		}
	}

	return nil
}
