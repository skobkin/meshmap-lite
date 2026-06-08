package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"meshmap-lite/internal/domain"
)

// scanTelemetryValues unpacks telemetry fields from nullable SQL types into a NodeTelemetrySnapshot.
// It consolidates telemetry unpacking logic to avoid inconsistencies across different scanner functions.
func scanTelemetryValues(nodeID string, pv, pbl, etc, eh, eph, ap25, ap10, aco2, aiaq sql.NullFloat64,
	source, uploaderID, uploaderLong, uploaderShort sql.NullString, reported sql.NullString, observed, updated sql.NullString) *domain.NodeTelemetrySnapshot {
	// Return nil if nodeID is empty (no row found)
	if nodeID == "" {
		return nil
	}

	out := &domain.NodeTelemetrySnapshot{
		NodeID: nodeID,
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
	out.SourceChannel = source.String
	out.MQTTUploaderNodeID = uploaderID.String
	out.MQTTUploaderDisplayName = displayName(uploaderLong.String, uploaderShort.String, uploaderID.String)
	out.ReportedAt = parseNullableTime(reported)
	if observed.Valid {
		out.ObservedAt = mustTime(observed)
	}
	if updated.Valid {
		out.UpdatedAt = mustTime(updated)
	}

	return out
}

func scanMapNode(rows *sql.Rows) (domain.Node, *domain.NodePosition, error) {
	var n domain.Node
	var nodeNum sql.NullInt64
	var hasDefaultCh sql.NullInt64
	var hasOptedReportLoc sql.NullInt64
	var neighbor sql.NullInt64
	var gw sql.NullInt64
	var firstSeen, lastAny, lastMQTT, lastUploaderID, lastUploaderLong, lastUploaderShort, lastUploaderAt, lastPos, updated sql.NullString
	var pLat, pLon, pAlt sql.NullFloat64
	var pPrec sql.NullInt64
	var pKind, pChannel, pUploaderID, pUploaderLong, pUploaderShort, pReported, pObserved, pUpdated sql.NullString
	err := rows.Scan(&n.NodeID, &nodeNum, &n.LongName, &n.ShortName, &n.Role, &n.BoardModel, &n.FirmwareVersion, &n.LoRaRegion, &n.LoRaFrequencyDesc,
		&n.ModemPreset, &hasDefaultCh, &hasOptedReportLoc, &neighbor, &gw, &firstSeen, &lastAny, &lastMQTT,
		&lastUploaderID, &lastUploaderLong, &lastUploaderShort, &lastUploaderAt, &lastPos, &updated,
		&pLat, &pLon, &pAlt, &pPrec, &pKind, &pChannel, &pUploaderID, &pUploaderLong, &pUploaderShort, &pReported, &pObserved, &pUpdated)
	if err != nil {
		return n, nil, err
	}
	if nodeNum.Valid {
		if v, ok := checkedUint32FromInt64(nodeNum.Int64); ok {
			n.NodeNum = &v
		}
	}
	if neighbor.Valid {
		v := int(neighbor.Int64)
		n.NeighborNodesCount = &v
	}
	if hasDefaultCh.Valid {
		v := hasDefaultCh.Int64 == 1
		n.HasDefaultChannel = &v
	}
	if hasOptedReportLoc.Valid {
		v := hasOptedReportLoc.Int64 == 1
		n.HasOptedReportLocation = &v
	}
	if gw.Valid {
		v := gw.Int64 == 1
		n.MQTTGatewayCapable = &v
	}
	n.FirstSeenAt = mustTime(firstSeen)
	n.LastSeenAnyEventAt = mustTime(lastAny)
	n.LastSeenMQTTGatewayAt = parseNullableTime(lastMQTT)
	n.LastMQTTUploaderNodeID = lastUploaderID.String
	n.LastMQTTUploaderDisplayName = displayName(lastUploaderLong.String, lastUploaderShort.String, lastUploaderID.String)
	n.LastMQTTUploaderAt = parseNullableTime(lastUploaderAt)
	n.LastSeenPositionAt = parseNullableTime(lastPos)
	n.UpdatedAt = mustTime(updated)
	if !pLat.Valid || !pLon.Valid {
		return n, nil, nil
	}
	pos := &domain.NodePosition{NodeID: n.NodeID, Latitude: pLat.Float64, Longitude: pLon.Float64, SourceKind: domain.PositionSourceKind(pKind.String), SourceChannel: pChannel.String, MQTTUploaderNodeID: pUploaderID.String, MQTTUploaderDisplayName: displayName(pUploaderLong.String, pUploaderShort.String, pUploaderID.String), ReportedAt: parseNullableTime(pReported), ObservedAt: mustTime(pObserved), UpdatedAt: mustTime(pUpdated)}
	if pAlt.Valid {
		v := pAlt.Float64
		pos.AltitudeM = &v
	}
	if pPrec.Valid {
		if v, ok := checkedUint32FromInt64(pPrec.Int64); ok {
			pos.PositionPrecision = &v
		}
	}

	return n, pos, nil
}

func scanMapNodeWithTelemetry(rows *sql.Rows) (domain.Node, *domain.NodePosition, *domain.NodeTelemetrySnapshot, error) {
	var n domain.Node
	var nodeNum sql.NullInt64
	var hasDefaultCh sql.NullInt64
	var hasOptedReportLoc sql.NullInt64
	var neighbor sql.NullInt64
	var gw sql.NullInt64
	var firstSeen, lastAny, lastMQTT, lastUploaderID, lastUploaderLong, lastUploaderShort, lastUploaderAt, lastPos, updated sql.NullString
	var pLat, pLon, pAlt sql.NullFloat64
	var pPrec sql.NullInt64
	var pKind, pChannel, pUploaderID, pUploaderLong, pUploaderShort, pReported, pObserved, pUpdated sql.NullString
	var tNodeID sql.NullString
	var tPv, tPbl, tEtc, tEh, tEph, tAp25, tAp10, tAco2, tAiaq sql.NullFloat64
	var tSource, tUploaderID, tUploaderLong, tUploaderShort, tReported, tObserved, tUpdated sql.NullString

	err := rows.Scan(&n.NodeID, &nodeNum, &n.LongName, &n.ShortName, &n.Role, &n.BoardModel, &n.FirmwareVersion, &n.LoRaRegion, &n.LoRaFrequencyDesc,
		&n.ModemPreset, &hasDefaultCh, &hasOptedReportLoc, &neighbor, &gw, &firstSeen, &lastAny, &lastMQTT,
		&lastUploaderID, &lastUploaderLong, &lastUploaderShort, &lastUploaderAt, &lastPos, &updated,
		&pLat, &pLon, &pAlt, &pPrec, &pKind, &pChannel, &pUploaderID, &pUploaderLong, &pUploaderShort, &pReported, &pObserved, &pUpdated,
		&tNodeID, &tPv, &tPbl, &tEtc, &tEh, &tEph, &tAp25, &tAp10, &tAco2, &tAiaq, &tSource, &tUploaderID, &tUploaderLong, &tUploaderShort, &tReported, &tObserved, &tUpdated)
	if err != nil {
		return n, nil, nil, err
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
	if hasDefaultCh.Valid {
		v := hasDefaultCh.Int64 == 1
		n.HasDefaultChannel = &v
	}
	if hasOptedReportLoc.Valid {
		v := hasOptedReportLoc.Int64 == 1
		n.HasOptedReportLocation = &v
	}
	if gw.Valid {
		v := gw.Int64 == 1
		n.MQTTGatewayCapable = &v
	}
	n.FirstSeenAt = mustTime(firstSeen)
	n.LastSeenAnyEventAt = mustTime(lastAny)
	n.LastSeenMQTTGatewayAt = parseNullableTime(lastMQTT)
	n.LastMQTTUploaderNodeID = lastUploaderID.String
	n.LastMQTTUploaderDisplayName = displayName(lastUploaderLong.String, lastUploaderShort.String, lastUploaderID.String)
	n.LastMQTTUploaderAt = parseNullableTime(lastUploaderAt)
	n.LastSeenPositionAt = parseNullableTime(lastPos)
	n.UpdatedAt = mustTime(updated)

	if !pLat.Valid || !pLon.Valid {
		return n, nil, nil, nil
	}
	pos := &domain.NodePosition{NodeID: n.NodeID, Latitude: pLat.Float64, Longitude: pLon.Float64, SourceKind: domain.PositionSourceKind(pKind.String), SourceChannel: pChannel.String, MQTTUploaderNodeID: pUploaderID.String, MQTTUploaderDisplayName: displayName(pUploaderLong.String, pUploaderShort.String, pUploaderID.String), ReportedAt: parseNullableTime(pReported), ObservedAt: mustTime(pObserved), UpdatedAt: mustTime(pUpdated)}
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

	telemetryNodeID := ""
	if tNodeID.Valid {
		telemetryNodeID = tNodeID.String
	}
	telemetry := scanTelemetryValues(telemetryNodeID, tPv, tPbl, tEtc, tEh, tEph, tAp25, tAp10, tAco2, tAiaq,
		tSource, tUploaderID, tUploaderLong, tUploaderShort, tReported, tObserved, tUpdated)

	return n, pos, telemetry, nil
}

func scanChat(rows *sql.Rows) (domain.ChatEvent, error) {
	var e domain.ChatEvent
	var eventType, channel, nodeID, longName, shortName, messageText, systemCode, msgTime, reported, observed, packetID, uploaderID, uploaderLong, uploaderShort, created sql.NullString
	var hopStart, hopLimit sql.NullInt64
	if err := rows.Scan(&e.ID, &eventType, &channel, &nodeID, &longName, &shortName, &messageText, &systemCode, &msgTime, &reported, &observed, &packetID, &uploaderID, &hopStart, &hopLimit, &uploaderLong, &uploaderShort, &created); err != nil {
		return e, err
	}
	e.EventType = domain.ChatEventType(eventType.String)
	e.ChannelName = channel.String
	e.NodeID = nodeID.String
	if e.NodeID != "" {
		e.NodeDisplay = displayName(longName.String, shortName.String, e.NodeID)
	}
	e.MessageText = messageText.String
	e.SystemCode = domain.ChatSystemCode(systemCode.String)
	e.MessageTime = mustTime(msgTime)
	e.ReportedAt = parseNullableTime(reported)
	e.ObservedAt = mustTime(observed)
	e.MQTTUploaderNodeID = uploaderID.String
	e.MQTTUploaderDisplayName = displayName(uploaderLong.String, uploaderShort.String, uploaderID.String)
	e.CreatedAt = mustTime(created)
	if packetID.Valid {
		if v, err := parseUint32(packetID.String); err == nil {
			e.PacketID = &v
		}
	}
	if hopStart.Valid {
		e.HopStart = checkedUint32Ptr(hopStart.Int64)
	}
	if hopLimit.Valid {
		e.HopLimit = checkedUint32Ptr(hopLimit.Int64)
	}

	return e, nil
}

func scanLogEvent(rows *sql.Rows) (domain.LogEventView, error) {
	var out domain.LogEventView
	var observed, nodeID, channel, longName, shortName, uploaderID, uploaderLong, uploaderShort, detailsJSON sql.NullString
	var hopStart, hopLimit sql.NullInt64
	var kind, encrypted int
	if err := rows.Scan(&out.ID, &observed, &nodeID, &kind, &encrypted, &channel, &longName, &shortName, &uploaderID, &uploaderLong, &uploaderShort, &detailsJSON, &hopStart, &hopLimit); err != nil {
		return out, err
	}
	out.ObservedAt = mustTime(observed)
	out.NodeID = nodeID.String
	if parsedKind, ok := domain.LogEventKindFromInt(kind); ok {
		out.EventKindValue = parsedKind
	}
	out.EventKindTitle = domain.LogEventKindTitle(out.EventKindValue)
	out.Encrypted = encrypted == 1
	if channel.Valid && channel.String != "" {
		ch := channel.String
		out.ChannelName = &ch
	}
	if out.NodeID != "" {
		out.NodeDisplay = displayName(longName.String, shortName.String, out.NodeID)
	}
	out.MQTTUploaderNodeID = uploaderID.String
	out.MQTTUploaderDisplayName = displayName(uploaderLong.String, uploaderShort.String, uploaderID.String)
	if detailsJSON.Valid && detailsJSON.String != "" {
		var details map[string]any
		if err := json.Unmarshal([]byte(detailsJSON.String), &details); err == nil && len(details) > 0 {
			out.Details = details
		}
	}
	if hopStart.Valid {
		out.HopStart = checkedUint32Ptr(hopStart.Int64)
	}
	if hopLimit.Valid {
		out.HopLimit = checkedUint32Ptr(hopLimit.Int64)
	}

	return out, nil
}

func scanTopologyEdge(rows *sql.Rows) (domain.TopologyEdge, error) {
	var out domain.TopologyEdge
	var (
		channelName, reportedBy, neighborLastRXAt, firstObserved, lastObserved, lastReported, updated sql.NullString
		snr                                                                                           sql.NullFloat64
		broadcastInterval                                                                             sql.NullInt64
		inferred, sourceKindValue                                                                     int
	)
	if err := rows.Scan(
		&sourceKindValue,
		&channelName,
		&out.FromNodeID,
		&out.ToNodeID,
		&reportedBy,
		&inferred,
		&snr,
		&neighborLastRXAt,
		&broadcastInterval,
		&firstObserved,
		&lastObserved,
		&lastReported,
		&updated,
	); err != nil {
		return out, err
	}
	if kind, ok := domain.TopologySourceKindFromInt(sourceKindValue); ok {
		out.SourceKind = kind
	}
	out.ChannelName = channelName.String
	out.ReportedByNodeID = reportedBy.String
	out.Inferred = inferred == 1
	out.SNR = parseNullableFloat(snr)
	out.NeighborLastRXAt = parseNullableTime(neighborLastRXAt)
	if broadcastInterval.Valid {
		if v, ok := checkedUint32FromInt64(broadcastInterval.Int64); ok {
			out.NeighborBroadcastIntervalSec = &v
		}
	}
	out.FirstObservedAt = mustTime(firstObserved)
	out.LastObservedAt = mustTime(lastObserved)
	out.LastReportedAt = parseNullableTime(lastReported)
	out.UpdatedAt = mustTime(updated)

	return out, nil
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

func checkedUint32FromInt64(v int64) (uint32, bool) {
	if v < 0 || v > math.MaxUint32 {
		return 0, false
	}

	//nolint:gosec // Safe: value is range-checked to fit into uint32 above.
	return uint32(v), true
}

func checkedUint32Ptr(v int64) *uint32 {
	if v < 0 || v > math.MaxUint32 {
		return nil
	}
	x := uint32(v)
	return &x
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

func cutoffParam(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339Nano)
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

func boolAsInt(v bool) int {
	if v {
		return 1
	}

	return 0
}
