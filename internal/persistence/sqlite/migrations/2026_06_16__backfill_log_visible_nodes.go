package migrations

import (
	"context"
	"database/sql"
)

func migrateV17BackfillLogVisibleNodes(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"nodes", "log_events", "chat_events"} {
		exists, err := tableExistsTx(ctx, tx, table)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
	}
	for _, column := range []string{"first_seen_at", "last_seen_any_event_at", "updated_at"} {
		hasColumn, err := hasColumnTx(ctx, tx, "nodes", column)
		if err != nil {
			return err
		}
		if !hasColumn {
			return nil
		}
	}

	_, err := tx.ExecContext(ctx, `
WITH evidence(node_id, observed_at) AS (
  SELECT TRIM(node_id), observed_at
  FROM log_events
  WHERE node_id IS NOT NULL AND TRIM(node_id) <> ''

  UNION ALL
  SELECT TRIM(mqtt_uploader_node_id), observed_at
  FROM log_events
  WHERE mqtt_uploader_node_id IS NOT NULL AND TRIM(mqtt_uploader_node_id) <> ''

  UNION ALL
  SELECT TRIM(node_id), observed_at
  FROM chat_events
  WHERE node_id IS NOT NULL AND TRIM(node_id) <> ''

  UNION ALL
  SELECT TRIM(mqtt_uploader_node_id), observed_at
  FROM chat_events
  WHERE mqtt_uploader_node_id IS NOT NULL AND TRIM(mqtt_uploader_node_id) <> ''

  UNION ALL
  SELECT TRIM(json_extract(details_json, '$.neighbor_node_id')), observed_at
  FROM log_events
  WHERE details_json IS NOT NULL AND json_type(details_json, '$.neighbor_node_id') = 'text'

  UNION ALL
  SELECT TRIM(json_extract(neighbor.value, '$.node_id')), log_events.observed_at
  FROM log_events, json_each(log_events.details_json, '$.neighbors') AS neighbor
  WHERE log_events.details_json IS NOT NULL
    AND json_type(log_events.details_json, '$.neighbors') = 'array'
    AND json_type(neighbor.value, '$.node_id') = 'text'

  UNION ALL
  SELECT TRIM(json_extract(details_json, '$.from')), observed_at
  FROM log_events
  WHERE details_json IS NOT NULL AND json_type(details_json, '$.from') = 'text'

  UNION ALL
  SELECT TRIM(json_extract(details_json, '$.to')), observed_at
  FROM log_events
  WHERE details_json IS NOT NULL AND json_type(details_json, '$.to') = 'text'

  UNION ALL
  SELECT TRIM(path.value), log_events.observed_at
  FROM log_events, json_each(log_events.details_json, '$.route') AS path
  WHERE log_events.details_json IS NOT NULL
    AND json_type(log_events.details_json, '$.route') = 'array'
    AND path.type = 'text'

  UNION ALL
  SELECT TRIM(path.value), log_events.observed_at
  FROM log_events, json_each(log_events.details_json, '$.route_back') AS path
  WHERE log_events.details_json IS NOT NULL
    AND json_type(log_events.details_json, '$.route_back') = 'array'
    AND path.type = 'text'

  UNION ALL
  SELECT TRIM(path.value), log_events.observed_at
  FROM log_events, json_each(log_events.details_json, '$.forward_path') AS path
  WHERE log_events.details_json IS NOT NULL
    AND json_type(log_events.details_json, '$.forward_path') = 'array'
    AND path.type = 'text'

  UNION ALL
  SELECT TRIM(path.value), log_events.observed_at
  FROM log_events, json_each(log_events.details_json, '$.return_path') AS path
  WHERE log_events.details_json IS NOT NULL
    AND json_type(log_events.details_json, '$.return_path') = 'array'
    AND path.type = 'text'

  UNION ALL
  SELECT TRIM(json_extract(details_json, '$.sender_node_id')), observed_at
  FROM log_events
  WHERE details_json IS NOT NULL AND json_type(details_json, '$.sender_node_id') = 'text'

  UNION ALL
  SELECT TRIM(json_extract(details_json, '$.destination_node_id')), observed_at
  FROM log_events
  WHERE details_json IS NOT NULL AND json_type(details_json, '$.destination_node_id') = 'text'

  UNION ALL
  SELECT TRIM(json_extract(details_json, '$.gateway_id')), observed_at
  FROM log_events
  WHERE details_json IS NOT NULL AND json_type(details_json, '$.gateway_id') = 'text'
),
valid_evidence AS (
  SELECT node_id, MIN(observed_at) AS first_seen_at, MAX(observed_at) AS last_seen_at
  FROM evidence
  WHERE node_id <> ''
  GROUP BY node_id
)
INSERT INTO nodes(node_id, first_seen_at, last_seen_any_event_at, updated_at)
SELECT node_id, first_seen_at, last_seen_at, last_seen_at
FROM valid_evidence
WHERE true
ON CONFLICT(node_id) DO UPDATE SET
  last_seen_any_event_at = CASE
    WHEN excluded.last_seen_any_event_at > nodes.last_seen_any_event_at THEN excluded.last_seen_any_event_at
    ELSE nodes.last_seen_any_event_at
  END,
  updated_at = CASE
    WHEN excluded.last_seen_any_event_at > nodes.updated_at THEN excluded.last_seen_any_event_at
    ELSE nodes.updated_at
  END;`)

	return err
}
