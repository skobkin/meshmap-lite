package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// UpsertNode inserts or updates node identity and liveness fields.
func (s *Store) UpsertNode(ctx context.Context, n domain.Node) (bool, error) {
	firstSeenAt := n.FirstSeenAt.UTC().Format(time.RFC3339Nano)
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentLongName, currentShortName sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT long_name, short_name FROM nodes WHERE node_id = ?`, n.NodeID).Scan(&currentLongName, &currentShortName)
	exists := true
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		exists = false
	}

	if exists {
		previousLongName := currentLongName.String
		previousShortName := currentShortName.String
		nextLongName := previousLongName
		if n.LongName != "" {
			nextLongName = n.LongName
		}
		nextShortName := previousShortName
		if n.ShortName != "" {
			nextShortName = n.ShortName
		}
		if nextLongName != previousLongName || nextShortName != previousShortName {
			changedAt := n.LastSeenAnyEventAt
			if changedAt.IsZero() {
				changedAt = n.UpdatedAt
			}
			if changedAt.IsZero() {
				changedAt = now
			}
			if _, err := tx.ExecContext(ctx, `
INSERT INTO node_name_history(node_id,previous_long_name,previous_short_name,new_long_name,new_short_name,changed_at,created_at)
VALUES(?,?,?,?,?,?,?)`, n.NodeID, previousLongName, previousShortName, nextLongName, nextShortName, changedAt.UTC().Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
				return false, err
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO nodes (
 node_id,node_num,long_name,short_name,role,board_model,firmware_version,lora_region,lora_frequency_desc,modem_preset,
 has_default_channel,has_opted_report_location,neighbor_nodes_count,mqtt_gateway_capable,first_seen_at,last_seen_any_event_at,last_seen_mqtt_gateway_at,last_mqtt_uploader_node_id,last_mqtt_uploader_at,last_seen_position_at,updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
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
 has_default_channel=COALESCE(excluded.has_default_channel,nodes.has_default_channel),
 has_opted_report_location=COALESCE(excluded.has_opted_report_location,nodes.has_opted_report_location),
 neighbor_nodes_count=COALESCE(excluded.neighbor_nodes_count,nodes.neighbor_nodes_count),
 mqtt_gateway_capable=COALESCE(excluded.mqtt_gateway_capable,nodes.mqtt_gateway_capable),
 last_seen_any_event_at=excluded.last_seen_any_event_at,
 last_seen_mqtt_gateway_at=COALESCE(excluded.last_seen_mqtt_gateway_at,nodes.last_seen_mqtt_gateway_at),
 last_mqtt_uploader_node_id=COALESCE(NULLIF(excluded.last_mqtt_uploader_node_id,''),nodes.last_mqtt_uploader_node_id),
 last_mqtt_uploader_at=COALESCE(excluded.last_mqtt_uploader_at,nodes.last_mqtt_uploader_at),
 last_seen_position_at=COALESCE(excluded.last_seen_position_at,nodes.last_seen_position_at),
 updated_at=excluded.updated_at
	`, n.NodeID, ptrUint32(n.NodeNum), n.LongName, n.ShortName, n.Role, n.BoardModel, n.FirmwareVersion,
		n.LoRaRegion, n.LoRaFrequencyDesc, n.ModemPreset, ptrBool(n.HasDefaultChannel), ptrBool(n.HasOptedReportLocation), ptrInt(n.NeighborNodesCount), ptrBool(n.MQTTGatewayCapable),
		firstSeenAt, n.LastSeenAnyEventAt.UTC().Format(time.RFC3339Nano),
		ptrTime(n.LastSeenMQTTGatewayAt), n.LastMQTTUploaderNodeID, ptrTime(n.LastMQTTUploaderAt), ptrTime(n.LastSeenPositionAt), n.UpdatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}

	return !exists, nil
}

