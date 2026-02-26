package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	// Register SQLite driver.
	_ "modernc.org/sqlite"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/persistence/sqlite/migrations"
	"meshmap-lite/internal/repo"
)

// Store implements repository operations on top of SQLite.
type Store struct {
	db  *sql.DB
	log *slog.Logger
}

// Open creates a SQLite-backed store and optionally runs migrations.
func Open(ctx context.Context, dsn string, autoMigrate bool, log *slog.Logger) (*Store, error) {
	if log != nil {
		log.Info("opening sqlite database", "dsn", dsn, "auto_migrate", autoMigrate)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db, log: log}
	if autoMigrate {
		if s.log != nil {
			s.log.Info("running sqlite migrations")
		}
		if err := s.Migrate(ctx); err != nil {
			return nil, err
		}
		if s.log != nil {
			s.log.Info("sqlite migrations complete")
		}
	}
	if s.log != nil {
		s.log.Info("sqlite database ready")
	}

	return s, nil
}

// Close releases the underlying SQL database handle.
func (s *Store) Close() error {
	if s.log != nil {
		s.log.Info("closing sqlite database")
	}

	return s.db.Close()
}

// Migrate applies pending schema migrations.
func (s *Store) Migrate(ctx context.Context) error {
	if err := migrations.Apply(ctx, s.db); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}

	return nil
}

// UpsertNode inserts or updates node identity and liveness fields.
func (s *Store) UpsertNode(ctx context.Context, n domain.Node) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM nodes WHERE node_id = ?`, n.NodeID).Scan(&exists)
	created := errors.Is(err, sql.ErrNoRows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO nodes (
 node_id,node_num,long_name,short_name,role,board_model,firmware_version,lora_region,lora_frequency_desc,modem_preset,
 neighbor_nodes_count,mqtt_gateway_capable,first_seen_at,last_seen_any_event_at,last_seen_mqtt_gateway_at,last_seen_position_at,updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(node_id) DO UPDATE SET
 node_num=COALESCE(excluded.node_num,nodes.node_num),
 long_name=CASE WHEN excluded.long_name<>'' THEN excluded.long_name ELSE nodes.long_name END,
 short_name=CASE WHEN excluded.short_name<>'' THEN excluded.short_name ELSE nodes.short_name END,
 role=CASE WHEN excluded.role<>'' THEN excluded.role ELSE nodes.role END,
 board_model=CASE WHEN excluded.board_model<>'' THEN excluded.board_model ELSE nodes.board_model END,
 firmware_version=CASE WHEN excluded.firmware_version<>'' THEN excluded.firmware_version ELSE nodes.firmware_version END,
 lora_region=CASE WHEN excluded.lora_region<>'' THEN excluded.lora_region ELSE nodes.lora_region END,
 lora_frequency_desc=CASE WHEN excluded.lora_frequency_desc<>'' THEN excluded.lora_frequency_desc ELSE nodes.lora_frequency_desc END,
 modem_preset=CASE WHEN excluded.modem_preset<>'' THEN excluded.modem_preset ELSE nodes.modem_preset END,
 neighbor_nodes_count=COALESCE(excluded.neighbor_nodes_count,nodes.neighbor_nodes_count),
 mqtt_gateway_capable=COALESCE(excluded.mqtt_gateway_capable,nodes.mqtt_gateway_capable),
 last_seen_any_event_at=excluded.last_seen_any_event_at,
 last_seen_mqtt_gateway_at=COALESCE(excluded.last_seen_mqtt_gateway_at,nodes.last_seen_mqtt_gateway_at),
 last_seen_position_at=COALESCE(excluded.last_seen_position_at,nodes.last_seen_position_at),
 updated_at=excluded.updated_at
`, n.NodeID, ptrUint32(n.NodeNum), n.LongName, n.ShortName, n.Role, n.BoardModel, n.FirmwareVersion,
		n.LoRaRegion, n.LoRaFrequencyDesc, n.ModemPreset, ptrInt(n.NeighborNodesCount), ptrBool(n.MQTTGatewayCapable),
		n.FirstSeenAt.UTC().Format(time.RFC3339Nano), n.LastSeenAnyEventAt.UTC().Format(time.RFC3339Nano),
		ptrTime(n.LastSeenMQTTGatewayAt), ptrTime(n.LastSeenPositionAt), n.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}

	return created, nil
}

