package migrations

import (
	"context"
	"database/sql"
)

func migrateV7TopologyEdges(ctx context.Context, tx *sql.Tx) error {
	return applyStatements(ctx, tx, "topology_edges", []string{
		`CREATE TABLE IF NOT EXISTS topology_edges (
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
  -- source_kind values:
  -- 1 neighbor_info
  -- 2 routing_forward
  -- 3 routing_return
  -- 4 traceroute_forward
  -- 5 traceroute_return
  CHECK (source_kind BETWEEN 1 AND 5),
  CHECK (inferred IN (0, 1))
);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_from_node ON topology_edges(from_node_id, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_to_node ON topology_edges(to_node_id, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_reported_by ON topology_edges(reported_by_node_id, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_source_kind ON topology_edges(source_kind, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_channel_name ON topology_edges(channel_name, last_observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_topology_edges_last_observed ON topology_edges(last_observed_at DESC);`,
	})
}
