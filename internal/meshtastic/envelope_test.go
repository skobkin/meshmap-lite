package meshtastic

import (
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

	evt, err := parseServiceEnvelope(payload, "LongFast")
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

func TestParseServiceEnvelopeTraceroute(t *testing.T) {
	ConfigureChannelKeys(nil)

	routePayload, err := proto.Marshal(&generated.RouteDiscovery{
		Route:      []uint32{0x11111111},
		SnrTowards: []int32{1, 2},
		RouteBack:  []uint32{0x22222222},
		SnrBack:    []int32{3},
	})
	if err != nil {
		t.Fatal(err)
	}

	packet := &generated.MeshPacket{
		From:     0x9028d008,
		To:       0xa55e5e56,
		Id:       900,
		HopStart: 1,
		PayloadVariant: &generated.MeshPacket_Decoded{Decoded: &generated.Data{
			Portnum:      generated.PortNum_TRACEROUTE_APP,
			Payload:      routePayload,
			RequestId:    777,
			WantResponse: false,
			Bitfield:     proto.Uint32(1),
		}},
	}
	env := &generated.ServiceEnvelope{Packet: packet, ChannelId: "LongFast", GatewayId: "gw"}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	evt, err := parseServiceEnvelope(payload, "LongFast")
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedTraceroute {
		t.Fatalf("expected traceroute, got %s", evt.Kind)
	}
	if evt.Traceroute == nil {
		t.Fatalf("unexpected traceroute payload: %#v", evt.Traceroute)
	}
	if evt.Traceroute.Role != "reply" || evt.Traceroute.RequestID != 777 {
		t.Fatalf("unexpected traceroute semantics: %#v", evt.Traceroute)
	}
	if len(evt.Traceroute.ForwardPath) != 3 || evt.Traceroute.ForwardPath[0] != "!a55e5e56" || evt.Traceroute.ForwardPath[2] != "!9028d008" {
		t.Fatalf("unexpected forward path: %#v", evt.Traceroute.ForwardPath)
	}
	if len(evt.Traceroute.ReturnPath) != 3 || evt.Traceroute.ReturnPath[0] != "!9028d008" || evt.Traceroute.ReturnPath[2] != "!a55e5e56" {
		t.Fatalf("unexpected return path: %#v", evt.Traceroute.ReturnPath)
	}
}

func TestParseServiceEnvelopeUnknownEncryptedWhenNoKey(t *testing.T) {
	ConfigureChannelKeys(nil)

	packet := &generated.MeshPacket{
		From:    0x9028d008,
		Id:      123,
		Channel: 7,
		PayloadVariant: &generated.MeshPacket_Encrypted{
			Encrypted: []byte{0xde, 0xad, 0xbe, 0xef},
		},
	}
	env := &generated.ServiceEnvelope{Packet: packet, ChannelId: "LongFast", GatewayId: "gw"}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	evt, err := parseServiceEnvelope(payload, "LongFast")
	if err != nil {
		t.Fatalf("expected opaque event, got err: %v", err)
	}
	if evt.Kind != ParsedUnknownEncrypted {
		t.Fatalf("expected unknown encrypted kind, got %s", evt.Kind)
	}
	if !evt.Encrypted {
		t.Fatalf("expected encrypted flag")
	}
}

func TestParseServiceEnvelopePKIBecomesOpaquePKIEvent(t *testing.T) {
	ConfigureChannelKeys(nil)

	packet := &generated.MeshPacket{
		From:         0xa55e5e56,
		To:           0x698509f8,
		Id:           3350416627,
		HopStart:     7,
		HopLimit:     7,
		PkiEncrypted: true,
		PayloadVariant: &generated.MeshPacket_Encrypted{
			Encrypted: []byte{0xde, 0xad, 0xbe, 0xef, 0xca},
		},
	}
	env := &generated.ServiceEnvelope{Packet: packet, ChannelId: "PKI", GatewayId: "!9028d008"}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	evt, err := parseServiceEnvelope(payload, "PKI")
	if err != nil {
		t.Fatalf("expected opaque PKI event, got err: %v", err)
	}
	if evt.Kind != ParsedPKI {
		t.Fatalf("expected PKI kind, got %s", evt.Kind)
	}
	if evt.NodeID != "!a55e5e56" {
		t.Fatalf("unexpected sender node id: %q", evt.NodeID)
	}
	if evt.PacketID != 3350416627 {
		t.Fatalf("unexpected packet id: %d", evt.PacketID)
	}
	if !evt.Encrypted || evt.Decrypted {
		t.Fatalf("unexpected encryption flags: encrypted=%v decrypted=%v", evt.Encrypted, evt.Decrypted)
	}
	if evt.PKI == nil {
		t.Fatalf("expected PKI payload details")
	}
	if evt.PKI.DestinationNodeID != "!698509f8" {
		t.Fatalf("unexpected destination: %#v", evt.PKI)
	}
	if evt.PKI.GatewayID != "!9028d008" || evt.PKI.TopicChannel != "PKI" {
		t.Fatalf("unexpected PKI routing metadata: %#v", evt.PKI)
	}
	if !evt.PKI.PKIEncrypted || evt.PKI.PayloadSizeBytes != 5 {
		t.Fatalf("unexpected PKI payload markers: %#v", evt.PKI)
	}
}
