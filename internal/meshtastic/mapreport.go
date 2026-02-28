package meshtastic

import (
	"fmt"
	"strings"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

func parseMapReportProtobuf(payload []byte, mapNodeHint string) (ParsedEvent, error) {
	var report generated.MapReport
	if err := proto.Unmarshal(payload, &report); err != nil {
		return ParsedEvent{}, fmt.Errorf("decode map report: %w", err)
	}

	out := ParsedEvent{
		Kind:    ParsedMapReport,
		NodeID:  normalizeNodeID(mapNodeHint),
		Portnum: generated.PortNum_MAP_REPORT_APP,
		Format:  "protobuf",
	}
	out.MapReport = buildMapReportPayload(&report, out.NodeID)

	return out, nil
}

func parseMapReportEnvelope(payload []byte, channelHint, mapNodeHint string) (ParsedEvent, error) {
	envelope, opaque, err := decodeEnvelopePayload(payload, channelHint, generated.PortNum_MAP_REPORT_APP)
	if err != nil {
		return ParsedEvent{}, err
	}
	if opaque != nil {
		return *opaque, nil
	}
	if len(envelope.decoded.GetPayload()) == 0 {
		return ParsedEvent{}, fmt.Errorf("missing map report payload")
	}

	nodeID := nodeIDFromNum(envelope.packet.GetFrom())
	if nodeID == "" {
		nodeID = mapNodeHint
	}

	out, err := parseMapReportProtobuf(envelope.decoded.GetPayload(), nodeID)
	if err != nil {
		return ParsedEvent{}, err
	}
	out.PacketID = envelope.packet.GetId()
	out.Portnum = generated.PortNum_MAP_REPORT_APP
	out.Encrypted = envelope.encrypted
	out.Decrypted = envelope.decrypted
	out.Timestamp = packetTimestamp(envelope.packet)

	return out, nil
}

func buildMapReportPayload(report *generated.MapReport, nodeID string) *MapReportPayload {
	out := &MapReportPayload{
		NodeID:                 normalizeNodeID(nodeID),
		LongName:               strings.TrimSpace(report.GetLongName()),
		ShortName:              strings.TrimSpace(report.GetShortName()),
		Role:                   report.GetRole().String(),
		BoardModel:             report.GetHwModel().String(),
		FirmwareVersion:        strings.TrimSpace(report.GetFirmwareVersion()),
		LoRaRegion:             report.GetRegion().String(),
		ModemPreset:            report.GetModemPreset().String(),
		HasDefaultChannel:      report.GetHasDefaultChannel(),
		HasOptedReportLocation: report.GetHasOptedReportLocation(),
		Latitude:               float64(report.GetLatitudeI()) * positionScale,
		Longitude:              float64(report.GetLongitudeI()) * positionScale,
	}
	if report.GetAltitude() != 0 {
		altitude := float64(report.GetAltitude())
		out.AltitudeM = &altitude
	}
	if report.GetPositionPrecision() > 0 {
		precision := report.GetPositionPrecision()
		out.PositionPrecision = &precision
	}
	if report.GetNumOnlineLocalNodes() > 0 {
		count := int(report.GetNumOnlineLocalNodes())
		out.NeighborNodesCount = &count
	}

	return out
}
