package sqlite

import (
	"context"
	"database/sql"
	"time"

	"meshmap-lite/internal/domain"
)

// MergeTelemetry merges incoming telemetry with existing snapshot and persists it.
func (s *Store) MergeTelemetry(ctx context.Context, snap domain.NodeTelemetrySnapshot) (domain.NodeTelemetrySnapshot, error) {
	cur, _ := s.getTelemetry(ctx, snap.NodeID, time.Time{})
	merged := domain.MergeTelemetry(cur, snap)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO node_telemetry_snapshots(
 node_id,power_voltage,power_battery_level,env_temperature_c,env_humidity,env_pressure_hpa,air_pm25,air_pm10,air_co2,air_iaq,util_ch_util,util_air_util_tx,dev_uptime_seconds,source_channel,mqtt_uploader_node_id,reported_at,observed_at,updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
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
 util_ch_util=excluded.util_ch_util,
 util_air_util_tx=excluded.util_air_util_tx,
 dev_uptime_seconds=excluded.dev_uptime_seconds,
 source_channel=excluded.source_channel,
 mqtt_uploader_node_id=excluded.mqtt_uploader_node_id,
 reported_at=excluded.reported_at,
 observed_at=excluded.observed_at,
 updated_at=excluded.updated_at
	`, merged.NodeID,
		ptrFloat(merged.Power.Voltage), ptrFloat(merged.Power.BatteryLevel),
		ptrFloat(merged.Environment.TemperatureC), ptrFloat(merged.Environment.Humidity), ptrFloat(merged.Environment.PressureHpa),
		ptrFloat(merged.AirQuality.PM25), ptrFloat(merged.AirQuality.PM10), ptrFloat(merged.AirQuality.CO2), ptrFloat(merged.AirQuality.IAQ),
		ptrFloat(merged.Utilization.ChUtil), ptrFloat(merged.Utilization.AirUtilTx), ptrUint32(merged.Device.UptimeSeconds),
		merged.SourceChannel, nullIfEmpty(merged.MQTTUploaderNodeID), ptrTime(merged.ReportedAt), merged.ObservedAt.UTC().Format(time.RFC3339Nano), merged.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return domain.NodeTelemetrySnapshot{}, err
	}

	return merged, nil
}

func (s *Store) getTelemetry(ctx context.Context, nodeID string, observedSince time.Time) (domain.NodeTelemetrySnapshot, error) {
	var nodeID2 string
	var pv, pbl, etc, eh, eph, ap25, ap10, aco2, aiaq sql.NullFloat64
	var utilCh, utilAir sql.NullFloat64
	var devUptime sql.NullInt64
	var source, uploaderID, uploaderLong, uploaderShort, reported sql.NullString
	var observed, updated sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT t.node_id,t.power_voltage,t.power_battery_level,t.env_temperature_c,t.env_humidity,t.env_pressure_hpa,t.air_pm25,t.air_pm10,t.air_co2,t.air_iaq,
       t.util_ch_util,t.util_air_util_tx,t.dev_uptime_seconds,
       t.source_channel,t.mqtt_uploader_node_id,mu.long_name,mu.short_name,t.reported_at,t.observed_at,t.updated_at
FROM node_telemetry_snapshots t
LEFT JOIN nodes mu ON mu.node_id=t.mqtt_uploader_node_id
WHERE t.node_id=?
  AND (?='' OR t.observed_at>=?)`, nodeID, cutoffParam(observedSince), cutoffParam(observedSince)).Scan(
		&nodeID2, &pv, &pbl, &etc, &eh, &eph, &ap25, &ap10, &aco2, &aiaq,
		&utilCh, &utilAir, &devUptime,
		&source, &uploaderID, &uploaderLong, &uploaderShort, &reported, &observed, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.NodeTelemetrySnapshot{}, nil
		}

		return domain.NodeTelemetrySnapshot{}, err
	}
	telemetry := scanTelemetryValues(nodeID2, pv, pbl, etc, eh, eph, ap25, ap10, aco2, aiaq,
		utilCh, utilAir, devUptime,
		source, uploaderID, uploaderLong, uploaderShort, reported, observed, updated)
	if telemetry == nil {
		return domain.NodeTelemetrySnapshot{}, nil
	}

	return *telemetry, nil
}
