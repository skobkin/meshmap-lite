package meshtastic

import (
	"testing"
	"time"

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

func TestParsePayloadRealWorldPositionSamples(t *testing.T) {
	for _, sample := range loadRealWorldChannelSamples(t, "real_world_position_samples.json") {
		t.Run(sample.Name, func(t *testing.T) {
			ConfigureChannelKeys(map[string]string{"LongFast": sample.ChannelPSK})

			info := ClassifyTopic("msh/RU/ARKH", "2/map", sample.Topic)
			if info.Kind != TopicKindChannel {
				t.Fatalf("expected channel topic kind, got %q", info.Kind)
			}
			if info.Channel != "LongFast" {
				t.Fatalf("unexpected channel: %q", info.Channel)
			}
			if info.GatewayID != sample.ExpectedGatewayID {
				t.Fatalf("unexpected gateway id: got %q want %q", info.GatewayID, sample.ExpectedGatewayID)
			}

			evt, err := ParsePayload(info.Kind, mustDecodeFixtureHex(t, sample.PayloadHex), info.Channel, info.MapNodeID)
			if err != nil {
				t.Fatalf("parse payload: %v", err)
			}
			if string(evt.Kind) != sample.ExpectedKind {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, sample.ExpectedKind)
			}
			if evt.NodeID != sample.ExpectedNodeID {
				t.Fatalf("unexpected node id: got %q want %q", evt.NodeID, sample.ExpectedNodeID)
			}
			if evt.PacketID != sample.ExpectedPacketID {
				t.Fatalf("unexpected packet id: got %d want %d", evt.PacketID, sample.ExpectedPacketID)
			}
			if !evt.Encrypted || !evt.Decrypted {
				t.Fatalf("expected decrypted encrypted event, got encrypted=%v decrypted=%v", evt.Encrypted, evt.Decrypted)
			}
			if !evt.Timestamp.Equal(mustParseFixtureTimestamp(t, sample.ExpectedTimestamp)) {
				t.Fatalf("unexpected timestamp: got %s want %s", evt.Timestamp.UTC().Format(time.RFC3339), sample.ExpectedTimestamp)
			}
			if evt.Position == nil {
				t.Fatalf("missing position payload")
			}
			if evt.Position.Latitude != sample.ExpectedLatitude {
				t.Fatalf("unexpected latitude: got %.7f want %.7f", evt.Position.Latitude, sample.ExpectedLatitude)
			}
			if evt.Position.Longitude != sample.ExpectedLongitude {
				t.Fatalf("unexpected longitude: got %.7f want %.7f", evt.Position.Longitude, sample.ExpectedLongitude)
			}
			if evt.Position.AltitudeM == nil || *evt.Position.AltitudeM != sample.ExpectedAltitudeM {
				t.Fatalf("unexpected altitude: got %#v want %.1f", evt.Position.AltitudeM, sample.ExpectedAltitudeM)
			}
		})
	}
}

func TestParsePayloadRealWorldChatSamples(t *testing.T) {
	for _, sample := range loadRealWorldChatSamples(t, "real_world_chat_samples.json") {
		t.Run(sample.Name, func(t *testing.T) {
			ConfigureChannelKeys(map[string]string{"LongFast": sample.ChannelPSK})

			info := ClassifyTopic("msh/RU/ARKH", "2/map", sample.Topic)
			if info.Kind != TopicKindChannel {
				t.Fatalf("expected channel topic kind, got %q", info.Kind)
			}
			if info.GatewayID != sample.ExpectedGatewayID {
				t.Fatalf("unexpected gateway id: got %q want %q", info.GatewayID, sample.ExpectedGatewayID)
			}

			evt, err := ParsePayload(info.Kind, mustDecodeFixtureHex(t, sample.PayloadHex), info.Channel, info.MapNodeID)
			if err != nil {
				t.Fatalf("parse payload: %v", err)
			}
			if evt.Kind != ParsedChat {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, ParsedChat)
			}
			if evt.NodeID != sample.ExpectedNodeID {
				t.Fatalf("unexpected node id: got %q want %q", evt.NodeID, sample.ExpectedNodeID)
			}
			if evt.PacketID != sample.ExpectedPacketID {
				t.Fatalf("unexpected packet id: got %d want %d", evt.PacketID, sample.ExpectedPacketID)
			}
			if !evt.Encrypted || !evt.Decrypted {
				t.Fatalf("expected decrypted encrypted event, got encrypted=%v decrypted=%v", evt.Encrypted, evt.Decrypted)
			}
			if !evt.Timestamp.Equal(mustParseFixtureTimestamp(t, sample.ExpectedTimestamp)) {
				t.Fatalf("unexpected timestamp: got %s want %s", evt.Timestamp.UTC().Format(time.RFC3339), sample.ExpectedTimestamp)
			}
			if evt.Chat == nil {
				t.Fatalf("missing chat payload")
			}
			if evt.Chat.Text != sample.ExpectedText {
				t.Fatalf("unexpected chat text: got %q want %q", evt.Chat.Text, sample.ExpectedText)
			}
		})
	}
}