// UpsertPosition inserts or updates a node's latest position.
func (s *Store) UpsertPosition(ctx context.Context, p domain.NodePosition) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO node_positions(node_id,latitude,longitude,altitude_m,position_precision,source_kind,source_channel,reported_at,observed_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(node_id) DO UPDATE SET
 latitude=excluded.latitude,
 longitude=excluded.longitude,
 altitude_m=excluded.altitude_m,
 position_precision=excluded.position_precision,
 source_kind=excluded.source_kind,
 source_channel=excluded.source_channel,
 reported_at=excluded.reported_at,
 observed_at=excluded.observed_at,
 updated_at=excluded.updated_at
`, p.NodeID, p.Latitude, p.Longitude, ptrFloat(p.AltitudeM), ptrUint32(p.PositionPrecision), string(p.SourceKind), p.SourceChannel,
		ptrTime(p.ReportedAt), p.ObservedAt.UTC().Format(time.RFC3339Nano), p.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE nodes SET last_seen_position_at=?, updated_at=? WHERE node_id=?`, p.ObservedAt.UTC().Format(time.RFC3339Nano), p.UpdatedAt.UTC().Format(time.RFC3339Nano), p.NodeID)

	return err
}

// MergeTelemetry merges incoming telemetry with existing snapshot and persists it.
func (s *Store) MergeTelemetry(ctx context.Context, snap domain.NodeTelemetrySnapshot) error {
	cur, _ := s.getTelemetry(ctx, snap.NodeID)
	merged := domain.MergeTelemetry(cur, snap)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO node_telemetry_snapshots(
 node_id,power_voltage,power_battery_level,env_temperature_c,env_humidity,env_pressure_hpa,air_pm25,air_pm10,air_co2,air_iaq,source_channel,reported_at,observed_at,updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(node_id) DO UPDATE SET
 power_voltage=excluded.power_voltage,
 power_battery_level=excluded.power_battery_level,
 env_temperature_c=excluded.env_temperature_c,
 env_humidity=excluded.env_humidity,
 env_pressure_hpa=excluded.env_pressure_hpa,
 air_pm25=excluded.air_pm25,
 air_pm10=excluded.air_pm10,
 air_co2=excluded.air_co2,
 air_iaq=excluded.air_iaq,
 source_channel=excluded.source_channel,
 reported_at=excluded.reported_at,
 observed_at=excluded.observed_at,
 updated_at=excluded.updated_at
	`, merged.NodeID,
		ptrFloat(merged.Power.Voltage), ptrFloat(merged.Power.BatteryLevel),
		ptrFloat(merged.Environment.TemperatureC), ptrFloat(merged.Environment.Humidity), ptrFloat(merged.Environment.PressureHpa),
		ptrFloat(merged.AirQuality.PM25), ptrFloat(merged.AirQuality.PM10), ptrFloat(merged.AirQuality.CO2), ptrFloat(merged.AirQuality.IAQ),
		merged.SourceChannel, ptrTime(merged.ReportedAt), merged.ObservedAt.UTC().Format(time.RFC3339Nano), merged.UpdatedAt.UTC().Format(time.RFC3339Nano))

	return err
}

// InsertChatEvent appends a chat or system event and returns its row ID.
func (s *Store) InsertChatEvent(ctx context.Context, e domain.ChatEvent) (int64, error) {
	messageText := interface{}(e.MessageText)
	if e.EventType == domain.ChatEventSystem {
		messageText = nil
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO chat_events(event_type,channel_name,node_id,message_text,system_code,message_time,reported_at,observed_at,packet_id,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?)
`, string(e.EventType), e.ChannelName, nullIfEmpty(e.NodeID), messageText, nullIfEmpty(string(e.SystemCode)),
		e.MessageTime.UTC().Format(time.RFC3339Nano), ptrTime(e.ReportedAt), e.ObservedAt.UTC().Format(time.RFC3339Nano), ptrUint32(e.PacketID), e.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// GetMapNodes returns nodes with optional latest positions for map rendering.
func (s *Store) GetMapNodes(ctx context.Context) ([]repo.MapNode, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT n.node_id,n.node_num,n.long_name,n.short_name,n.role,n.board_model,n.firmware_version,n.lora_region,n.lora_frequency_desc,
       n.modem_preset,n.neighbor_nodes_count,n.mqtt_gateway_capable,n.first_seen_at,n.last_seen_any_event_at,n.last_seen_mqtt_gateway_at,n.last_seen_position_at,n.updated_at,
       p.latitude,p.longitude,p.altitude_m,p.position_precision,p.source_kind,p.source_channel,p.reported_at,p.observed_at,p.updated_at
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id
ORDER BY n.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]repo.MapNode, 0)
	for rows.Next() {
		n, p, err := scanMapNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, repo.MapNode{Node: n, Position: p})
	}

	return out, rows.Err()
}

