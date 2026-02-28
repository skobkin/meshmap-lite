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
