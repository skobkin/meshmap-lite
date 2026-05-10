package migrations

import (
	"context"
	"database/sql"
)

func migrateV14DiscardZeroPositions(ctx context.Context, tx *sql.Tx) error {
	hasPositions, err := tableExistsTx(ctx, tx, "node_positions")
	if err != nil {
		return err
	}
	if !hasPositions {
		return nil
	}

	statements := []string{
		`CREATE TEMP TABLE zero_position_nodes(node_id TEXT PRIMARY KEY);`,
		`INSERT INTO zero_position_nodes(node_id)
SELECT node_id FROM node_positions WHERE latitude = 0 AND longitude = 0;`,
		`DELETE FROM node_positions WHERE latitude = 0 AND longitude = 0;`,
	}

	hasNodes, err := tableExistsTx(ctx, tx, "nodes")
	if err != nil {
		return err
	}
	if hasNodes {
		hasLastSeenPositionAt, err := hasColumnTx(ctx, tx, "nodes", "last_seen_position_at")
		if err != nil {
			return err
		}
		if hasLastSeenPositionAt {
			statements = append(statements,
				`UPDATE nodes
SET last_seen_position_at = NULL
WHERE node_id IN (SELECT node_id FROM zero_position_nodes)
  AND NOT EXISTS (
    SELECT 1 FROM node_positions WHERE node_positions.node_id = nodes.node_id
  );`,
			)
		}
	}

	statements = append(statements,
		`DROP TABLE zero_position_nodes;`,
	)

	return applyStatements(ctx, tx, "discard_zero_positions", statements)
}
