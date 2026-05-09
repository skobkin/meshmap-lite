package sqlite

import (
	"context"
	"database/sql"
	"time"

	"meshmap-lite/internal/domain"
)

// ActivityBuckets returns complete activity buckets with explicit zero counts.
func (s *Store) ActivityBuckets(ctx context.Context, q domain.ActivityQuery) ([]domain.ActivityBucket, error) {
	if !q.End.After(q.Start) || q.Bucket <= 0 {
		return nil, nil
	}
	bucketSeconds := int64(q.Bucket / time.Second)
	if bucketSeconds <= 0 {
		return nil, nil
	}
	count := int(q.End.Sub(q.Start) / q.Bucket)
	if count <= 0 {
		return nil, nil
	}
	out := make([]domain.ActivityBucket, count)
	indexByUnix := make(map[int64]int, count)
	startUnix := q.Start.UTC().Unix()
	for i := range out {
		start := q.Start.UTC().Add(time.Duration(i) * q.Bucket)
		out[i].BucketStart = start
		indexByUnix[start.Unix()] = i
	}

	startText := q.Start.UTC().Format(time.RFC3339Nano)
	endText := q.End.UTC().Format(time.RFC3339Nano)
	if err := s.fillChatActivityBuckets(ctx, out, indexByUnix, startUnix, bucketSeconds, startText, endText); err != nil {
		return nil, err
	}
	if err := s.fillLogActivityBuckets(ctx, out, indexByUnix, startUnix, bucketSeconds, startText, endText); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) fillChatActivityBuckets(ctx context.Context, buckets []domain.ActivityBucket, indexByUnix map[int64]int, startUnix, bucketSeconds int64, startText, endText string) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT ? + CAST((unixepoch(observed_at) - ?) / ? AS INTEGER) * ? AS bucket_start_unix,
       COUNT(*)
FROM chat_events
WHERE event_type = ? AND observed_at >= ? AND observed_at < ?
GROUP BY bucket_start_unix
`, startUnix, startUnix, bucketSeconds, bucketSeconds, string(domain.ChatEventMessage), startText, endText)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var bucketUnix int64
		var count int
		if err := rows.Scan(&bucketUnix, &count); err != nil {
			return err
		}
		if idx, ok := indexByUnix[bucketUnix]; ok {
			buckets[idx].TextMessages = count
		}
	}

	return rows.Err()
}

func (s *Store) fillLogActivityBuckets(ctx context.Context, buckets []domain.ActivityBucket, indexByUnix map[int64]int, startUnix, bucketSeconds int64, startText, endText string) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT ? + CAST((unixepoch(observed_at) - ?) / ? AS INTEGER) * ? AS bucket_start_unix,
       SUM(CASE WHEN event_kind = ? THEN 1 ELSE 0 END) AS pki,
       SUM(CASE WHEN event_kind = ? THEN 1 ELSE 0 END) AS node_info,
       SUM(CASE WHEN event_kind = ? THEN 1 ELSE 0 END) AS telemetry,
       SUM(CASE WHEN event_kind = ? THEN 1 ELSE 0 END) AS neighbor_info,
       SUM(CASE WHEN event_kind = ? THEN 1 ELSE 0 END) AS range_test,
       SUM(CASE WHEN event_kind = ? THEN 1 ELSE 0 END) AS traceroute
FROM log_events
WHERE event_kind IN (?, ?, ?, ?, ?, ?) AND observed_at >= ? AND observed_at < ?
GROUP BY bucket_start_unix
`, startUnix, startUnix, bucketSeconds, bucketSeconds,
		int(domain.LogEventKindPKIValue),
		int(domain.LogEventKindNodeInfoValue),
		int(domain.LogEventKindTelemetryValue),
		int(domain.LogEventKindNeighborInfoValue),
		int(domain.LogEventKindRangeTestValue),
		int(domain.LogEventKindTracerouteValue),
		int(domain.LogEventKindPKIValue),
		int(domain.LogEventKindNodeInfoValue),
		int(domain.LogEventKindTelemetryValue),
		int(domain.LogEventKindNeighborInfoValue),
		int(domain.LogEventKindRangeTestValue),
		int(domain.LogEventKindTracerouteValue),
		startText, endText)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var bucketUnix int64
		var counts activityLogCounts
		if err := rows.Scan(&bucketUnix, &counts.PKI, &counts.NodeInfo, &counts.Telemetry, &counts.NeighborInfo, &counts.RangeTest, &counts.Traceroute); err != nil {
			return err
		}
		if idx, ok := indexByUnix[bucketUnix]; ok {
			buckets[idx].PKI = counts.PKI
			buckets[idx].NodeInfo = counts.NodeInfo
			buckets[idx].Telemetry = counts.Telemetry
			buckets[idx].NeighborInfo = counts.NeighborInfo
			buckets[idx].RangeTest = counts.RangeTest
			buckets[idx].Traceroute = counts.Traceroute
		}
	}

	return rows.Err()
}

type activityLogCounts struct {
	PKI          int
	NodeInfo     int
	Telemetry    int
	NeighborInfo int
	RangeTest    int
	Traceroute   int
}

// Stats returns aggregate node and ingest statistics.
func (s *Store) Stats(ctx context.Context, threshold time.Duration) (domain.Stats, error) {
	var st domain.Stats
	var last sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes`).Scan(&st.KnownNodesCount); err != nil {
		return st, err
	}
	cutoff := time.Now().Add(-threshold).UTC().Format(time.RFC3339Nano)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes WHERE last_seen_mqtt_gateway_at IS NOT NULL AND last_seen_mqtt_gateway_at >= ?`, cutoff).Scan(&st.OnlineNodesCount); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(observed_at) FROM (
		SELECT observed_at FROM chat_events
		UNION ALL
		SELECT observed_at FROM log_events
	)`).Scan(&last); err == nil && last.Valid {
		if t, e := time.Parse(time.RFC3339Nano, last.String); e == nil {
			st.LastIngestAt = t
		}
	}

	return st, nil
}
