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
		Route:      []uint32{0x11111111, 0x22222222},
		SnrTowards: []int32{1, 2},
		RouteBack:  []uint32{0x22222222, 0x11111111},
		SnrBack:    []int32{3, 4},
	})
	if err != nil {
		t.Fatal(err)
	}

	packet := &generated.MeshPacket{
		From: 0x9028d008,
		Id:   900,
		PayloadVariant: &generated.MeshPacket_Decoded{Decoded: &generated.Data{
			Portnum: generated.PortNum_TRACEROUTE_APP,
			Payload: routePayload,
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
	if evt.Traceroute == nil || evt.Traceroute.HopsTowards != 2 || evt.Traceroute.HopsBack != 2 {
		t.Fatalf("unexpected traceroute payload: %#v", evt.Traceroute)
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
