package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// UpsertTopologyEdges inserts or updates the latest observed topology edge snapshots.
func (s *Store) UpsertTopologyEdges(ctx context.Context, edges []domain.TopologyEdge) error {
	if len(edges) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO topology_edges(
 source_kind,channel_name,from_node_id,to_node_id,reported_by_node_id,inferred,snr,neighbor_last_rx_at,neighbor_broadcast_interval_secs,first_observed_at,last_observed_at,last_reported_at,updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(source_kind,channel_name,from_node_id,to_node_id) DO UPDATE SET
 reported_by_node_id=COALESCE(NULLIF(excluded.reported_by_node_id,''),topology_edges.reported_by_node_id),
 inferred=excluded.inferred,
 snr=COALESCE(excluded.snr,topology_edges.snr),
 neighbor_last_rx_at=COALESCE(excluded.neighbor_last_rx_at,topology_edges.neighbor_last_rx_at),
 neighbor_broadcast_interval_secs=COALESCE(excluded.neighbor_broadcast_interval_secs,topology_edges.neighbor_broadcast_interval_secs),
 last_observed_at=excluded.last_observed_at,
 last_reported_at=COALESCE(excluded.last_reported_at,topology_edges.last_reported_at),
 updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, edge := range edges {
		sourceValue, ok := domain.TopologySourceKindValue(edge.SourceKind)
		if !ok {
			return fmt.Errorf("unknown topology source kind: %q", edge.SourceKind)
		}
		if edge.SourceKind == domain.TopologySourceMQTTDirect {
			updated, err := updateExistingDirectTopologyEdge(ctx, tx, edge)
			if err != nil {
				return err
			}
			if updated {
				continue
			}
		}
		if _, err := stmt.ExecContext(ctx,
			sourceValue,
			edge.ChannelName,
			edge.FromNodeID,
			edge.ToNodeID,
			nullIfEmpty(edge.ReportedByNodeID),
			boolAsInt(edge.Inferred),
			ptrFloat(edge.SNR),
			ptrTime(edge.NeighborLastRXAt),
			ptrUint32(edge.NeighborBroadcastIntervalSec),
			edge.FirstObservedAt.UTC().Format(time.RFC3339Nano),
			edge.LastObservedAt.UTC().Format(time.RFC3339Nano),
			ptrTime(edge.LastReportedAt),
			edge.UpdatedAt.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func updateExistingDirectTopologyEdge(ctx context.Context, tx *sql.Tx, edge domain.TopologyEdge) (bool, error) {
	result, err := tx.ExecContext(ctx, `
	UPDATE topology_edges
	SET last_observed_at=?,
	    last_reported_at=COALESCE(?,last_reported_at),
	    updated_at=?,
	    snr=CASE WHEN source_kind=? THEN COALESCE(?,snr) ELSE snr END
	WHERE rowid = (
  SELECT rowid
  FROM topology_edges
  WHERE channel_name=?
    AND ((from_node_id=? AND to_node_id=?) OR (from_node_id=? AND to_node_id=?))
  ORDER BY
    CASE source_kind
      WHEN ? THEN 0
      WHEN ? THEN 1
      WHEN ? THEN 2
      WHEN ? THEN 3
      WHEN ? THEN 4
      WHEN ? THEN 5
      ELSE 6
    END,
    last_observed_at DESC
  LIMIT 1
)`,
		edge.LastObservedAt.UTC().Format(time.RFC3339Nano),
		ptrTime(edge.LastReportedAt),
		edge.UpdatedAt.UTC().Format(time.RFC3339Nano),
		domain.TopologySourceMQTTDirectValue,
		ptrFloat(edge.SNR),
		edge.ChannelName,
		edge.FromNodeID,
		edge.ToNodeID,
		edge.ToNodeID,
		edge.FromNodeID,
		domain.TopologySourceNeighborInfoValue,
		domain.TopologySourceMQTTDirectValue,
		domain.TopologySourceRoutingForwardValue,
		domain.TopologySourceRoutingReturnValue,
		domain.TopologySourceTracerouteForwardValue,
		domain.TopologySourceTracerouteReturnValue,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// ListTopologyEdges returns current topology-edge snapshots ordered by recency.
func (s *Store) ListTopologyEdges(ctx context.Context, q repo.TopologyEdgeQuery) ([]domain.TopologyEdge, error) {
	var (
		b strings.Builder
		a []interface{}
		w []string
	)
	b.WriteString(`
SELECT source_kind,channel_name,from_node_id,to_node_id,reported_by_node_id,inferred,snr,neighbor_last_rx_at,neighbor_broadcast_interval_secs,first_observed_at,last_observed_at,last_reported_at,updated_at
FROM topology_edges`)
	if nodeID := strings.TrimSpace(q.NodeID); nodeID != "" {
		w = append(w, `(from_node_id=? OR to_node_id=?)`)
		a = append(a, nodeID, nodeID)
	}
	if ch := strings.TrimSpace(q.Channel); ch != "" {
		w = append(w, `LOWER(channel_name)=LOWER(?)`)
		a = append(a, ch)
	}
	if len(q.SourceKinds) > 0 {
		var in strings.Builder
		count := 0
		in.WriteString(`source_kind IN (`)
		for _, kind := range q.SourceKinds {
			value, ok := domain.TopologySourceKindValue(kind)
			if !ok {
				continue
			}
			if count > 0 {
				in.WriteString(`,`)
			}
			in.WriteString(`?`)
			a = append(a, value)
			count++
		}
		in.WriteString(`)`)
		if count > 0 {
			w = append(w, in.String())
		}
	}
	if !q.UpdatedSince.IsZero() {
		w = append(w, `updated_at>=?`)
		a = append(a, q.UpdatedSince.UTC().Format(time.RFC3339Nano))
	}
	if len(w) > 0 {
		b.WriteString(` WHERE `)
		b.WriteString(strings.Join(w, ` AND `))
	}
	b.WriteString(` ORDER BY last_observed_at DESC, source_kind, from_node_id, to_node_id`)

	rows, err := s.db.QueryContext(ctx, b.String(), a...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]domain.TopologyEdge, 0)
	for rows.Next() {
		item, err := scanTopologyEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	return out, rows.Err()
}