// ListNodes returns compact node summaries sorted by last activity time.
func (s *Store) ListNodes(ctx context.Context) ([]repo.NodeSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT n.node_id,n.long_name,n.short_name,n.last_seen_any_event_at,n.last_seen_position_at,n.last_seen_mqtt_gateway_at,
       (p.node_id IS NOT NULL) has_position,n.role,n.board_model
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id
ORDER BY n.last_seen_any_event_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]repo.NodeSummary, 0)
	for rows.Next() {
		var id, longName, shortName, lastAny, role, board string
		var lastPos, lastMQTT sql.NullString
		var hasPos bool
		if err := rows.Scan(&id, &longName, &shortName, &lastAny, &lastPos, &lastMQTT, &hasPos, &role, &board); err != nil {
			return nil, err
		}
		la, _ := time.Parse(time.RFC3339Nano, lastAny)
		items = append(items, repo.NodeSummary{
			NodeID:             id,
			DisplayName:        displayName(longName, shortName, id),
			LastSeenAnyEventAt: la,
			LastSeenPositionAt: parseNullableTime(lastPos),
			LastSeenMQTTAt:     parseNullableTime(lastMQTT),
			HasPosition:        hasPos,
			Role:               role,
			BoardModel:         board,
		})
	}

	return items, rows.Err()
}

// GetNodeDetails returns full details for a node including position and telemetry.
func (s *Store) GetNodeDetails(ctx context.Context, nodeID string) (repo.NodeDetails, error) {
	var d repo.NodeDetails
	rows, err := s.db.QueryContext(ctx, `
SELECT n.node_id,n.node_num,n.long_name,n.short_name,n.role,n.board_model,n.firmware_version,n.lora_region,n.lora_frequency_desc,
       n.modem_preset,n.neighbor_nodes_count,n.mqtt_gateway_capable,n.first_seen_at,n.last_seen_any_event_at,n.last_seen_mqtt_gateway_at,n.last_seen_position_at,n.updated_at,
       p.latitude,p.longitude,p.altitude_m,p.position_precision,p.source_kind,p.source_channel,p.reported_at,p.observed_at,p.updated_at
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id
WHERE n.node_id=?`, nodeID)
	if err != nil {
		return d, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		n, p, err := scanMapNode(rows)
		if err != nil {
			return d, err
		}
		d.Node = n
		d.Position = p
	} else {
		return d, sql.ErrNoRows
	}
	t, _ := s.getTelemetry(ctx, nodeID)
	if t.NodeID != "" {
		d.Telemetry = &t
	}

	return d, nil
}

