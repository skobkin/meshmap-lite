package migrations

import (
	"context"
	"database/sql"
)

func migrateV10ActivityIndexes(ctx context.Context, tx *sql.Tx) error {
	statements := make([]string, 0, 2)
	hasChatEvents, err := tableExistsTx(ctx, tx, "chat_events")
	if err != nil {
		return err
	}
	if hasChatEvents {
		statements = append(statements, `CREATE INDEX IF NOT EXISTS idx_chat_events_type_observed_at ON chat_events(event_type, observed_at);`)
	}
	hasLogEvents, err := tableExistsTx(ctx, tx, "log_events")
	if err != nil {
		return err
	}
	if hasLogEvents {
		statements = append(statements, `CREATE INDEX IF NOT EXISTS idx_log_events_kind_observed_at ON log_events(event_kind, observed_at);`)
	}

	return applyStatements(ctx, tx, "activity_indexes", statements)
}
