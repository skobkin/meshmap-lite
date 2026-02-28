package meshtastic

import (
	cryptoaes "crypto/aes"
	cryptocipher "crypto/cipher"
	"encoding/binary"
	"testing"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

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
	packetID := uint32(77)
	key := append([]byte(nil), defaultChannelKeyExpandedBytes[:]...)
	ciphertext := encryptCTR(plain, from, packetID, key)

	packet := &generated.MeshPacket{
		From:    from,
		Id:      packetID,
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

func TestDecodeAndExpandPSKDefaultChannelIndex(t *testing.T) {
	key, keyLen, ok := decodeAndExpandPSK("AQ==")
	if !ok {
		t.Fatalf("expected valid default channel psk")
	}
	if keyLen != len(defaultChannelKeyExpandedBytes) {
		t.Fatalf("unexpected key len: %d", keyLen)
	}
	for i, b := range defaultChannelKeyExpandedBytes {
		if key[i] != b {
			t.Fatalf("unexpected key byte at %d: %x", i, key[i])
		}
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