// ListChatEvents returns paginated chat timeline items for a channel.
func (s *Store) ListChatEvents(ctx context.Context, channel string, limit int, before int64) ([]domain.ChatEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	query := `
SELECT id,event_type,channel_name,node_id,message_text,system_code,message_time,reported_at,observed_at,packet_id,created_at
FROM chat_events
WHERE (LOWER(channel_name)=LOWER(?) OR channel_name='')`
	args := []interface{}{channel}
	if before > 0 {
		query += ` AND id < ?`
		args = append(args, before)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
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
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(observed_at) FROM chat_events`).Scan(&last); err == nil && last.Valid {
		if t, e := time.Parse(time.RFC3339Nano, last.String); e == nil {
			st.LastIngestAt = t
		}
	}

	return st, nil
}

func (s *Store) getTelemetry(ctx context.Context, nodeID string) (domain.NodeTelemetrySnapshot, error) {
	var out domain.NodeTelemetrySnapshot
	var reported sql.NullString
	var pv, pbl, etc, eh, eph, ap25, ap10, aco2, aiaq sql.NullFloat64
	var observed, updated, source string
	err := s.db.QueryRowContext(ctx, `
SELECT node_id,power_voltage,power_battery_level,env_temperature_c,env_humidity,env_pressure_hpa,air_pm25,air_pm10,air_co2,air_iaq,source_channel,reported_at,observed_at,updated_at
FROM node_telemetry_snapshots WHERE node_id=?`, nodeID).Scan(
		&out.NodeID, &pv, &pbl, &etc, &eh, &eph, &ap25, &ap10, &aco2, &aiaq, &source, &reported, &observed, &updated)
	if err != nil {
		return out, err
	}
	out.Power.Voltage = parseNullableFloat(pv)
	out.Power.BatteryLevel = parseNullableFloat(pbl)
	out.Environment.TemperatureC = parseNullableFloat(etc)
	out.Environment.Humidity = parseNullableFloat(eh)
	out.Environment.PressureHpa = parseNullableFloat(eph)
	out.AirQuality.PM25 = parseNullableFloat(ap25)
	out.AirQuality.PM10 = parseNullableFloat(ap10)
	out.AirQuality.CO2 = parseNullableFloat(aco2)
	out.AirQuality.IAQ = parseNullableFloat(aiaq)
	out.SourceChannel = source
	out.ReportedAt = parseNullableTime(reported)
	out.ObservedAt, _ = time.Parse(time.RFC3339Nano, observed)
	out.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)

	return out, nil
}

func scanMapNode(rows *sql.Rows) (domain.Node, *domain.NodePosition, error) {
	var n domain.Node
	var nodeNum sql.NullInt64
	var neighbor sql.NullInt64
	var gw sql.NullInt64
	var firstSeen, lastAny, lastMQTT, lastPos, updated sql.NullString
	var pLat, pLon, pAlt sql.NullFloat64
	var pPrec sql.NullInt64
	var pKind, pChannel, pReported, pObserved, pUpdated sql.NullString
	err := rows.Scan(&n.NodeID, &nodeNum, &n.LongName, &n.ShortName, &n.Role, &n.BoardModel, &n.FirmwareVersion, &n.LoRaRegion, &n.LoRaFrequencyDesc,
		&n.ModemPreset, &neighbor, &gw, &firstSeen, &lastAny, &lastMQTT, &lastPos, &updated,
		&pLat, &pLon, &pAlt, &pPrec, &pKind, &pChannel, &pReported, &pObserved, &pUpdated)
	if err != nil {
		return n, nil, err
	}
	if nodeNum.Valid {
		if nodeNum.Int64 >= 0 && nodeNum.Int64 <= math.MaxUint32 {
			v := uint32(nodeNum.Int64)
			n.NodeNum = &v
		}
	}
	if neighbor.Valid {
		v := int(neighbor.Int64)
		n.NeighborNodesCount = &v
	}
	if gw.Valid {
		v := gw.Int64 == 1
		n.MQTTGatewayCapable = &v
	}
	n.FirstSeenAt = mustTime(firstSeen)
	n.LastSeenAnyEventAt = mustTime(lastAny)
	n.LastSeenMQTTGatewayAt = parseNullableTime(lastMQTT)
	n.LastSeenPositionAt = parseNullableTime(lastPos)
	n.UpdatedAt = mustTime(updated)
	if !pLat.Valid || !pLon.Valid {
		return n, nil, nil
	}
	pos := &domain.NodePosition{NodeID: n.NodeID, Latitude: pLat.Float64, Longitude: pLon.Float64, SourceKind: domain.PositionSourceKind(pKind.String), SourceChannel: pChannel.String, ReportedAt: parseNullableTime(pReported), ObservedAt: mustTime(pObserved), UpdatedAt: mustTime(pUpdated)}
	if pAlt.Valid {
		v := pAlt.Float64
		pos.AltitudeM = &v
	}
	if pPrec.Valid {
		if pPrec.Int64 >= 0 && pPrec.Int64 <= math.MaxUint32 {
			v := uint32(pPrec.Int64)
			pos.PositionPrecision = &v
		}
	}

	return n, pos, nil
}

func scanChat(rows *sql.Rows) (domain.ChatEvent, error) {
	var e domain.ChatEvent
	var eventType, channel, nodeID, messageText, systemCode, msgTime, reported, observed, packetID, created sql.NullString
	if err := rows.Scan(&e.ID, &eventType, &channel, &nodeID, &messageText, &systemCode, &msgTime, &reported, &observed, &packetID, &created); err != nil {
		return e, err
	}
	e.EventType = domain.ChatEventType(eventType.String)
	e.ChannelName = channel.String
	e.NodeID = nodeID.String
	e.MessageText = messageText.String
	e.SystemCode = domain.ChatSystemCode(systemCode.String)
	e.MessageTime = mustTime(msgTime)
	e.ReportedAt = parseNullableTime(reported)
	e.ObservedAt = mustTime(observed)
	e.CreatedAt = mustTime(created)
	if packetID.Valid {
		if v, err := parseUint32(packetID.String); err == nil {
			e.PacketID = &v
		}
	}

	return e, nil
}

func displayName(longName, shortName, id string) string {
	if longName != "" {
		return longName
	}
	if shortName != "" {
		return shortName
	}

	return id
}

func mustTime(v sql.NullString) time.Time {
	if !v.Valid || v.String == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, v.String)

	return t
}

func parseNullableTime(v sql.NullString) *time.Time {
	if !v.Valid || v.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, v.String)
	if err != nil {
		return nil
	}

	return &t
}

func parseNullableFloat(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64

	return &x
}

func parseUint32(v string) (uint32, error) {
	var x uint32
	_, err := fmt.Sscanf(v, "%d", &x)

	return x, err
}

func ptrTime(v *time.Time) interface{} {
	if v == nil {
		return nil
	}

	return v.UTC().Format(time.RFC3339Nano)
}

func ptrFloat(v *float64) interface{} {
	if v == nil {
		return nil
	}

	return *v
}

func ptrUint32(v *uint32) interface{} {
	if v == nil {
		return nil
	}

	return int64(*v)
}

func ptrInt(v *int) interface{} {
	if v == nil {
		return nil
	}

	return *v
}

func ptrBool(v *bool) interface{} {
	if v == nil {
		return nil
	}
	if *v {
		return 1
	}

	return 0
}

func nullIfEmpty(v string) interface{} {
	if v == "" {
		return nil
	}

	return v
}
