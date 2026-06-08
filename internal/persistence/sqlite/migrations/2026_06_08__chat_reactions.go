package migrations

import (
	"context"
	"database/sql"
)

// migrateV16ChatReactions adds support for Meshtastic emoji reactions on chat
// messages. A reaction is stored in the same chat_events table with
// event_type='reaction', the payload emoji in reaction_emoji, and the
// packet_id of the message being reacted to in reply_to_packet_id. The index
// speeds up the per-message reaction lookup used when rendering a single
// message's reaction row.
func migrateV16ChatReactions(ctx context.Context, tx *sql.Tx) error {
	hasTable, err := tableExistsTx(ctx, tx, "chat_events")
	if err != nil {
		return err
	}
	if !hasTable {
		return nil
	}

	for _, column := range []struct {
		name string
		ddl  string
	}{
		{"reaction_emoji", `ALTER TABLE chat_events ADD COLUMN reaction_emoji TEXT;`},
		{"reply_to_packet_id", `ALTER TABLE chat_events ADD COLUMN reply_to_packet_id INTEGER;`},
	} {
		hasColumn, err := hasColumnTx(ctx, tx, "chat_events", column.name)
		if err != nil {
			return err
		}
		if hasColumn {
			continue
		}
		if _, err := tx.ExecContext(ctx, column.ddl); err != nil {
			return err
		}
	}

	// Partial index keyed on the target packet id; only reaction rows are
	// useful for the per-message reaction lookup, so the index stays small.
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_chat_events_reactions ON chat_events(reply_to_packet_id) WHERE event_type='reaction';`); err != nil {
		return err
	}

	return nil
}
