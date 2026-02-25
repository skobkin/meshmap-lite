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
