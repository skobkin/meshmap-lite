package domain

// MergeTelemetry overlays non-nil incoming telemetry values on top of current snapshot.
func MergeTelemetry(current NodeTelemetrySnapshot, incoming NodeTelemetrySnapshot) NodeTelemetrySnapshot {
	merged := current
	merged.NodeID = incoming.NodeID
	mergeFloat(&merged.Power.Voltage, incoming.Power.Voltage)
	mergeFloat(&merged.Power.BatteryLevel, incoming.Power.BatteryLevel)
	mergeFloat(&merged.Environment.TemperatureC, incoming.Environment.TemperatureC)
	mergeFloat(&merged.Environment.Humidity, incoming.Environment.Humidity)
	mergeFloat(&merged.Environment.PressureHpa, incoming.Environment.PressureHpa)
	mergeFloat(&merged.AirQuality.PM25, incoming.AirQuality.PM25)
	mergeFloat(&merged.AirQuality.PM10, incoming.AirQuality.PM10)
	mergeFloat(&merged.AirQuality.CO2, incoming.AirQuality.CO2)
	mergeFloat(&merged.AirQuality.IAQ, incoming.AirQuality.IAQ)
	if incoming.SourceChannel != "" {
		merged.SourceChannel = incoming.SourceChannel
	}
	if incoming.ReportedAt != nil {
		merged.ReportedAt = incoming.ReportedAt
	}
	if !incoming.ObservedAt.IsZero() {
		merged.ObservedAt = incoming.ObservedAt
	}
	if !incoming.UpdatedAt.IsZero() {
		merged.UpdatedAt = incoming.UpdatedAt
	}

	return merged
}

func mergeFloat(dst **float64, src *float64) {
	if src == nil {
		return
	}
	v := *src
	*dst = &v
}
