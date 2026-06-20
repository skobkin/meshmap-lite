package domain

import "testing"

func TestMergeTelemetryPreservesMissingAndAllowsZero(t *testing.T) {
	v1 := 4.2
	temp := 20.0
	cur := NodeTelemetrySnapshot{}
	cur.Power.Voltage = &v1
	cur.Environment.TemperatureC = &temp

	zero := 0.0
	inc := NodeTelemetrySnapshot{}
	inc.Power.BatteryLevel = &zero

	out := MergeTelemetry(cur, inc)
	if out.Power.Voltage == nil || *out.Power.Voltage != 4.2 {
		t.Fatalf("expected voltage preserved")
	}
	if out.Environment.TemperatureC == nil || *out.Environment.TemperatureC != 20 {
		t.Fatalf("expected temperature preserved")
	}
	if out.Power.BatteryLevel == nil || *out.Power.BatteryLevel != 0 {
		t.Fatalf("expected zero to overwrite as valid")
	}
}

func TestMergeTelemetryUtilizationAndDevice(t *testing.T) {
	chutil := 12.5
	airutil := 3.2
	uptime := uint32(86400)

	// Round 1: incoming has values; current is empty.
	cur := NodeTelemetrySnapshot{}
	inc := NodeTelemetrySnapshot{}
	inc.Utilization.ChUtil = &chutil
	inc.Utilization.AirUtilTx = &airutil
	inc.Device.UptimeSeconds = &uptime

	out := MergeTelemetry(cur, inc)
	if out.Utilization.ChUtil == nil || *out.Utilization.ChUtil != 12.5 {
		t.Fatalf("expected ch_util set, got %#v", out.Utilization.ChUtil)
	}
	if out.Utilization.AirUtilTx == nil || *out.Utilization.AirUtilTx != 3.2 {
		t.Fatalf("expected air_util_tx set, got %#v", out.Utilization.AirUtilTx)
	}
	if out.Device.UptimeSeconds == nil || *out.Device.UptimeSeconds != 86400 {
		t.Fatalf("expected uptime_seconds set, got %#v", out.Device.UptimeSeconds)
	}

	// Round 2: incoming has nil for the same fields; previous values must be preserved.
	inc2 := NodeTelemetrySnapshot{}
	out2 := MergeTelemetry(out, inc2)
	if out2.Utilization.ChUtil == nil || *out2.Utilization.ChUtil != 12.5 {
		t.Fatalf("expected ch_util preserved from previous merge, got %#v", out2.Utilization.ChUtil)
	}
	if out2.Utilization.AirUtilTx == nil || *out2.Utilization.AirUtilTx != 3.2 {
		t.Fatalf("expected air_util_tx preserved from previous merge, got %#v", out2.Utilization.AirUtilTx)
	}
	if out2.Device.UptimeSeconds == nil || *out2.Device.UptimeSeconds != 86400 {
		t.Fatalf("expected uptime_seconds preserved from previous merge, got %#v", out2.Device.UptimeSeconds)
	}

	// Round 3: zero is valid and should overwrite the previous non-zero value.
	zero := 0.0
	zeroU := uint32(0)
	inc3 := NodeTelemetrySnapshot{}
	inc3.Utilization.ChUtil = &zero
	inc3.Utilization.AirUtilTx = &zero
	inc3.Device.UptimeSeconds = &zeroU

	out3 := MergeTelemetry(out2, inc3)
	if out3.Utilization.ChUtil == nil || *out3.Utilization.ChUtil != 0 {
		t.Fatalf("expected ch_util overwritten with zero (valid), got %#v", out3.Utilization.ChUtil)
	}
	if out3.Utilization.AirUtilTx == nil || *out3.Utilization.AirUtilTx != 0 {
		t.Fatalf("expected air_util_tx overwritten with zero (valid), got %#v", out3.Utilization.AirUtilTx)
	}
	if out3.Device.UptimeSeconds == nil || *out3.Device.UptimeSeconds != 0 {
		t.Fatalf("expected uptime_seconds overwritten with zero (valid), got %#v", out3.Device.UptimeSeconds)
	}
}
