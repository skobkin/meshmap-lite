package migrations

import (
	"context"
	"database/sql"
)

func migrateV13TopologyMQTTDirectSource(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExistsTx(ctx, tx, "topology_edges")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	return applyStatements(ctx, tx, "topology_mqtt_direct_source", []string{
		`CREATE TABLE topology_edges_v13 (
  source_kind INTEGER NOT NULL,
  channel_name TEXT NOT NULL DEFAULT '',
  from_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  to_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  reported_by_node_id TEXT REFERENCES nodes(node_id) ON DELETE SET NULL,
  inferred INTEGER NOT NULL DEFAULT 0,
  snr REAL,
  neighbor_last_rx_at TEXT,
  neighbor_broadcast_interval_secs INTEGER,
  first_observed_at TEXT NOT NULL,
  last_observed_at TEXT NOT NULL,
  last_reported_at TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (source_kind, channel_name, from_node_id, to_node_id),
  CHECK (source_kind BETWEEN 1 AND 6),
  CHECK (inferred IN (0, 1))
);`,
		`INSERT INTO topology_edges_v13(
  source_kind,channel_name,from_node_id,to_node_id,reported_by_node_id,inferred,snr,neighbor_last_rx_at,neighbor_broadcast_interval_secs,first_observed_at,last_observed_at,last_reported_at,updated_at
)
SELECT source_kind,channel_name,from_node_id,to_node_id,reported_by_node_id,inferred,snr,neighbor_last_rx_at,neighbor_broadcast_interval_secs,first_observed_at,last_observed_at,last_reported_at,updated_at
FROM topology_edges;`,
		`DROP TABLE topology_edges;`,
		`ALTER TABLE topology_edges_v13 RENAME TO topology_edges;`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_from_node ON topology_edges(from_node_id, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_to_node ON topology_edges(to_node_id, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_reported_by ON topology_edges(reported_by_node_id, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_source_kind ON topology_edges(source_kind, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_channel_name ON topology_edges(channel_name, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_last_observed ON topology_edges(last_observed_at DESC);`,
	})
}
