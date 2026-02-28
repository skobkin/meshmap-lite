package meshtastic

import (
	"fmt"
	"time"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

type decodedEnvelope struct {
	packet    *generated.MeshPacket
	decoded   *generated.Data
	encrypted bool
	decrypted bool
}

func parseServiceEnvelope(payload []byte, channelHint string) (ParsedEvent, error) {
	envelope, opaque, err := decodeEnvelopePayload(payload, channelHint, 0)
	if err != nil {
		return ParsedEvent{}, err
	}
	if opaque != nil {
		return *opaque, nil
	}

	return parseDecodedPacket(envelope)
}

func decodeEnvelopePayload(payload []byte, channelHint string, opaquePortnum generated.PortNum) (decodedEnvelope, *ParsedEvent, error) {
	var env generated.ServiceEnvelope
	if err := proto.Unmarshal(payload, &env); err != nil {
		return decodedEnvelope{}, nil, fmt.Errorf("decode service envelope: %w", err)
	}

	packet := env.GetPacket()
	if packet == nil {
		return decodedEnvelope{}, nil, fmt.Errorf("empty packet")
	}

	decoded := packet.GetDecoded()
	encrypted := decoded == nil
	wasDecrypted := false
	if decoded == nil {
		decryptedData, ok := decryptPacketIfPossible(packet, env.GetChannelId(), channelHint)
		if !ok {
			event := newUnknownEncryptedEvent(packet, opaquePortnum)

			return decodedEnvelope{}, &event, nil
		}
		decoded = decryptedData
		wasDecrypted = true
	}

	return decodedEnvelope{
		packet:    packet,
		decoded:   decoded,
		encrypted: encrypted,
		decrypted: wasDecrypted,
	}, nil, nil
}

func packetTimestamp(packet *generated.MeshPacket) *time.Time {
	if rx := packet.GetRxTime(); rx > 0 {
		ts := time.Unix(int64(rx), 0).UTC()

		return &ts
	}

	return nil
}

func newUnknownEncryptedEvent(packet *generated.MeshPacket, portnum generated.PortNum) ParsedEvent {
	return ParsedEvent{
		Kind:      ParsedUnknownEncrypted,
		NodeID:    nodeIDFromNum(packet.GetFrom()),
		PacketID:  packet.GetId(),
		Portnum:   portnum,
		Format:    "protobuf",
		Encrypted: true,
		Decrypted: false,
	}
}
