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
