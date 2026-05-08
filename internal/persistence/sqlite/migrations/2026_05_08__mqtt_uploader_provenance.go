package migrations

import (
	"context"
	"database/sql"
)

func migrateV11MQTTUploaderProvenance(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct {
		table string
		name  string
		sql   string
	}{
		{"nodes", "last_mqtt_uploader_node_id", `ALTER TABLE nodes ADD COLUMN last_mqtt_uploader_node_id TEXT;`},
		{"nodes", "last_mqtt_uploader_at", `ALTER TABLE nodes ADD COLUMN last_mqtt_uploader_at TEXT;`},
		{"chat_events", "mqtt_uploader_node_id", `ALTER TABLE chat_events ADD COLUMN mqtt_uploader_node_id TEXT;`},
		{"log_events", "mqtt_uploader_node_id", `ALTER TABLE log_events ADD COLUMN mqtt_uploader_node_id TEXT;`},
		{"node_positions", "mqtt_uploader_node_id", `ALTER TABLE node_positions ADD COLUMN mqtt_uploader_node_id TEXT;`},
		{"node_telemetry_snapshots", "mqtt_uploader_node_id", `ALTER TABLE node_telemetry_snapshots ADD COLUMN mqtt_uploader_node_id TEXT;`},
	} {
		hasTable, err := tableExistsTx(ctx, tx, column.table)
		if err != nil {
			return err
		}
		if !hasTable {
			continue
		}
		hasColumn, err := hasColumnTx(ctx, tx, column.table, column.name)
		if err != nil {
			return err
		}
		if hasColumn {
			continue
		}
		if _, err := tx.ExecContext(ctx, column.sql); err != nil {
			return err
		}
	}

	for _, index := range []struct {
		table string
		sql   string
	}{
		{"nodes", `CREATE INDEX IF NOT EXISTS idx_nodes_last_mqtt_uploader_node_id ON nodes(last_mqtt_uploader_node_id);`},
		{"chat_events", `CREATE INDEX IF NOT EXISTS idx_chat_events_mqtt_uploader_node_id ON chat_events(mqtt_uploader_node_id);`},
		{"log_events", `CREATE INDEX IF NOT EXISTS idx_log_events_mqtt_uploader_node_id ON log_events(mqtt_uploader_node_id);`},
	} {
		hasTable, err := tableExistsTx(ctx, tx, index.table)
		if err != nil {
			return err
		}
		if !hasTable {
			continue
		}
		if _, err := tx.ExecContext(ctx, index.sql); err != nil {
			return err
		}
	}

	return nil
}
