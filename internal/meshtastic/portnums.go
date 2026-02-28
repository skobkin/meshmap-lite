package meshtastic

import (
	"fmt"
	"strings"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

func parseDecodedPacketPayload(base ParsedEvent, packet *generated.MeshPacket, data *generated.Data) (ParsedEvent, error) {
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
		traceroute, err := decodeTraceroutePayload(packet, data)
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
		routing, err := decodeRoutingPayload(packet, data)
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

	return parseDecodedPacketPayload(base, envelope.packet, envelope.decoded)
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

func decodeTraceroutePayload(packet *generated.MeshPacket, data *generated.Data) (*TraceroutePayload, error) {
	var route generated.RouteDiscovery
	if err := proto.Unmarshal(data.GetPayload(), &route); err != nil {
		return nil, fmt.Errorf("decode traceroute: %w", err)
	}

	fromNodeNum := packet.GetFrom()
	if source := data.GetSource(); source != 0 {
		fromNodeNum = source
	}
	toNodeNum := packet.GetTo()
	if dest := data.GetDest(); dest != 0 {
		toNodeNum = dest
	}

	rawForward := nodeIDsFromNums(route.GetRoute())
	rawReturn := nodeIDsFromNums(route.GetRouteBack())
	requestID := data.GetRequestId()
	role := "reply"
	status := "partial"
	if data.GetWantResponse() {
		role = "request"
		status = "requested"
		requestID = packet.GetId()
	}
	if requestID == 0 {
		requestID = data.GetReplyId()
	}

	var (
		forwardPath     []string
		inferredForward bool
		inferredDirect  bool
		returnPath      []string
		inferredReturn  bool
	)
	if role == "reply" {
		forwardPath, inferredForward, inferredDirect = tracerouteForwardPath(packet, data, &route)
		returnPath, inferredReturn = tracerouteReturnPath(packet, data, &route)
		if len(forwardPath) > 0 {
			status = "completed"
		}
	}

	return &TraceroutePayload{
		Role:                role,
		Status:              status,
		RequestID:           requestID,
		ReplyID:             data.GetReplyId(),
		FromNodeID:          nodeIDFromNum(fromNodeNum),
		ToNodeID:            nodeIDFromNum(toNodeNum),
		Route:               rawForward,
		SnrTowards:          append([]int32(nil), route.GetSnrTowards()...),
		RouteBack:           rawReturn,
		SnrBack:             append([]int32(nil), route.GetSnrBack()...),
		ForwardPath:         forwardPath,
		ReturnPath:          returnPath,
		InferredForwardPath: inferredForward,
		InferredReturnPath:  inferredReturn,
		InferredDirect:      inferredDirect,
		WantResponse:        data.GetWantResponse(),
		HopStart:            packet.GetHopStart(),
		HopLimit:            packet.GetHopLimit(),
		Bitfield:            data.GetBitfield(),
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

func decodeRoutingPayload(packet *generated.MeshPacket, data *generated.Data) (*RoutingPayload, error) {
	var routing generated.Routing
	if err := proto.Unmarshal(data.GetPayload(), &routing); err != nil {
		return nil, fmt.Errorf("decode routing: %w", err)
	}

	out := &RoutingPayload{
		RequestID:     data.GetRequestId(),
		FromNodeID:    nodeIDFromNum(packet.GetFrom()),
		ToNodeID:      nodeIDFromNum(packet.GetTo()),
		TracerouteRef: data.GetRequestId() > 0,
	}
	if req := routing.GetRouteRequest(); req != nil {
		out.Variant = "route_request"
		out.Route = nodeIDsFromNums(req.GetRoute())
		out.RouteBack = nodeIDsFromNums(req.GetRouteBack())

		return out, nil
	}
	if reply := routing.GetRouteReply(); reply != nil {
		out.Variant = "route_reply"
		out.Route = nodeIDsFromNums(reply.GetRoute())
		out.RouteBack = nodeIDsFromNums(reply.GetRouteBack())

		return out, nil
	}

	out.Variant = "error"
	out.ErrorReason = routing.GetErrorReason().String()

	return out, nil
}

func tracerouteForwardPath(packet *generated.MeshPacket, data *generated.Data, route *generated.RouteDiscovery) ([]string, bool, bool) {
	destinationID := data.GetDest()
	if destinationID == 0 {
		destinationID = packet.GetTo()
	}
	sourceID := data.GetSource()
	if sourceID == 0 {
		sourceID = packet.GetFrom()
	}

	if destinationID == 0 && sourceID == 0 && len(route.GetRoute()) == 0 {
		return nil, false, false
	}

	path := make([]string, 0, len(route.GetRoute())+2)
	if destinationID != 0 {
		path = append(path, nodeIDFromNum(destinationID))
	}
	path = append(path, nodeIDsFromNums(route.GetRoute())...)
	if sourceID != 0 {
		path = append(path, nodeIDFromNum(sourceID))
	}

	inferred := destinationID != 0 || sourceID != 0
	inferredDirect := len(route.GetRoute()) == 0 && len(path) == 2

	return path, inferred, inferredDirect
}

func tracerouteReturnPath(packet *generated.MeshPacket, data *generated.Data, route *generated.RouteDiscovery) ([]string, bool) {
	destinationID := data.GetDest()
	if destinationID == 0 {
		destinationID = packet.GetTo()
	}
	sourceID := data.GetSource()
	if sourceID == 0 {
		sourceID = packet.GetFrom()
	}

	raw := nodeIDsFromNums(route.GetRouteBack())
	if (packet.GetHopStart() > 0 || data.GetBitfield() != 0) && len(route.GetSnrBack()) > 0 {
		path := make([]string, 0, len(raw)+2)
		if sourceID != 0 {
			path = append(path, nodeIDFromNum(sourceID))
		}
		path = append(path, raw...)
		if destinationID != 0 {
			path = append(path, nodeIDFromNum(destinationID))
		}

		return path, sourceID != 0 || destinationID != 0
	}
	if len(raw) == 0 {
		return nil, false
	}

	return raw, false
}

func nodeIDsFromNums(nums []uint32) []string {
	if len(nums) == 0 {
		return nil
	}

	out := make([]string, 0, len(nums))
	for _, num := range nums {
		out = append(out, nodeIDFromNum(num))
	}

	return out
}
