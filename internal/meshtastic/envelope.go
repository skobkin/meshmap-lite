package meshtastic

import (
	"fmt"
	"strings"
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
		envelopeChannelID := strings.TrimSpace(env.GetChannelId())
		if isPKITransportPacket(packet, channelHint, envelopeChannelID) {
			event := newPKIEvent(packet, strings.TrimSpace(env.GetGatewayId()), channelHint, envelopeChannelID)

			return decodedEnvelope{}, &event, nil
		}
		decryptedData, ok := decryptPacketIfPossible(packet, envelopeChannelID, channelHint)
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

func isPKITransportPacket(packet *generated.MeshPacket, topicChannel, envelopeChannelID string) bool {
	if packet.GetPkiEncrypted() {
		return true
	}
	if len(packet.GetEncrypted()) == 0 {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(topicChannel), "PKI") ||
		strings.EqualFold(strings.TrimSpace(envelopeChannelID), "PKI")
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
		HopStart:  packet.GetHopStart(),
		HopLimit:  packet.GetHopLimit(),
	}
}

func newPKIEvent(packet *generated.MeshPacket, gatewayID, topicChannel, envelopeChannelID string) ParsedEvent {
	senderNodeID := nodeIDFromNum(packet.GetFrom())

	return ParsedEvent{
		Kind:      ParsedPKI,
		NodeID:    senderNodeID,
		PacketID:  packet.GetId(),
		Portnum:   0,
		Format:    "protobuf",
		Encrypted: true,
		Decrypted: false,
		HopStart:  packet.GetHopStart(),
		HopLimit:  packet.GetHopLimit(),
		Timestamp: packetTimestamp(packet),
		PKI: &PKIPayload{
			SenderNodeID:      senderNodeID,
			DestinationNodeID: nodeIDFromNum(packet.GetTo()),
			GatewayID:         gatewayID,
			TopicChannel:      topicChannel,
			EnvelopeChannelID: envelopeChannelID,
			PacketID:          packet.GetId(),
			Encrypted:         true,
			Decrypted:         false,
			PKIEncrypted:      packet.GetPkiEncrypted(),
			PayloadSizeBytes:  len(packet.GetEncrypted()),
			HopStart:          packet.GetHopStart(),
			HopLimit:          packet.GetHopLimit(),
			Priority:          packet.GetPriority().String(),
		},
	}
}
