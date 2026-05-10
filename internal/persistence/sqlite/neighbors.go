package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

type collapsedNeighbor struct {
	row repo.NodeNeighbor
	key neighborEvidenceKey
}

type neighborEvidenceKey struct {
	rank     int
	snrKnown bool
	snr      float64
	observed time.Time
}

func (s *Store) getNodeNeighbors(ctx context.Context, nodeID string) ([]repo.NodeNeighbor, error) {
	edges, err := s.ListTopologyEdges(ctx, repo.TopologyEdgeQuery{
		NodeID: nodeID,
		SourceKinds: []domain.TopologySourceKind{
			domain.TopologySourceNeighborInfo,
			domain.TopologySourceMQTTDirect,
			domain.TopologySourceRoutingForward,
			domain.TopologySourceRoutingReturn,
		},
	})
	if err != nil {
		return nil, err
	}
	peers := make(map[string]struct{})
	for _, edge := range edges {
		peerID := edge.FromNodeID
		if peerID == nodeID {
			peerID = edge.ToNodeID
		}
		if peerID == "" || peerID == nodeID {
			continue
		}
		peers[peerID] = struct{}{}
	}
	peerMeta, err := s.getNeighborNodeMeta(ctx, peers)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string]collapsedNeighbor)
	for _, edge := range edges {
		peerID := edge.FromNodeID
		if peerID == nodeID {
			peerID = edge.ToNodeID
		}
		if peerID == "" || peerID == nodeID {
			continue
		}
		meta := peerMeta[peerID]

		candidate := repo.NodeNeighbor{
			NodeID:                       peerID,
			DisplayName:                  displayName(meta.LongName, meta.ShortName, peerID),
			LongName:                     meta.LongName,
			ShortName:                    meta.ShortName,
			HasPosition:                  meta.HasPosition,
			EvidenceKind:                 "inferred",
			ChannelName:                  edge.ChannelName,
			ReportedByNodeID:             edge.ReportedByNodeID,
			NeighborLastRXAt:             edge.NeighborLastRXAt,
			NeighborBroadcastIntervalSec: edge.NeighborBroadcastIntervalSec,
			LastObservedAt:               edge.LastObservedAt,
			LastReportedAt:               edge.LastReportedAt,
		}
		key := neighborEvidenceSortKey(edge)
		switch edge.SourceKind {
		case domain.TopologySourceNeighborInfo:
			candidate.EvidenceKind = "neighbor_info"
			candidate.SNR = edge.SNR
		case domain.TopologySourceMQTTDirect:
			candidate.EvidenceKind = "mqtt_direct"
			candidate.SNR = edge.SNR
		}

		existing, ok := grouped[peerID]
		if !ok || preferNeighborCandidate(key, existing.key) {
			grouped[peerID] = collapsedNeighbor{
				row: mergeNeighborMetadata(candidate, existing.row),
				key: key,
			}

			continue
		}
		existing.row = mergeNeighborMetadata(existing.row, candidate)
		grouped[peerID] = existing
	}

	out := make([]repo.NodeNeighbor, 0, len(grouped))
	for _, item := range grouped {
		out = append(out, item.row)
	}
	sort.Slice(out, func(i, j int) bool {
		left := neighborDisplaySortKey(out[i])
		right := neighborDisplaySortKey(out[j])
		if left.rank != right.rank {
			return left.rank < right.rank
		}
		if left.snrKnown != right.snrKnown {
			return left.snrKnown
		}
		if left.snr != right.snr {
			return left.snr > right.snr
		}
		if !left.observed.Equal(right.observed) {
			return left.observed.After(right.observed)
		}

		return out[i].DisplayName < out[j].DisplayName
	})

	return out, nil
}

type neighborNodeMeta struct {
	LongName    string
	ShortName   string
	HasPosition bool
}

