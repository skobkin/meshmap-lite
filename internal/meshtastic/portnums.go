package meshtastic

import (
	"fmt"
	"strings"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

func parseDecodedPacketPayload(base ParsedEvent, data *generated.Data) (ParsedEvent, error) {
	switch data.GetPortnum() {
	// Text payloads.
	case generated.PortNum_TEXT_MESSAGE_APP, generated.PortNum_TEXT_MESSAGE_COMPRESSED_APP:
		base.Kind = ParsedChat
		base.Chat = &ChatPayload{Text: string(data.GetPayload())}

	// Node identity and discovery payloads.
	case generated.PortNum_NODEINFO_APP:
		nodeInfo, nodeID, err := decodeNodeInfoPayload(data.GetPayload())
		if err != nil {
			return ParsedEvent{}, err
		}
		base.Kind = ParsedNodeInfo
		base.NodeInfo = nodeInfo
		if nodeID != "" {
			base.NodeID = nodeID
		}

	// Node state payloads.
	case generated.PortNum_POSITION_APP:
		position, err := decodePositionPayload(data.GetPayload())
		if err != nil {
			return ParsedEvent{}, err
		}
		base.Kind = ParsedPosition
		base.Position = position
	case generated.PortNum_TELEMETRY_APP:
		telemetry, err := decodeTelemetryPayload(data.GetPayload())
		if err != nil {
			return ParsedEvent{}, err
		}
		base.Kind = ParsedTelemetry
		base.Telemetry = telemetry

	// Topology and routing payloads.
	case generated.PortNum_TRACEROUTE_APP:
		traceroute, err := decodeTraceroutePayload(data.GetPayload())
		if err != nil {
			return ParsedEvent{}, err
		}
		base.Kind = ParsedTraceroute
		base.Traceroute = traceroute
	case generated.PortNum_NEIGHBORINFO_APP:
		neighbor, err := decodeNeighborInfoPayload(data.GetPayload())
		if err != nil {
			return ParsedEvent{}, err
		}
		base.Kind = ParsedNeighborInfo
		base.Neighbor = neighbor
		if base.NodeID == "" {
			base.NodeID = neighbor.NodeID
		}
	case generated.PortNum_ROUTING_APP:
		routing, err := decodeRoutingPayload(data.GetPayload())
		if err != nil {
			return ParsedEvent{}, err
		}
		base.Kind = ParsedRouting
		base.Routing = routing

	// Known-but-unhandled app payloads.
	default:
		if data.GetPortnum() == generated.PortNum_UNKNOWN_APP {
			return ParsedEvent{}, fmt.Errorf("unsupported portnum: %s", data.GetPortnum().String())
		}
		base.Kind = ParsedOtherPortnum
		base.Other = &OtherPortnumPayload{
			PortnumValue: int32(data.GetPortnum()),
			PortnumName:  data.GetPortnum().String(),
		}
	}

	return base, nil
}

func parseDecodedPacket(envelope decodedEnvelope) (ParsedEvent, error) {
	base := ParsedEvent{
		NodeID:    nodeIDFromNum(envelope.packet.GetFrom()),
		PacketID:  envelope.packet.GetId(),
		Portnum:   envelope.decoded.GetPortnum(),
		Format:    "protobuf",
		Encrypted: envelope.encrypted,
		Decrypted: envelope.decrypted,
		Timestamp: packetTimestamp(envelope.packet),
	}

	return parseDecodedPacketPayload(base, envelope.decoded)
}

func decodeNodeInfoPayload(payload []byte) (*NodeInfoPayload, string, error) {
	var user generated.User
	if err := proto.Unmarshal(payload, &user); err != nil {
		return nil, "", fmt.Errorf("decode nodeinfo: %w", err)
	}

	nodeInfo := &NodeInfoPayload{
		LongName:        strings.TrimSpace(user.GetLongName()),
		ShortName:       strings.TrimSpace(user.GetShortName()),
		Role:            user.GetRole().String(),
		BoardModel:      user.GetHwModel().String(),
		FirmwareVersion: "",
	}

	return nodeInfo, normalizeNodeID(strings.TrimSpace(user.GetId())), nil
}

func decodePositionPayload(payload []byte) (*PositionPayload, error) {
	var pos generated.Position
	if err := proto.Unmarshal(payload, &pos); err != nil {
		return nil, fmt.Errorf("decode position: %w", err)
	}

	position := &PositionPayload{
		Latitude:  float64(pos.GetLatitudeI()) * positionScale,
		Longitude: float64(pos.GetLongitudeI()) * positionScale,
	}
	if pos.GetAltitude() != 0 {
		altitude := float64(pos.GetAltitude())
		position.AltitudeM = &altitude
	}

	return position, nil
}

func decodeTelemetryPayload(payload []byte) (*TelemetryPayload, error) {
	var tel generated.Telemetry
	if err := proto.Unmarshal(payload, &tel); err != nil {
		return nil, fmt.Errorf("decode telemetry: %w", err)
	}

	telemetry := &TelemetryPayload{}
	if metrics := tel.GetDeviceMetrics(); metrics != nil {
		if metrics.Voltage != nil {
			voltage := float64(metrics.GetVoltage())
			telemetry.Power.Voltage = &voltage
		}
		if metrics.BatteryLevel != nil {
			batteryLevel := float64(metrics.GetBatteryLevel())
			telemetry.Power.BatteryLevel = &batteryLevel
		}
	}
	if metrics := tel.GetEnvironmentMetrics(); metrics != nil {
		if metrics.Temperature != nil {
			temperature := float64(metrics.GetTemperature())
			telemetry.Environment.TemperatureC = &temperature
		}
		if metrics.RelativeHumidity != nil {
			humidity := float64(metrics.GetRelativeHumidity())
			telemetry.Environment.Humidity = &humidity
		}
		if metrics.BarometricPressure != nil {
			pressure := float64(metrics.GetBarometricPressure())
			telemetry.Environment.PressureHpa = &pressure
		}
		if metrics.Iaq != nil {
			iaq := float64(metrics.GetIaq())
			telemetry.AirQuality.IAQ = &iaq
		}
	}
	if metrics := tel.GetAirQualityMetrics(); metrics != nil {
		if metrics.Pm25Standard != nil {
			pm25 := float64(metrics.GetPm25Standard())
			telemetry.AirQuality.PM25 = &pm25
		}
		if metrics.Pm10Standard != nil {
			pm10 := float64(metrics.GetPm10Standard())
			telemetry.AirQuality.PM10 = &pm10
		}
		if metrics.Co2 != nil {
			co2 := float64(metrics.GetCo2())
			telemetry.AirQuality.CO2 = &co2
		}
	}
	if metrics := tel.GetPowerMetrics(); metrics != nil {
		if metrics.Ch1Voltage != nil {
			voltage := float64(metrics.GetCh1Voltage())
			telemetry.Power.Voltage = &voltage
		}
	}

	return telemetry, nil
}

func decodeTraceroutePayload(payload []byte) (*TraceroutePayload, error) {
	var route generated.RouteDiscovery
	if err := proto.Unmarshal(payload, &route); err != nil {
		return nil, fmt.Errorf("decode traceroute: %w", err)
	}

	return &TraceroutePayload{
		HopsTowards: len(route.GetRoute()),
		HopsBack:    len(route.GetRouteBack()),
		SnrTowards:  len(route.GetSnrTowards()),
		SnrBack:     len(route.GetSnrBack()),
	}, nil
}

func decodeNeighborInfoPayload(payload []byte) (*NeighborInfoPayload, error) {
	var info generated.NeighborInfo
	if err := proto.Unmarshal(payload, &info); err != nil {
		return nil, fmt.Errorf("decode neighbor info: %w", err)
	}

	return &NeighborInfoPayload{
		NodeID:            nodeIDFromNum(info.GetNodeId()),
		NeighborsCount:    len(info.GetNeighbors()),
		BroadcastInterval: info.GetNodeBroadcastIntervalSecs(),
	}, nil
}

func decodeRoutingPayload(payload []byte) (*RoutingPayload, error) {
	var routing generated.Routing
	if err := proto.Unmarshal(payload, &routing); err != nil {
		return nil, fmt.Errorf("decode routing: %w", err)
	}

	out := &RoutingPayload{}
	if req := routing.GetRouteRequest(); req != nil {
		out.Variant = "route_request"
		out.HopsTowards = len(req.GetRoute())
		out.HopsBack = len(req.GetRouteBack())

		return out, nil
	}
	if reply := routing.GetRouteReply(); reply != nil {
		out.Variant = "route_reply"
		out.HopsTowards = len(reply.GetRoute())
		out.HopsBack = len(reply.GetRouteBack())

		return out, nil
	}

	out.Variant = "error"
	out.ErrorReason = routing.GetErrorReason().String()

	return out, nil
}
