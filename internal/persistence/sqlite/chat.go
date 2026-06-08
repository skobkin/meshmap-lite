package sqlite

import (
	"context"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// InsertChatEvent appends a chat or system event and returns its row ID.
func (s *Store) InsertChatEvent(ctx context.Context, e domain.ChatEvent) (int64, error) {
	messageText := interface{}(e.MessageText)
	if e.EventType == domain.ChatEventSystem {
		messageText = nil
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO chat_events(event_type,channel_name,node_id,message_text,system_code,message_time,reported_at,observed_at,packet_id,mqtt_uploader_node_id,hop_start,hop_limit,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
`, string(e.EventType), e.ChannelName, nullIfEmpty(e.NodeID), messageText, nullIfEmpty(string(e.SystemCode)),
		e.MessageTime.UTC().Format(time.RFC3339Nano), ptrTime(e.ReportedAt), e.ObservedAt.UTC().Format(time.RFC3339Nano), ptrUint32(e.PacketID), nullIfEmpty(e.MQTTUploaderNodeID), ptrUint32(e.HopStart), ptrUint32(e.HopLimit), e.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// ListChatEvents returns paginated chat timeline items for a channel.
func (s *Store) ListChatEvents(ctx context.Context, q repo.ChatEventQuery) ([]domain.ChatEvent, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 500 {
		q.Limit = 500
	}
	query := `
SELECT e.id,e.event_type,e.channel_name,e.node_id,n.long_name,n.short_name,
       e.message_text,e.system_code,e.message_time,e.reported_at,e.observed_at,e.packet_id,e.mqtt_uploader_node_id,e.hop_start,e.hop_limit,mu.long_name,mu.short_name,e.created_at
FROM chat_events e
LEFT JOIN nodes n ON n.node_id=e.node_id
LEFT JOIN nodes mu ON mu.node_id=e.mqtt_uploader_node_id
WHERE (LOWER(channel_name)=LOWER(?) OR channel_name='')`
	args := []interface{}{q.Channel}
	if q.BeforeID > 0 {
		query += ` AND e.id < ?`
		args = append(args, q.BeforeID)
	}
	if !q.ObservedSinceAt.IsZero() {
		query += ` AND e.observed_at >= ?`
		args = append(args, q.ObservedSinceAt.UTC().Format(time.RFC3339Nano))
	}
	query += ` ORDER BY e.id DESC LIMIT ?`
	args = append(args, q.Limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.ChatEvent, 0)
	for rows.Next() {
		v, err := scanChat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	return out, rows.Err()
}