func TestParsePayloadRealWorldNodeInfoSamples(t *testing.T) {
	for _, sample := range loadRealWorldNodeInfoSamples(t, "real_world_nodeinfo_samples.json") {
		t.Run(sample.Name, func(t *testing.T) {
			ConfigureChannelKeys(map[string]string{"LongFast": sample.ChannelPSK})

			info := ClassifyTopic("msh/RU/ARKH", "2/map", sample.Topic)
			if info.Kind != TopicKindChannel {
				t.Fatalf("expected channel topic kind, got %q", info.Kind)
			}
			if info.GatewayID != sample.ExpectedGatewayID {
				t.Fatalf("unexpected gateway id: got %q want %q", info.GatewayID, sample.ExpectedGatewayID)
			}

			evt, err := ParsePayload(info.Kind, mustDecodeFixtureHex(t, sample.PayloadHex), info.Channel, info.MapNodeID)
			if err != nil {
				t.Fatalf("parse payload: %v", err)
			}
			if evt.Kind != ParsedNodeInfo {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, ParsedNodeInfo)
			}
			if evt.NodeID != sample.ExpectedNodeID {
				t.Fatalf("unexpected node id: got %q want %q", evt.NodeID, sample.ExpectedNodeID)
			}
			if evt.PacketID != sample.ExpectedPacketID {
				t.Fatalf("unexpected packet id: got %d want %d", evt.PacketID, sample.ExpectedPacketID)
			}
			if !evt.Encrypted || !evt.Decrypted {
				t.Fatalf("expected decrypted encrypted event, got encrypted=%v decrypted=%v", evt.Encrypted, evt.Decrypted)
			}
			if !evt.Timestamp.Equal(mustParseFixtureTimestamp(t, sample.ExpectedTimestamp)) {
				t.Fatalf("unexpected timestamp: got %s want %s", evt.Timestamp.UTC().Format(time.RFC3339), sample.ExpectedTimestamp)
			}
			if evt.NodeInfo == nil {
				t.Fatalf("missing node info payload")
			}
			if evt.NodeInfo.LongName != sample.ExpectedLongName {
				t.Fatalf("unexpected long name: got %q want %q", evt.NodeInfo.LongName, sample.ExpectedLongName)
			}
			if evt.NodeInfo.ShortName != sample.ExpectedShortName {
				t.Fatalf("unexpected short name: got %q want %q", evt.NodeInfo.ShortName, sample.ExpectedShortName)
			}
			if evt.NodeInfo.Role != sample.ExpectedRole {
				t.Fatalf("unexpected role: got %q want %q", evt.NodeInfo.Role, sample.ExpectedRole)
			}
			if evt.NodeInfo.BoardModel != sample.ExpectedBoardModel {
				t.Fatalf("unexpected board model: got %q want %q", evt.NodeInfo.BoardModel, sample.ExpectedBoardModel)
			}
			if evt.NodeInfo.FirmwareVersion != sample.ExpectedFirmware {
				t.Fatalf("unexpected firmware version: got %q want %q", evt.NodeInfo.FirmwareVersion, sample.ExpectedFirmware)
			}
			if evt.NodeInfo.LoRaRegion != sample.ExpectedLoRaRegion {
				t.Fatalf("unexpected LoRa region: got %q want %q", evt.NodeInfo.LoRaRegion, sample.ExpectedLoRaRegion)
			}
			if evt.NodeInfo.ModemPreset != sample.ExpectedModemPreset {
				t.Fatalf("unexpected modem preset: got %q want %q", evt.NodeInfo.ModemPreset, sample.ExpectedModemPreset)
			}
		})
	}
}

func TestParsePayloadRealWorldTelemetrySamples(t *testing.T) {
	for _, sample := range loadRealWorldTelemetrySamples(t, "real_world_telemetry_samples.json") {
		t.Run(sample.Name, func(t *testing.T) {
			ConfigureChannelKeys(map[string]string{"LongFast": sample.ChannelPSK})

			info := ClassifyTopic("msh/RU/ARKH", "2/map", sample.Topic)
			if info.Kind != TopicKindChannel {
				t.Fatalf("expected channel topic kind, got %q", info.Kind)
			}
			if info.GatewayID != sample.ExpectedGatewayID {
				t.Fatalf("unexpected gateway id: got %q want %q", info.GatewayID, sample.ExpectedGatewayID)
			}

			evt, err := ParsePayload(info.Kind, mustDecodeFixtureHex(t, sample.PayloadHex), info.Channel, info.MapNodeID)
			if err != nil {
				t.Fatalf("parse payload: %v", err)
			}
			if evt.Kind != ParsedTelemetry {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, ParsedTelemetry)
			}
			if evt.NodeID != sample.ExpectedNodeID {
				t.Fatalf("unexpected node id: got %q want %q", evt.NodeID, sample.ExpectedNodeID)
			}
			if evt.PacketID != sample.ExpectedPacketID {
				t.Fatalf("unexpected packet id: got %d want %d", evt.PacketID, sample.ExpectedPacketID)
			}
			if !evt.Encrypted || !evt.Decrypted {
				t.Fatalf("expected decrypted encrypted event, got encrypted=%v decrypted=%v", evt.Encrypted, evt.Decrypted)
			}
			if !evt.Timestamp.Equal(mustParseFixtureTimestamp(t, sample.ExpectedTimestamp)) {
				t.Fatalf("unexpected timestamp: got %s want %s", evt.Timestamp.UTC().Format(time.RFC3339), sample.ExpectedTimestamp)
			}
			if evt.Telemetry == nil {
				t.Fatalf("missing telemetry payload")
			}
			if evt.Telemetry.Power.Voltage == nil || *evt.Telemetry.Power.Voltage != sample.ExpectedVoltage {
				t.Fatalf("unexpected voltage: got %#v want %v", evt.Telemetry.Power.Voltage, sample.ExpectedVoltage)
			}
			if evt.Telemetry.Power.BatteryLevel == nil || *evt.Telemetry.Power.BatteryLevel != sample.ExpectedBatteryPct {
				t.Fatalf("unexpected battery level: got %#v want %v", evt.Telemetry.Power.BatteryLevel, sample.ExpectedBatteryPct)
			}
		})
	}
}
