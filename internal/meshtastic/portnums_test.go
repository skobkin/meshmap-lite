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

	request, err := decodeRoutingPayload(&generated.MeshPacket{From: 0x11111111, To: 0x22222222}, &generated.Data{
		Portnum:   generated.PortNum_ROUTING_APP,
		Payload:   requestPayload,
		RequestId: 123,
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Variant != "route_request" || request.RequestID != 123 || len(request.Route) != 2 || len(request.RouteBack) != 2 {
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

	reply, err := decodeRoutingPayload(&generated.MeshPacket{From: 0x11111111, To: 0x22222222}, &generated.Data{
		Portnum:   generated.PortNum_ROUTING_APP,
		Payload:   errorPayload,
		RequestId: 456,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.Variant != "error" || reply.ErrorReason != generated.Routing_NO_ROUTE.String() || reply.RequestID != 456 || !reply.TracerouteRef {
		t.Fatalf("unexpected route error payload: %#v", reply)
	}
}

func TestDecodeTraceroutePayloadClassifiesRequest(t *testing.T) {
	payload, err := proto.Marshal(&generated.RouteDiscovery{})
	if err != nil {
		t.Fatal(err)
	}

	traceroute, err := decodeTraceroutePayload(&generated.MeshPacket{
		From:     0x9028d008,
		To:       0xa55e5e56,
		Id:       321,
		HopStart: 7,
		HopLimit: 7,
	}, &generated.Data{
		Portnum:      generated.PortNum_TRACEROUTE_APP,
		Payload:      payload,
		WantResponse: true,
		Bitfield:     proto.Uint32(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if traceroute.Role != "request" || traceroute.Status != "requested" {
		t.Fatalf("unexpected traceroute request semantics: %#v", traceroute)
	}
	if traceroute.RequestID != 321 {
		t.Fatalf("unexpected request id: got %d want 321", traceroute.RequestID)
	}
	if len(traceroute.ForwardPath) != 0 || len(traceroute.ReturnPath) != 0 {
		t.Fatalf("request packet must not produce result paths: %#v", traceroute)
	}
}

func TestDecodeTraceroutePayloadReconstructsDirectReply(t *testing.T) {
	payload, err := proto.Marshal(&generated.RouteDiscovery{
		SnrTowards: []int32{22},
	})
	if err != nil {
		t.Fatal(err)
	}

	traceroute, err := decodeTraceroutePayload(&generated.MeshPacket{
		From:     0x9028d008,
		To:       0xa55e5e56,
		Id:       654,
		HopStart: 7,
		HopLimit: 7,
	}, &generated.Data{
		Portnum:      generated.PortNum_TRACEROUTE_APP,
		Payload:      payload,
		RequestId:    321,
		WantResponse: false,
		Bitfield:     proto.Uint32(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if traceroute.Role != "reply" || traceroute.Status != "completed" {
		t.Fatalf("unexpected traceroute reply semantics: %#v", traceroute)
	}
	if want := []string{"!a55e5e56", "!9028d008"}; len(traceroute.ForwardPath) != len(want) || traceroute.ForwardPath[0] != want[0] || traceroute.ForwardPath[1] != want[1] {
		t.Fatalf("unexpected forward path: got %#v want %#v", traceroute.ForwardPath, want)
	}
	if !traceroute.InferredForwardPath || !traceroute.InferredDirect {
		t.Fatalf("expected inferred direct path markers, got %#v", traceroute)
	}
	if len(traceroute.ReturnPath) != 0 {
		t.Fatalf("did not expect return path without evidence, got %#v", traceroute.ReturnPath)
	}
}

func TestDecodeTraceroutePayloadKeepsReturnPathConditional(t *testing.T) {
	payload, err := proto.Marshal(&generated.RouteDiscovery{
		RouteBack: []uint32{0x01020304},
		SnrBack:   []int32{12},
	})
	if err != nil {
		t.Fatal(err)
	}

	withoutEvidence, err := decodeTraceroutePayload(&generated.MeshPacket{
		From: 0x9028d008,
		To:   0xa55e5e56,
	}, &generated.Data{
		Portnum:   generated.PortNum_TRACEROUTE_APP,
		Payload:   payload,
		RequestId: 321,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"!01020304"}; len(withoutEvidence.ReturnPath) != len(want) || withoutEvidence.ReturnPath[0] != want[0] {
		t.Fatalf("unexpected raw return path: got %#v want %#v", withoutEvidence.ReturnPath, want)
	}
	if withoutEvidence.InferredReturnPath {
		t.Fatalf("did not expect inferred return path without packet evidence")
	}

	withEvidence, err := decodeTraceroutePayload(&generated.MeshPacket{
		From:     0x9028d008,
		To:       0xa55e5e56,
		HopStart: 1,
	}, &generated.Data{
		Portnum:   generated.PortNum_TRACEROUTE_APP,
		Payload:   payload,
		RequestId: 321,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"!9028d008", "!01020304", "!a55e5e56"}; len(withEvidence.ReturnPath) != len(want) || withEvidence.ReturnPath[0] != want[0] || withEvidence.ReturnPath[2] != want[2] {
		t.Fatalf("unexpected reconstructed return path: got %#v want %#v", withEvidence.ReturnPath, want)
	}
	if !withEvidence.InferredReturnPath {
		t.Fatalf("expected inferred return path marker")
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

func TestParsePayloadRealWorldTracerouteSamples(t *testing.T) {
	for _, sample := range loadRealWorldTracerouteSamples(t, "real_world_traceroute_samples.json") {
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
			if evt.Kind != ParsedTraceroute {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, ParsedTraceroute)
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
			if evt.Traceroute == nil {
				t.Fatalf("missing traceroute payload")
			}
			if evt.Traceroute.Role != sample.ExpectedRole {
				t.Fatalf("unexpected traceroute role: got %q want %q", evt.Traceroute.Role, sample.ExpectedRole)
			}
			if evt.Traceroute.Status != sample.ExpectedStatus {
				t.Fatalf("unexpected traceroute status: got %q want %q", evt.Traceroute.Status, sample.ExpectedStatus)
			}
			if evt.Traceroute.RequestID != sample.ExpectedRequestID {
				t.Fatalf("unexpected traceroute request id: got %d want %d", evt.Traceroute.RequestID, sample.ExpectedRequestID)
			}
			if len(evt.Traceroute.ForwardPath) != len(sample.ExpectedForwardPath) {
				t.Fatalf("unexpected forward path length: got %#v want %#v", evt.Traceroute.ForwardPath, sample.ExpectedForwardPath)
			}
			for i := range sample.ExpectedForwardPath {
				if evt.Traceroute.ForwardPath[i] != sample.ExpectedForwardPath[i] {
					t.Fatalf("unexpected forward path: got %#v want %#v", evt.Traceroute.ForwardPath, sample.ExpectedForwardPath)
				}
			}
			if len(evt.Traceroute.ReturnPath) != len(sample.ExpectedReturnPath) {
				t.Fatalf("unexpected return path length: got %#v want %#v", evt.Traceroute.ReturnPath, sample.ExpectedReturnPath)
			}
			for i := range sample.ExpectedReturnPath {
				if evt.Traceroute.ReturnPath[i] != sample.ExpectedReturnPath[i] {
					t.Fatalf("unexpected return path: got %#v want %#v", evt.Traceroute.ReturnPath, sample.ExpectedReturnPath)
				}
			}
			if evt.Traceroute.InferredDirect != sample.ExpectedInferredDirect {
				t.Fatalf("unexpected inferred direct flag: got %v want %v", evt.Traceroute.InferredDirect, sample.ExpectedInferredDirect)
			}
		})
	}
}

func TestParsePayloadRealWorldRoutingSamples(t *testing.T) {
	for _, sample := range loadRealWorldRoutingSamples(t, "real_world_routing_samples.json") {
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
			if evt.Kind != ParsedRouting {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, ParsedRouting)
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
			if evt.Routing == nil {
				t.Fatalf("missing routing payload")
			}
			if evt.Routing.Variant != sample.ExpectedVariant {
				t.Fatalf("unexpected routing variant: got %q want %q", evt.Routing.Variant, sample.ExpectedVariant)
			}
			if evt.Routing.ErrorReason != sample.ExpectedErrorReason {
				t.Fatalf("unexpected routing error reason: got %q want %q", evt.Routing.ErrorReason, sample.ExpectedErrorReason)
			}
			if evt.Routing.RequestID != sample.ExpectedRequestID {
				t.Fatalf("unexpected routing request id: got %d want %d", evt.Routing.RequestID, sample.ExpectedRequestID)
			}
			if evt.Routing.TracerouteRef != sample.ExpectedTracerouteRef {
				t.Fatalf("unexpected routing traceroute ref flag: got %v want %v", evt.Routing.TracerouteRef, sample.ExpectedTracerouteRef)
			}
		})
	}
}
