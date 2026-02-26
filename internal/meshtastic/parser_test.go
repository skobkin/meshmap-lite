package meshtastic

import (
	cryptoaes "crypto/aes"
	cryptocipher "crypto/cipher"
	"encoding/binary"
	"testing"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

func TestParseServiceEnvelopeChat(t *testing.T) {
	ConfigureChannelKeys(nil)
	packet := &generated.MeshPacket{
		From: 0x9028d008,
		Id:   42,
		PayloadVariant: &generated.MeshPacket_Decoded{Decoded: &generated.Data{
			Portnum: generated.PortNum_TEXT_MESSAGE_APP,
			Payload: []byte("hello"),
		}},
	}
	env := &generated.ServiceEnvelope{Packet: packet, ChannelId: "LongFast", GatewayId: "gw"}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	evt, err := ParsePayload(TopicKindChannel, payload, "LongFast", "")
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedChat {
		t.Fatalf("expected chat, got %s", evt.Kind)
	}
	if evt.NodeID != "!9028d008" {
		t.Fatalf("unexpected node id: %q", evt.NodeID)
	}
	if evt.Chat == nil || evt.Chat.Text != "hello" {
		t.Fatalf("unexpected chat payload")
	}
}

func TestParseMapReport(t *testing.T) {
	ConfigureChannelKeys(nil)
	report := &generated.MapReport{LongName: "Node", ShortName: "ND", LatitudeI: 645000000, LongitudeI: 406000000}
	payload, err := proto.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	evt, err := ParsePayload(TopicKindMapReport, payload, "", "!1234abcd")
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

func TestParseMapReportServiceEnvelope(t *testing.T) {
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

	evt, err := ParsePayload(TopicKindMapReport, payload, "", "")
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

func TestParseServiceEnvelopeEncryptedLongFast(t *testing.T) {
	ConfigureChannelKeys(map[string]string{"longfast": "AQ=="})

	data := &generated.Data{
		Portnum: generated.PortNum_TEXT_MESSAGE_APP,
		Payload: []byte("secret hello"),
	}
	plain, err := proto.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	from := uint32(0x9028d008)
	id := uint32(77)
	key := append([]byte(nil), defaultPSK...)
	ciphertext := encryptCTR(plain, from, id, key)

	packet := &generated.MeshPacket{
		From: from,
		Id:   id,
		// "LongFast" with default psk (#1).
		Channel: uint32(channelHash("LongFast", key)),
		PayloadVariant: &generated.MeshPacket_Encrypted{
			Encrypted: ciphertext,
		},
	}
	env := &generated.ServiceEnvelope{
		Packet:    packet,
		ChannelId: "LongFast",
		GatewayId: "gw",
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	evt, err := ParsePayload(TopicKindChannel, payload, "LongFast", "")
	if err != nil {
		t.Fatalf("expected decrypt success, got error: %v", err)
	}
	if evt.Kind != ParsedChat {
		t.Fatalf("expected chat, got %s", evt.Kind)
	}
	if evt.NodeID != "!9028d008" {
		t.Fatalf("unexpected node id: %q", evt.NodeID)
	}
	if evt.Chat == nil || evt.Chat.Text != "secret hello" {
		t.Fatalf("unexpected chat payload after decryption")
	}
}

func encryptCTR(plaintext []byte, from, packetID uint32, key []byte) []byte {
	block, err := cryptoaes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	iv := make([]byte, 16)
	binary.LittleEndian.PutUint64(iv[0:8], uint64(packetID))
	binary.LittleEndian.PutUint32(iv[8:12], from)
	out := make([]byte, len(plaintext))
	cryptocipher.NewCTR(block, iv).XORKeyStream(out, plaintext)

	return out
}