func (s *Store) getNeighborNodeMeta(ctx context.Context, peers map[string]struct{}) (map[string]neighborNodeMeta, error) {
	if len(peers) == 0 {
		return map[string]neighborNodeMeta{}, nil
	}
	args := make([]interface{}, 0, len(peers))
	placeholders := make([]string, 0, len(peers))
	for peerID := range peers {
		args = append(args, peerID)
		placeholders = append(placeholders, `?`)
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT n.node_id,n.long_name,n.short_name,CASE WHEN p.node_id IS NULL THEN 0 ELSE 1 END
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id
WHERE n.node_id IN (%s)`, strings.Join(placeholders, `,`)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]neighborNodeMeta, len(peers))
	for rows.Next() {
		var (
			nodeID, longName, shortName sql.NullString
			hasPosition                 int
		)
		if err := rows.Scan(&nodeID, &longName, &shortName, &hasPosition); err != nil {
			return nil, err
		}
		if !nodeID.Valid || nodeID.String == "" {
			continue
		}
		out[nodeID.String] = neighborNodeMeta{
			LongName:    longName.String,
			ShortName:   shortName.String,
			HasPosition: hasPosition == 1,
		}
	}

	return out, rows.Err()
}

func neighborEvidenceSortKey(edge domain.TopologyEdge) neighborEvidenceKey {
	switch edge.SourceKind {
	case domain.TopologySourceNeighborInfo:
		if edge.SNR != nil {
			return neighborEvidenceKey{rank: 0, snrKnown: true, snr: *edge.SNR, observed: edge.LastObservedAt}
		}

		return neighborEvidenceKey{rank: 1, observed: edge.LastObservedAt}
	case domain.TopologySourceMQTTDirect:
		if edge.SNR != nil {
			return neighborEvidenceKey{rank: 2, snrKnown: true, snr: *edge.SNR, observed: edge.LastObservedAt}
		}

		return neighborEvidenceKey{rank: 2, observed: edge.LastObservedAt}
	default:
		return neighborEvidenceKey{rank: 3, observed: edge.LastObservedAt}
	}
}

func preferNeighborCandidate(candidate, current neighborEvidenceKey) bool {
	if candidate.rank != current.rank {
		return candidate.rank < current.rank
	}
	if candidate.snrKnown != current.snrKnown {
		return candidate.snrKnown
	}
	if candidate.snr != current.snr {
		return candidate.snr > current.snr
	}
	if !candidate.observed.Equal(current.observed) {
		return candidate.observed.After(current.observed)
	}

	return false
}

func neighborDisplaySortKey(neighbor repo.NodeNeighbor) neighborEvidenceKey {
	switch neighbor.EvidenceKind {
	case "neighbor_info":
		if neighbor.SNR != nil {
			return neighborEvidenceKey{rank: 0, snrKnown: true, snr: *neighbor.SNR, observed: neighbor.LastObservedAt}
		}

		return neighborEvidenceKey{rank: 1, observed: neighbor.LastObservedAt}
	case "mqtt_direct":
		if neighbor.SNR != nil {
			return neighborEvidenceKey{rank: 2, snrKnown: true, snr: *neighbor.SNR, observed: neighbor.LastObservedAt}
		}

		return neighborEvidenceKey{rank: 2, observed: neighbor.LastObservedAt}
	default:
		return neighborEvidenceKey{rank: 3, observed: neighbor.LastObservedAt}
	}
}

func mergeNeighborMetadata(preferred, secondary repo.NodeNeighbor) repo.NodeNeighbor {
	if preferred.NodeID == "" {
		return secondary
	}
	if secondary.NodeID == "" {
		return preferred
	}
	if preferred.DisplayName == "" {
		preferred.DisplayName = secondary.DisplayName
	}
	if preferred.LongName == "" {
		preferred.LongName = secondary.LongName
	}
	if preferred.ShortName == "" {
		preferred.ShortName = secondary.ShortName
	}
	preferred.HasPosition = preferred.HasPosition || secondary.HasPosition
	if preferred.LastObservedAt.Before(secondary.LastObservedAt) {
		preferred.LastObservedAt = secondary.LastObservedAt
	}
	if preferred.LastReportedAt == nil || (secondary.LastReportedAt != nil && secondary.LastReportedAt.After(*preferred.LastReportedAt)) {
		preferred.LastReportedAt = secondary.LastReportedAt
	}
	if preferred.ChannelName == "" {
		preferred.ChannelName = secondary.ChannelName
	}
	if preferred.ReportedByNodeID == "" {
		preferred.ReportedByNodeID = secondary.ReportedByNodeID
	}
	if preferred.NeighborLastRXAt == nil {
		preferred.NeighborLastRXAt = secondary.NeighborLastRXAt
	}
	if preferred.NeighborBroadcastIntervalSec == nil {
		preferred.NeighborBroadcastIntervalSec = secondary.NeighborBroadcastIntervalSec
	}

	if preferred.SNR == nil {
		preferred.SNR = secondary.SNR
	}

	return preferred
}
