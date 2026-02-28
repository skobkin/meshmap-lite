package meshtastic

import (
	"testing"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

func TestParseMapReportProtobuf(t *testing.T) {
	ConfigureChannelKeys(nil)

	report := &generated.MapReport{
		LongName:   "Node",
		ShortName:  "ND",
		LatitudeI:  645000000,
		LongitudeI: 406000000,
	}
	payload, err := proto.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	evt, err := parseMapReportProtobuf(payload, "!1234abcd")
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedMapReport {
		t.Fatalf("expected map_report, got %s", evt.Kind)
	}
	if evt.MapReport == nil || evt.MapReport.NodeID != "!1234abcd" {
		t.Fatalf("unexpected map report node id")
	}
}

func TestParseMapReportEnvelope(t *testing.T) {
	ConfigureChannelKeys(nil)

	report := &generated.MapReport{
		LongName:               "Node Name",
		ShortName:              "ND",
		FirmwareVersion:        "2.7.18.fb3bf780",
		Region:                 generated.Config_LoRaConfig_EU_868,
		ModemPreset:            generated.Config_LoRaConfig_LONG_FAST,
		HasDefaultChannel:      true,
		LatitudeI:              645000000,
		LongitudeI:             406000000,
		Altitude:               42,
		PositionPrecision:      11,
		NumOnlineLocalNodes:    7,
		HasOptedReportLocation: true,
	}
	mapPayload, err := proto.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	data := &generated.Data{
		Portnum: generated.PortNum_MAP_REPORT_APP,
		Payload: mapPayload,
	}
	packet := &generated.MeshPacket{
		From: 0x9028d008,
		Id:   321,
		PayloadVariant: &generated.MeshPacket_Decoded{
			Decoded: data,
		},
	}
	env := &generated.ServiceEnvelope{Packet: packet, ChannelId: "LongFast", GatewayId: "!gw"}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	evt, err := parseMapReportEnvelope(payload, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedMapReport {
		t.Fatalf("expected map_report, got %s", evt.Kind)
	}
	if evt.NodeID != "!9028d008" {
		t.Fatalf("unexpected node id: %q", evt.NodeID)
	}
	if evt.PacketID != 321 {
		t.Fatalf("unexpected packet id: %d", evt.PacketID)
	}
	if evt.MapReport == nil {
		t.Fatalf("missing map report payload")
	}
	if evt.MapReport.FirmwareVersion != "2.7.18.fb3bf780" {
		t.Fatalf("unexpected firmware version: %q", evt.MapReport.FirmwareVersion)
	}
	if evt.MapReport.LoRaRegion != "EU_868" {
		t.Fatalf("unexpected region: %q", evt.MapReport.LoRaRegion)
	}
	if evt.MapReport.ModemPreset != "LONG_FAST" {
		t.Fatalf("unexpected modem preset: %q", evt.MapReport.ModemPreset)
	}
	if evt.MapReport.PositionPrecision == nil || *evt.MapReport.PositionPrecision != 11 {
		t.Fatalf("unexpected position precision: %v", evt.MapReport.PositionPrecision)
	}
	if !evt.MapReport.HasDefaultChannel || !evt.MapReport.HasOptedReportLocation {
		t.Fatalf("unexpected map-report booleans")
	}
}

func TestParsePayloadRealWorldMapReportSamples(t *testing.T) {
	for _, sample := range loadRealWorldMapReportSamples(t, "real_world_mapreport_samples.json") {
		t.Run(sample.Name, func(t *testing.T) {
			info := ClassifyTopic("msh/RU/ARKH", "2/map", sample.Topic)
			if info.Kind != TopicKindMapReport {
				t.Fatalf("expected map report topic kind, got %q", info.Kind)
			}

			evt, err := ParsePayload(info.Kind, mustDecodeFixtureHex(t, sample.PayloadHex), info.Channel, info.MapNodeID)
			if err != nil {
				t.Fatalf("parse payload: %v", err)
			}
			if evt.Kind != ParsedMapReport {
				t.Fatalf("unexpected kind: got %q want %q", evt.Kind, ParsedMapReport)
			}
			if evt.NodeID != sample.ExpectedNodeID {
				t.Fatalf("unexpected node id: got %q want %q", evt.NodeID, sample.ExpectedNodeID)
			}
			if evt.MapReport == nil {
				t.Fatalf("missing map report payload")
			}
			if evt.MapReport.LongName != sample.ExpectedLongName {
				t.Fatalf("unexpected long name: got %q want %q", evt.MapReport.LongName, sample.ExpectedLongName)
			}
			if evt.MapReport.ShortName != sample.ExpectedShortName {
				t.Fatalf("unexpected short name: got %q want %q", evt.MapReport.ShortName, sample.ExpectedShortName)
			}
			if evt.MapReport.Role != sample.ExpectedRole {
				t.Fatalf("unexpected role: got %q want %q", evt.MapReport.Role, sample.ExpectedRole)
			}
			if evt.MapReport.BoardModel != sample.ExpectedBoardModel {
				t.Fatalf("unexpected board model: got %q want %q", evt.MapReport.BoardModel, sample.ExpectedBoardModel)
			}
			if evt.MapReport.FirmwareVersion != sample.ExpectedFirmware {
				t.Fatalf("unexpected firmware version: got %q want %q", evt.MapReport.FirmwareVersion, sample.ExpectedFirmware)
			}
			if evt.MapReport.LoRaRegion != sample.ExpectedLoRaRegion {
				t.Fatalf("unexpected LoRa region: got %q want %q", evt.MapReport.LoRaRegion, sample.ExpectedLoRaRegion)
			}
			if evt.MapReport.ModemPreset != sample.ExpectedModemPreset {
				t.Fatalf("unexpected modem preset: got %q want %q", evt.MapReport.ModemPreset, sample.ExpectedModemPreset)
			}
			if evt.MapReport.HasDefaultChannel != sample.ExpectedHasDefaultChannel {
				t.Fatalf("unexpected has_default_channel: got %v want %v", evt.MapReport.HasDefaultChannel, sample.ExpectedHasDefaultChannel)
			}
			if evt.MapReport.HasOptedReportLocation != sample.ExpectedHasOptedLocation {
				t.Fatalf("unexpected has_opted_report_location: got %v want %v", evt.MapReport.HasOptedReportLocation, sample.ExpectedHasOptedLocation)
			}
			if evt.MapReport.NeighborNodesCount == nil || *evt.MapReport.NeighborNodesCount != sample.ExpectedNeighborNodesCount {
				t.Fatalf("unexpected neighbor nodes count: got %#v want %d", evt.MapReport.NeighborNodesCount, sample.ExpectedNeighborNodesCount)
			}
			if evt.MapReport.Latitude != sample.ExpectedLatitude {
				t.Fatalf("unexpected latitude: got %.7f want %.7f", evt.MapReport.Latitude, sample.ExpectedLatitude)
			}
			if evt.MapReport.Longitude != sample.ExpectedLongitude {
				t.Fatalf("unexpected longitude: got %.7f want %.7f", evt.MapReport.Longitude, sample.ExpectedLongitude)
			}
			if evt.MapReport.AltitudeM == nil || *evt.MapReport.AltitudeM != sample.ExpectedAltitudeM {
				t.Fatalf("unexpected altitude: got %#v want %.1f", evt.MapReport.AltitudeM, sample.ExpectedAltitudeM)
			}
			if evt.MapReport.PositionPrecision == nil || *evt.MapReport.PositionPrecision != sample.ExpectedPositionPrecision {
				t.Fatalf("unexpected position precision: got %#v want %d", evt.MapReport.PositionPrecision, sample.ExpectedPositionPrecision)
			}
		})
	}
}
