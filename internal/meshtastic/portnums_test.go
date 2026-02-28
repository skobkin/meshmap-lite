package meshtastic

import (
	"testing"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

func TestDecodeTelemetryPayloadPreservesExplicitZeroValues(t *testing.T) {
	zeroFloat := float32(0)
	zeroUint := uint32(0)

	testCases := []struct {
		name   string
		msg    *generated.Telemetry
		assert func(*testing.T, *TelemetryPayload)
	}{
		{
			name: "device metrics",
			msg: &generated.Telemetry{
				Variant: &generated.Telemetry_DeviceMetrics{
					DeviceMetrics: &generated.DeviceMetrics{
						Voltage:      &zeroFloat,
						BatteryLevel: &zeroUint,
					},
				},
			},
			assert: func(t *testing.T, telemetry *TelemetryPayload) {
				if telemetry.Power.Voltage == nil || *telemetry.Power.Voltage != 0 {
					t.Fatalf("expected voltage pointer with zero value, got %#v", telemetry.Power.Voltage)
				}
				if telemetry.Power.BatteryLevel == nil || *telemetry.Power.BatteryLevel != 0 {
					t.Fatalf("expected battery pointer with zero value, got %#v", telemetry.Power.BatteryLevel)
				}
			},
		},
		{
			name: "environment metrics",
			msg: &generated.Telemetry{
				Variant: &generated.Telemetry_EnvironmentMetrics{
					EnvironmentMetrics: &generated.EnvironmentMetrics{
						Temperature:        &zeroFloat,
						RelativeHumidity:   &zeroFloat,
						BarometricPressure: &zeroFloat,
						Iaq:                &zeroUint,
					},
				},
			},
			assert: func(t *testing.T, telemetry *TelemetryPayload) {
				if telemetry.Environment.TemperatureC == nil || *telemetry.Environment.TemperatureC != 0 {
					t.Fatalf("expected temperature pointer with zero value, got %#v", telemetry.Environment.TemperatureC)
				}
				if telemetry.Environment.Humidity == nil || *telemetry.Environment.Humidity != 0 {
					t.Fatalf("expected humidity pointer with zero value, got %#v", telemetry.Environment.Humidity)
				}
				if telemetry.Environment.PressureHpa == nil || *telemetry.Environment.PressureHpa != 0 {
					t.Fatalf("expected pressure pointer with zero value, got %#v", telemetry.Environment.PressureHpa)
				}
				if telemetry.AirQuality.IAQ == nil || *telemetry.AirQuality.IAQ != 0 {
					t.Fatalf("expected IAQ pointer with zero value, got %#v", telemetry.AirQuality.IAQ)
				}
			},
		},
		{
			name: "air quality metrics",
			msg: &generated.Telemetry{
				Variant: &generated.Telemetry_AirQualityMetrics{
					AirQualityMetrics: &generated.AirQualityMetrics{
						Pm25Standard: &zeroUint,
						Pm10Standard: &zeroUint,
						Co2:          &zeroUint,
					},
				},
			},
			assert: func(t *testing.T, telemetry *TelemetryPayload) {
				if telemetry.AirQuality.PM25 == nil || *telemetry.AirQuality.PM25 != 0 {
					t.Fatalf("expected PM2.5 pointer with zero value, got %#v", telemetry.AirQuality.PM25)
				}
				if telemetry.AirQuality.PM10 == nil || *telemetry.AirQuality.PM10 != 0 {
					t.Fatalf("expected PM10 pointer with zero value, got %#v", telemetry.AirQuality.PM10)
				}
				if telemetry.AirQuality.CO2 == nil || *telemetry.AirQuality.CO2 != 0 {
					t.Fatalf("expected CO2 pointer with zero value, got %#v", telemetry.AirQuality.CO2)
				}
			},
		},
		{
			name: "power metrics",
			msg: &generated.Telemetry{
				Variant: &generated.Telemetry_PowerMetrics{
					PowerMetrics: &generated.PowerMetrics{
						Ch1Voltage: &zeroFloat,
					},
				},
			},
			assert: func(t *testing.T, telemetry *TelemetryPayload) {
				if telemetry.Power.Voltage == nil || *telemetry.Power.Voltage != 0 {
					t.Fatalf("expected voltage pointer with zero value, got %#v", telemetry.Power.Voltage)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := proto.Marshal(tc.msg)
			if err != nil {
				t.Fatal(err)
			}

			telemetry, err := decodeTelemetryPayload(payload)
			if err != nil {
				t.Fatal(err)
			}
			tc.assert(t, telemetry)
		})
	}
}

func TestDecodeRoutingPayloadVariants(t *testing.T) {
	requestPayload, err := proto.Marshal(&generated.Routing{
		Variant: &generated.Routing_RouteRequest{
			RouteRequest: &generated.RouteDiscovery{
				Route:     []uint32{1, 2},
				RouteBack: []uint32{2, 1},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	request, err := decodeRoutingPayload(requestPayload)
	if err != nil {
		t.Fatal(err)
	}
	if request.Variant != "route_request" || request.HopsTowards != 2 || request.HopsBack != 2 {
		t.Fatalf("unexpected route request payload: %#v", request)
	}

	errorPayload, err := proto.Marshal(&generated.Routing{
		Variant: &generated.Routing_ErrorReason{
			ErrorReason: generated.Routing_NO_ROUTE,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	reply, err := decodeRoutingPayload(errorPayload)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Variant != "error" || reply.ErrorReason != generated.Routing_NO_ROUTE.String() {
		t.Fatalf("unexpected route error payload: %#v", reply)
	}
}
