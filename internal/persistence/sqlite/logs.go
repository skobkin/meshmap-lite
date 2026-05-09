package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"meshmap-lite/internal/domain"
)

// InsertLogEvent appends a compact mesh activity event and returns its row ID.
func (s *Store) InsertLogEvent(ctx context.Context, e domain.LogEvent) (int64, error) {
	var channelID interface{}
	if ch := strings.TrimSpace(e.Channel); ch != "" {
		id, err := s.ensureLogChannel(ctx, ch)
		if err != nil {
			return 0, err
		}
		channelID = id
	}

	var detailsJSON interface{}
	if len(e.Details) > 0 {
		body, err := json.Marshal(e.Details)
		if err != nil {
			return 0, fmt.Errorf("marshal log details: %w", err)
		}
		detailsJSON = string(body)
	}

	res, err := s.db.ExecContext(ctx, `
INSERT INTO log_events(observed_at,node_id,event_kind,encrypted,channel_id,mqtt_uploader_node_id,details_json)
VALUES(?,?,?,?,?,?,?)
`, e.ObservedAt.UTC().Format(time.RFC3339Nano), nullIfEmpty(e.NodeID), int(e.EventKind), boolAsInt(e.Encrypted), channelID, nullIfEmpty(e.MQTTUploaderNodeID), detailsJSON)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if s.shouldPruneLogEvents(id) {
		if err := s.pruneLogEvents(ctx); err != nil {
			return id, err
		}
	}

	return id, nil
}

func (s *Store) shouldPruneLogEvents(insertedID int64) bool {
	if s.logMaxRows <= 0 {
		return false
	}

	next := s.nextLogPruneAtID.Load()
	if next == 0 {
		next = int64(s.logMaxRows+s.logPruneBatchRows) + 1
		if !s.nextLogPruneAtID.CompareAndSwap(0, next) {
			next = s.nextLogPruneAtID.Load()
		}
	}
	if insertedID < next {
		return false
	}

	interval := int64(maxInt(1, s.logPruneBatchRows))
	s.nextLogPruneAtID.Store(insertedID + interval)

	return true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func (s *Store) cachedLogChannelID(name string) (int64, bool) {
	s.logChannelMu.RLock()
	id, ok := s.logChannelIDs[name]
	s.logChannelMu.RUnlock()

	return id, ok
}

func (s *Store) storeLogChannelID(name string, id int64) {
	s.logChannelMu.Lock()
	s.logChannelIDs[name] = id
	s.logChannelMu.Unlock()
}

func (s *Store) ensureLogChannel(ctx context.Context, name string) (int64, error) {
	if id, ok := s.cachedLogChannelID(name); ok {
		return id, nil
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO log_channels(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM log_channels WHERE name=?`, name).Scan(&id); err != nil {
		return id, err
	}
	s.storeLogChannelID(name, id)

	return id, nil
}

// ListLogEvents returns paginated Log-tab items with node display fallback.
func (s *Store) ListLogEvents(ctx context.Context, q domain.LogEventQuery) ([]domain.LogEventView, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var (
		b strings.Builder
		a []interface{}
		w []string
	)
	b.WriteString(`
SELECT e.id,e.observed_at,e.node_id,e.event_kind,e.encrypted,c.name,
       n.long_name,n.short_name,e.mqtt_uploader_node_id,mu.long_name,mu.short_name,e.details_json
FROM log_events e
LEFT JOIN log_channels c ON c.id=e.channel_id
LEFT JOIN nodes n ON n.node_id=e.node_id
LEFT JOIN nodes mu ON mu.node_id=e.mqtt_uploader_node_id`)
	if q.BeforeID > 0 {
		w = append(w, `e.id < ?`)
		a = append(a, q.BeforeID)
	}
	if ch := strings.TrimSpace(q.Channel); ch != "" {
		w = append(w, `LOWER(c.name)=LOWER(?)`)
		a = append(a, ch)
	}
	if nodeID := strings.TrimSpace(q.NodeID); nodeID != "" {
		w = append(w, `e.node_id = ?`)
		a = append(a, nodeID)
	}
	if len(q.EventKinds) > 0 {
		var in strings.Builder
		in.WriteString(`e.event_kind IN (`)
		for i, kind := range q.EventKinds {
			if i > 0 {
				in.WriteString(`,`)
			}
			in.WriteString(`?`)
			a = append(a, int(kind))
		}
		in.WriteString(`)`)
		w = append(w, in.String())
	}
	if len(w) > 0 {
		b.WriteString(` WHERE `)
		b.WriteString(strings.Join(w, ` AND `))
	}
	b.WriteString(` ORDER BY e.id DESC LIMIT ?`)
	a = append(a, limit)

	rows, err := s.db.QueryContext(ctx, b.String(), a...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.LogEventView, 0, limit)
	for rows.Next() {
		v, err := scanLogEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	return out, rows.Err()
}

func (s *Store) pruneLogEvents(ctx context.Context) error {
	if s.logMaxRows <= 0 {
		return nil
	}
	triggerOffset := s.logMaxRows + s.logPruneBatchRows
	_, err := s.db.ExecContext(ctx, `
WITH trigger AS (
	SELECT id FROM log_events ORDER BY id DESC LIMIT 1 OFFSET ?
),
cutoff AS (
	SELECT id FROM log_events ORDER BY id DESC LIMIT 1 OFFSET ?
)
DELETE FROM log_events
WHERE EXISTS (SELECT 1 FROM trigger)
  AND id <= (SELECT id FROM cutoff)
`, triggerOffset, s.logMaxRows)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "out of range") {
		return err
	}

	return nil
}
