package sqlite

import (
	"context"
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