// UpsertPosition inserts or updates a node's latest position.
func (s *Store) UpsertPosition(ctx context.Context, p domain.NodePosition) error {
	if !domain.IsValidPosition(p.Latitude, p.Longitude) {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
INSERT INTO node_positions(node_id,latitude,longitude,altitude_m,position_precision,source_kind,source_channel,mqtt_uploader_node_id,reported_at,observed_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(node_id) DO UPDATE SET
 latitude=excluded.latitude,
 longitude=excluded.longitude,
 altitude_m=excluded.altitude_m,
 position_precision=excluded.position_precision,
 source_kind=excluded.source_kind,
 source_channel=excluded.source_channel,
 mqtt_uploader_node_id=excluded.mqtt_uploader_node_id,
 reported_at=excluded.reported_at,
 observed_at=excluded.observed_at,
 updated_at=excluded.updated_at
`, p.NodeID, p.Latitude, p.Longitude, ptrFloat(p.AltitudeM), ptrUint32(p.PositionPrecision), string(p.SourceKind), p.SourceChannel, nullIfEmpty(p.MQTTUploaderNodeID),
		ptrTime(p.ReportedAt), p.ObservedAt.UTC().Format(time.RFC3339Nano), p.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE nodes SET last_seen_position_at=?, updated_at=? WHERE node_id=?`, p.ObservedAt.UTC().Format(time.RFC3339Nano), p.UpdatedAt.UTC().Format(time.RFC3339Nano), p.NodeID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ResolveNodeDisplay returns the best-known user-facing node label.
func (s *Store) ResolveNodeDisplay(ctx context.Context, nodeID string) (string, error) {
	if strings.TrimSpace(nodeID) == "" {
		return "", nil
	}

	var longName, shortName sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT long_name,short_name FROM nodes WHERE node_id=?`, nodeID).Scan(&longName, &shortName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nodeID, nil
		}

		return "", err
	}

	return displayName(longName.String, shortName.String, nodeID), nil
}

// GetMapNodes returns nodes with relevant latest positions for map rendering.
func (s *Store) GetMapNodes(ctx context.Context, q repo.MapNodeQuery) ([]repo.MapNode, error) {
	positionCutoff := cutoffParam(q.PositionObservedSince)
	telemetryCutoff := cutoffParam(q.TelemetryObservedSince)
	rows, err := s.db.QueryContext(ctx, `
SELECT n.node_id,n.node_num,n.long_name,n.short_name,n.role,n.board_model,n.firmware_version,n.lora_region,n.lora_frequency_desc,
       n.modem_preset,n.has_default_channel,n.has_opted_report_location,n.neighbor_nodes_count,n.mqtt_gateway_capable,n.first_seen_at,n.last_seen_any_event_at,n.last_seen_mqtt_gateway_at,
       n.last_mqtt_uploader_node_id,nu.long_name,nu.short_name,n.last_mqtt_uploader_at,n.last_seen_position_at,n.updated_at,
       p.latitude,p.longitude,p.altitude_m,p.position_precision,p.source_kind,p.source_channel,p.mqtt_uploader_node_id,pu.long_name,pu.short_name,p.reported_at,p.observed_at,p.updated_at,
       t.node_id,t.power_voltage,t.power_battery_level,t.env_temperature_c,t.env_humidity,t.env_pressure_hpa,t.air_pm25,t.air_pm10,t.air_co2,t.air_iaq,t.source_channel,t.mqtt_uploader_node_id,tu.long_name,tu.short_name,t.reported_at,t.observed_at,t.updated_at
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id
LEFT JOIN node_telemetry_snapshots t ON t.node_id=n.node_id AND (?='' OR t.observed_at>=?)
LEFT JOIN nodes nu ON nu.node_id=n.last_mqtt_uploader_node_id
LEFT JOIN nodes pu ON pu.node_id=p.mqtt_uploader_node_id
LEFT JOIN nodes tu ON tu.node_id=t.mqtt_uploader_node_id
WHERE p.node_id IS NOT NULL
  AND (?='' OR p.observed_at >= ?)
ORDER BY n.updated_at DESC`, telemetryCutoff, telemetryCutoff, positionCutoff, positionCutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]repo.MapNode, 0)
	for rows.Next() {
		n, p, t, err := scanMapNodeWithTelemetry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, repo.MapNode{Node: n, Position: p, Telemetry: t})
	}

	return out, rows.Err()
}

// ListNodes returns compact node summaries sorted by last activity time.
func (s *Store) ListNodes(ctx context.Context, q repo.NodeListQuery) ([]repo.NodeSummary, error) {
	positionCutoff := cutoffParam(q.PositionObservedSince)
	rows, err := s.db.QueryContext(ctx, `
SELECT n.node_id,n.long_name,n.short_name,n.last_seen_any_event_at,n.last_seen_position_at,n.last_seen_mqtt_gateway_at,
       n.last_mqtt_uploader_node_id,nu.long_name,nu.short_name,n.last_mqtt_uploader_at,
       (p.node_id IS NOT NULL AND (?='' OR p.observed_at>=?)) has_position,n.role,n.board_model
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id
LEFT JOIN nodes nu ON nu.node_id=n.last_mqtt_uploader_node_id
ORDER BY n.last_seen_any_event_at DESC`, positionCutoff, positionCutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]repo.NodeSummary, 0)
	for rows.Next() {
		var id, lastAny string
		var longName, shortName, role, board sql.NullString
		var lastPos, lastMQTT, uploaderID, uploaderLong, uploaderShort, uploaderAt sql.NullString
		var hasPos bool
		if err := rows.Scan(&id, &longName, &shortName, &lastAny, &lastPos, &lastMQTT, &uploaderID, &uploaderLong, &uploaderShort, &uploaderAt, &hasPos, &role, &board); err != nil {
			return nil, err
		}
		la, _ := time.Parse(time.RFC3339Nano, lastAny)
		var lastSeenPositionAt *time.Time
		if hasPos {
			lastSeenPositionAt = parseNullableTime(lastPos)
		}
		items = append(items, repo.NodeSummary{
			NodeID:                      id,
			DisplayName:                 displayName(longName.String, shortName.String, id),
			LongName:                    longName.String,
			ShortName:                   shortName.String,
			LastSeenAnyEventAt:          la,
			LastSeenPositionAt:          lastSeenPositionAt,
			LastSeenMQTTAt:              parseNullableTime(lastMQTT),
			LastMQTTUploaderNodeID:      uploaderID.String,
			LastMQTTUploaderDisplayName: displayName(uploaderLong.String, uploaderShort.String, uploaderID.String),
			LastMQTTUploaderAt:          parseNullableTime(uploaderAt),
			HasPosition:                 hasPos,
			Role:                        role.String,
			BoardModel:                  board.String,
		})
	}

	return items, rows.Err()
}

// GetNodeDetails returns full details for a node including relevant position and telemetry.
func (s *Store) GetNodeDetails(ctx context.Context, q repo.NodeDetailsQuery) (repo.NodeDetails, error) {
	var d repo.NodeDetails
	positionCutoff := cutoffParam(q.PositionObservedSince)
	rows, err := s.db.QueryContext(ctx, `
SELECT n.node_id,n.node_num,n.long_name,n.short_name,n.role,n.board_model,n.firmware_version,n.lora_region,n.lora_frequency_desc,
       n.modem_preset,n.has_default_channel,n.has_opted_report_location,n.neighbor_nodes_count,n.mqtt_gateway_capable,n.first_seen_at,n.last_seen_any_event_at,n.last_seen_mqtt_gateway_at,
       n.last_mqtt_uploader_node_id,nu.long_name,nu.short_name,n.last_mqtt_uploader_at,n.last_seen_position_at,n.updated_at,
       p.latitude,p.longitude,p.altitude_m,p.position_precision,p.source_kind,p.source_channel,p.mqtt_uploader_node_id,pu.long_name,pu.short_name,p.reported_at,p.observed_at,p.updated_at
FROM nodes n
LEFT JOIN node_positions p ON p.node_id=n.node_id AND (?='' OR p.observed_at>=?)
LEFT JOIN nodes nu ON nu.node_id=n.last_mqtt_uploader_node_id
LEFT JOIN nodes pu ON pu.node_id=p.mqtt_uploader_node_id
WHERE n.node_id=?`, positionCutoff, positionCutoff, q.NodeID)
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
	if err := rows.Close(); err != nil {
		return d, err
	}
	if err := rows.Err(); err != nil {
		return d, err
	}
	t, _ := s.getTelemetry(ctx, q.NodeID, q.TelemetryObservedSince)
	if t.NodeID != "" {
		d.Telemetry = &t
	}
	neighbors, err := s.getNodeNeighbors(ctx, q.NodeID, q.TopologyUpdatedSince, q.PositionObservedSince)
	if err != nil {
		return d, err
	}
	d.Neighbors = neighbors
	previousNames, err := s.getNodeNameHistory(ctx, q.NodeID)
	if err != nil {
		return d, err
	}
	d.PreviousNames = previousNames

	return d, nil
}

func (s *Store) getNodeNameHistory(ctx context.Context, nodeID string) ([]repo.NodeNameHistory, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT previous_long_name,previous_short_name,new_long_name,new_short_name,changed_at
FROM node_name_history
WHERE node_id=?
ORDER BY changed_at DESC, id DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]repo.NodeNameHistory, 0)
	for rows.Next() {
		var item repo.NodeNameHistory
		var changedAt sql.NullString
		if err := rows.Scan(&item.PreviousLongName, &item.PreviousShortName, &item.NewLongName, &item.NewShortName, &changedAt); err != nil {
			return nil, err
		}
		item.ChangedAt = mustTime(changedAt)
		items = append(items, item)
	}

	return items, rows.Err()
}
