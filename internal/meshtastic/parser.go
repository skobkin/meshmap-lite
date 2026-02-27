package meshtastic

import (
	cryptoaes "crypto/aes"
	cryptocipher "crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

const positionScale = 1e-7

var (
	defaultPSK = []byte{0xd4, 0xf1, 0xbb, 0x3a, 0x20, 0x29, 0x07, 0x59, 0xf0, 0xbc, 0xff, 0xab, 0xcf, 0x4e, 0x69, 0x01}

	channelKeyMu sync.RWMutex
	channelKeys  = map[string]string{}
)

// ConfigureChannelKeys sets channel-name->PSK mappings used for decrypting
// MQTT encrypted MeshPacket payloads.
func ConfigureChannelKeys(m map[string]string) {
	next := make(map[string]string, len(m))
	for k, v := range m {
		name := strings.TrimSpace(k)
		if name == "" {
			continue
		}
		next[name] = strings.TrimSpace(v)
	}
	channelKeyMu.Lock()
	channelKeys = next
	channelKeyMu.Unlock()
}

// ParsedKind classifies decoded Meshtastic packet payload types.
type ParsedKind string

// Parsed Meshtastic payload families.
const (
	ParsedChat             ParsedKind = "chat"
	ParsedNodeInfo         ParsedKind = "node_info"
	ParsedPosition         ParsedKind = "position"
	ParsedTelemetry        ParsedKind = "telemetry"
	ParsedMapReport        ParsedKind = "map_report"
	ParsedTraceroute       ParsedKind = "traceroute"
	ParsedNeighborInfo     ParsedKind = "neighbor_info"
	ParsedRouting          ParsedKind = "routing"
	ParsedOtherPortnum     ParsedKind = "other_portnum"
	ParsedUnknownEncrypted ParsedKind = "unknown_encrypted"
)

// ParsedEvent is a normalized decoded payload produced by parser.
type ParsedEvent struct {
	Kind       ParsedKind
	NodeID     string
	PacketID   uint32
	Portnum    generated.PortNum
	Format     string
	Encrypted  bool
	Decrypted  bool
	Timestamp  *time.Time
	Chat       *ChatPayload
	NodeInfo   *NodeInfoPayload
	Position   *PositionPayload
	Telemetry  *TelemetryPayload
	MapReport  *MapReportPayload
	Traceroute *TraceroutePayload
	Neighbor   *NeighborInfoPayload
	Routing    *RoutingPayload
	Other      *OtherPortnumPayload
}

// ChatPayload contains decoded text message fields.
type ChatPayload struct {
	Text   string `json:"text"`
	Sender string `json:"sender"`
}

// NodeInfoPayload contains decoded node identity and capabilities fields.
type NodeInfoPayload struct {
	LongName               string `json:"long_name"`
	ShortName              string `json:"short_name"`
	Role                   string `json:"role"`
	BoardModel             string `json:"board_model"`
	FirmwareVersion        string `json:"firmware_version"`
	LoRaRegion             string `json:"lora_region"`
	LoRaFrequencyDesc      string `json:"lora_frequency_desc"`
	ModemPreset            string `json:"modem_preset"`
	HasDefaultChannel      *bool  `json:"has_default_channel,omitempty"`
	HasOptedReportLocation *bool  `json:"has_opted_report_location,omitempty"`
	NeighborNodesCount     *int   `json:"neighbor_nodes_count"`
}

// PositionPayload contains decoded geolocation fields.
type PositionPayload struct {
	Latitude          float64  `json:"latitude"`
	Longitude         float64  `json:"longitude"`
	AltitudeM         *float64 `json:"altitude_m"`
	PositionPrecision *uint32  `json:"position_precision,omitempty"`
}

// TelemetryPayload contains decoded telemetry sections.
type TelemetryPayload struct {
	Power struct {
		Voltage      *float64 `json:"voltage"`
		BatteryLevel *float64 `json:"battery_level"`
	} `json:"power"`
	Environment struct {
		TemperatureC *float64 `json:"temperature_c"`
		Humidity     *float64 `json:"humidity"`
		PressureHpa  *float64 `json:"pressure_hpa"`
	} `json:"environment"`
	AirQuality struct {
		PM25 *float64 `json:"pm25"`
		PM10 *float64 `json:"pm10"`
		CO2  *float64 `json:"co2"`
		IAQ  *float64 `json:"iaq"`
	} `json:"air_quality"`
}

// MapReportPayload contains decoded map report content.
type MapReportPayload struct {
	NodeID                 string   `json:"node_id"`
	LongName               string   `json:"long_name"`
	ShortName              string   `json:"short_name"`
	Role                   string   `json:"role"`
	BoardModel             string   `json:"board_model"`
	FirmwareVersion        string   `json:"firmware_version"`
	LoRaRegion             string   `json:"lora_region"`
	ModemPreset            string   `json:"modem_preset"`
	HasDefaultChannel      bool     `json:"has_default_channel"`
	HasOptedReportLocation bool     `json:"has_opted_report_location"`
	NeighborNodesCount     *int     `json:"neighbor_nodes_count"`
	Latitude               float64  `json:"latitude"`
	Longitude              float64  `json:"longitude"`
	AltitudeM              *float64 `json:"altitude_m"`
	PositionPrecision      *uint32  `json:"position_precision"`
}

// TraceroutePayload contains compact TRACEROUTE_APP details.
type TraceroutePayload struct {
	HopsTowards int `json:"hops_towards"`
	HopsBack    int `json:"hops_back"`
	SnrTowards  int `json:"snr_towards"`
	SnrBack     int `json:"snr_back"`
}

// NeighborInfoPayload contains compact NEIGHBORINFO_APP details.
type NeighborInfoPayload struct {
	NodeID            string `json:"node_id,omitempty"`
	NeighborsCount    int    `json:"neighbors_count"`
	BroadcastInterval uint32 `json:"broadcast_interval_secs,omitempty"`
}

// RoutingPayload contains compact ROUTING_APP details.
type RoutingPayload struct {
	Variant     string `json:"variant"`
	HopsTowards int    `json:"hops_towards,omitempty"`
	HopsBack    int    `json:"hops_back,omitempty"`
	ErrorReason string `json:"error_reason,omitempty"`
}

// OtherPortnumPayload carries fallback details for known-but-unhandled app packets.
type OtherPortnumPayload struct {
	PortnumValue int32  `json:"portnum_value"`
	PortnumName  string `json:"portnum_name"`
}

// ParsePayload decodes real Meshtastic MQTT protobuf payloads.
// JSON fallback remains for local synthetic tests.
func ParsePayload(kind TopicKind, payload []byte, channelHint, mapNodeHint string) (ParsedEvent, error) {
	if kind == TopicKindMapReport {
		if evt, err := parseMapReportProtobuf(payload, mapNodeHint); err == nil {
			return evt, nil
		}
		if evt, err := parseMapReportEnvelope(payload, channelHint, mapNodeHint); err == nil {
			return evt, nil
		}
	}
	if kind == TopicKindChannel {
		if evt, err := parseServiceEnvelope(payload, channelHint); err == nil {
			return evt, nil
		}
	}

	return parseJSONFallback(kind, payload)
}

func parseServiceEnvelope(payload []byte, channelHint string) (ParsedEvent, error) {
	var env generated.ServiceEnvelope
	if err := proto.Unmarshal(payload, &env); err != nil {
		return ParsedEvent{}, fmt.Errorf("decode service envelope: %w", err)
	}
	packet := env.GetPacket()
	if packet == nil {
		return ParsedEvent{}, fmt.Errorf("empty packet")
	}
	decoded := packet.GetDecoded()
	encrypted := decoded == nil
	wasDecrypted := false
	if decoded == nil {
		decryptedData, ok := decryptPacketIfPossible(packet, env.GetChannelId(), channelHint)
		if !ok {
			return ParsedEvent{
				Kind:      ParsedUnknownEncrypted,
				NodeID:    nodeIDFromNum(packet.GetFrom()),
				PacketID:  packet.GetId(),
				Format:    "protobuf",
				Encrypted: true,
				Decrypted: false,
			}, nil
		}
		decoded = decryptedData
		wasDecrypted = true
	}
	out := ParsedEvent{
		NodeID:    nodeIDFromNum(packet.GetFrom()),
		PacketID:  packet.GetId(),
		Portnum:   decoded.GetPortnum(),
		Format:    "protobuf",
		Encrypted: encrypted,
		Decrypted: wasDecrypted,
	}
	if rx := packet.GetRxTime(); rx > 0 {
		ts := time.Unix(int64(rx), 0).UTC()
		out.Timestamp = &ts
	}
	switch decoded.GetPortnum() {
	case generated.PortNum_TEXT_MESSAGE_APP, generated.PortNum_TEXT_MESSAGE_COMPRESSED_APP:
		out.Kind = ParsedChat
		out.Chat = &ChatPayload{Text: string(decoded.GetPayload())}
	case generated.PortNum_NODEINFO_APP:
		var user generated.User
		if err := proto.Unmarshal(decoded.GetPayload(), &user); err != nil {
			return ParsedEvent{}, fmt.Errorf("decode nodeinfo: %w", err)
		}
		out.Kind = ParsedNodeInfo
		out.NodeInfo = &NodeInfoPayload{
			LongName:   strings.TrimSpace(user.GetLongName()),
			ShortName:  strings.TrimSpace(user.GetShortName()),
			Role:       user.GetRole().String(),
			BoardModel: user.GetHwModel().String(),
			// User payload does not include firmware version; keep it empty here.
			FirmwareVersion: "",
		}
		if id := strings.TrimSpace(user.GetId()); id != "" {
			out.NodeID = normalizeNodeID(id)
		}
	case generated.PortNum_POSITION_APP:
		var pos generated.Position
		if err := proto.Unmarshal(decoded.GetPayload(), &pos); err != nil {
			return ParsedEvent{}, fmt.Errorf("decode position: %w", err)
		}
		out.Kind = ParsedPosition
		out.Position = &PositionPayload{Latitude: float64(pos.GetLatitudeI()) * positionScale, Longitude: float64(pos.GetLongitudeI()) * positionScale}
		if pos.GetAltitude() != 0 {
			v := float64(pos.GetAltitude())
			out.Position.AltitudeM = &v
		}
	case generated.PortNum_TELEMETRY_APP:
		var tel generated.Telemetry
		if err := proto.Unmarshal(decoded.GetPayload(), &tel); err != nil {
			return ParsedEvent{}, fmt.Errorf("decode telemetry: %w", err)
		}
		out.Kind = ParsedTelemetry
		t := &TelemetryPayload{}
		if v := tel.GetDeviceMetrics(); v != nil {
			if v.Voltage != nil {
				x := float64(v.GetVoltage())
				t.Power.Voltage = &x
			}
			if v.BatteryLevel != nil {
				x := float64(v.GetBatteryLevel())
				t.Power.BatteryLevel = &x
			}
		}
		if v := tel.GetEnvironmentMetrics(); v != nil {
			if v.Temperature != nil {
				x := float64(v.GetTemperature())
				t.Environment.TemperatureC = &x
			}
			if v.RelativeHumidity != nil {
				x := float64(v.GetRelativeHumidity())
				t.Environment.Humidity = &x
			}
			if v.BarometricPressure != nil {
				x := float64(v.GetBarometricPressure())
				t.Environment.PressureHpa = &x
			}
			if v.Iaq != nil {
				x := float64(v.GetIaq())
				t.AirQuality.IAQ = &x
			}
		}
		if v := tel.GetAirQualityMetrics(); v != nil {
			if v.Pm25Standard != nil {
				x := float64(v.GetPm25Standard())
				t.AirQuality.PM25 = &x
			}
			if v.Pm10Standard != nil {
				x := float64(v.GetPm10Standard())
				t.AirQuality.PM10 = &x
			}
			if v.Co2 != nil {
				x := float64(v.GetCo2())
				t.AirQuality.CO2 = &x
			}
		}
		if v := tel.GetPowerMetrics(); v != nil {
			if v.Ch1Voltage != nil {
				x := float64(v.GetCh1Voltage())
				t.Power.Voltage = &x
			}
		}
		out.Telemetry = t
	case generated.PortNum_TRACEROUTE_APP:
		var route generated.RouteDiscovery
		if err := proto.Unmarshal(decoded.GetPayload(), &route); err != nil {
			return ParsedEvent{}, fmt.Errorf("decode traceroute: %w", err)
		}
		out.Kind = ParsedTraceroute
		out.Traceroute = &TraceroutePayload{
			HopsTowards: len(route.GetRoute()),
			HopsBack:    len(route.GetRouteBack()),
			SnrTowards:  len(route.GetSnrTowards()),
			SnrBack:     len(route.GetSnrBack()),
		}
	case generated.PortNum_NEIGHBORINFO_APP:
		var info generated.NeighborInfo
		if err := proto.Unmarshal(decoded.GetPayload(), &info); err != nil {
			return ParsedEvent{}, fmt.Errorf("decode neighbor info: %w", err)
		}
		out.Kind = ParsedNeighborInfo
		out.Neighbor = &NeighborInfoPayload{
			NodeID:            nodeIDFromNum(info.GetNodeId()),
			NeighborsCount:    len(info.GetNeighbors()),
			BroadcastInterval: info.GetNodeBroadcastIntervalSecs(),
		}
		if out.NodeID == "" {
			out.NodeID = out.Neighbor.NodeID
		}
	case generated.PortNum_ROUTING_APP:
		var routing generated.Routing
		if err := proto.Unmarshal(decoded.GetPayload(), &routing); err != nil {
			return ParsedEvent{}, fmt.Errorf("decode routing: %w", err)
		}
		out.Kind = ParsedRouting
		rp := &RoutingPayload{}
		if req := routing.GetRouteRequest(); req != nil {
			rp.Variant = "route_request"
			rp.HopsTowards = len(req.GetRoute())
			rp.HopsBack = len(req.GetRouteBack())
		} else if reply := routing.GetRouteReply(); reply != nil {
			rp.Variant = "route_reply"
			rp.HopsTowards = len(reply.GetRoute())
			rp.HopsBack = len(reply.GetRouteBack())
		} else {
			rp.Variant = "error"
			rp.ErrorReason = routing.GetErrorReason().String()
		}
		out.Routing = rp
	default:
		if decoded.GetPortnum() == generated.PortNum_UNKNOWN_APP {
			return ParsedEvent{}, fmt.Errorf("unsupported portnum: %s", decoded.GetPortnum().String())
		}
		out.Kind = ParsedOtherPortnum
		out.Other = &OtherPortnumPayload{
			PortnumValue: int32(decoded.GetPortnum()),
			PortnumName:  decoded.GetPortnum().String(),
		}
	}

	return out, nil
}

func decryptPacket(packet *generated.MeshPacket, envelopeChannelID, topicChannel string) (*generated.Data, error) {
	if packet.GetPkiEncrypted() {
		return nil, fmt.Errorf("pki encrypted packet unsupported in mqtt parser")
	}
	ciphertext := packet.GetEncrypted()
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("missing encrypted payload")
	}

	keys := configuredChannelKeys()
	candidates := buildChannelCandidates(keys, envelopeChannelID, topicChannel)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no channel key configured for encrypted packet")
	}

	hash := byte(packet.GetChannel() & 0xff)
	tried := 0
	for _, c := range candidates {
		if c.KeyLen <= 0 {
			continue
		}
		if c.Hash != hash {
			continue
		}
		tried++
		if d, ok := tryDecryptData(packet, ciphertext, c.Key[:c.KeyLen]); ok {
			return d, nil
		}
	}
	// Hash is a hint; if no exact-hash candidate works, try remaining keys.
	for _, c := range candidates {
		if c.KeyLen <= 0 || c.Hash == hash {
			continue
		}
		tried++
		if d, ok := tryDecryptData(packet, ciphertext, c.Key[:c.KeyLen]); ok {
			return d, nil
		}
	}
	if tried == 0 {
		return nil, fmt.Errorf("no decryptable keys for encrypted packet")
	}

	return nil, fmt.Errorf("failed to decrypt encrypted packet (bad psk?)")
}

func decryptPacketIfPossible(packet *generated.MeshPacket, envelopeChannelID, topicChannel string) (*generated.Data, bool) {
	decoded, err := decryptPacket(packet, envelopeChannelID, topicChannel)
	if err != nil {
		return nil, false
	}

	return decoded, true
}

type channelCandidate struct {
	Name   string
	Key    [32]byte
	KeyLen int
	Hash   byte
}

func buildChannelCandidates(keys map[string]string, envelopeChannelID, topicChannel string) []channelCandidate {
	names := make([]string, 0, len(keys)+2)
	seen := map[string]struct{}{}
	add := func(name string) {
		raw := strings.TrimSpace(name)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		names = append(names, raw)
	}
	add(envelopeChannelID)
	add(topicChannel)
	for k := range keys {
		add(k)
	}

	out := make([]channelCandidate, 0, len(names))
	for _, name := range names {
		pskRaw, ok := keyForChannelName(keys, name)
		if !ok {
			continue
		}
		k, klen, ok := decodeAndExpandPSK(pskRaw)
		if !ok {
			continue
		}
		out = append(out, channelCandidate{
			Name:   name,
			Key:    k,
			KeyLen: klen,
			Hash:   channelHash(name, k[:klen]),
		})
	}

	return out
}

func keyForChannelName(keys map[string]string, name string) (string, bool) {
	needle := strings.TrimSpace(name)
	psk, ok := keys[needle]
	if ok {
		return psk, true
	}
	for key, value := range keys {
		if strings.EqualFold(strings.TrimSpace(key), needle) {
			return value, true
		}
	}

	return "", false
}

func decodeAndExpandPSK(encoded string) ([32]byte, int, bool) {
	var out [32]byte
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return out, 0, false
	}

	keyBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		keyBytes, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return out, 0, false
		}
	}
	switch l := len(keyBytes); {
	case l == 0:
		return out, 0, true
	case l == 1:
		idx := keyBytes[0]
		if idx == 0 {
			return out, 0, true
		}
		copy(out[:], defaultPSK)
		out[len(defaultPSK)-1] = out[len(defaultPSK)-1] + idx - 1

		return out, len(defaultPSK), true
	case l < 16:
		copy(out[:], keyBytes)

		return out, 16, true
	case l == 16:
		copy(out[:], keyBytes)

		return out, 16, true
	case l < 32:
		copy(out[:], keyBytes)

		return out, 32, true
	case l == 32:
		copy(out[:], keyBytes)

		return out, 32, true
	default:
		return out, 0, false
	}
}

func configuredChannelKeys() map[string]string {
	channelKeyMu.RLock()
	defer channelKeyMu.RUnlock()
	out := make(map[string]string, len(channelKeys))
	for k, v := range channelKeys {
		out[k] = v
	}

	return out
}

func tryDecryptData(packet *generated.MeshPacket, ciphertext, key []byte) (*generated.Data, bool) {
	block, err := cryptoaes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	iv := make([]byte, 16)
	binary.LittleEndian.PutUint64(iv[0:8], uint64(packet.GetId()))
	binary.LittleEndian.PutUint32(iv[8:12], packet.GetFrom())

	plaintext := make([]byte, len(ciphertext))
	copy(plaintext, ciphertext)
	cryptocipher.NewCTR(block, iv).XORKeyStream(plaintext, plaintext)

	var data generated.Data
	if err := proto.Unmarshal(plaintext, &data); err != nil {
		return nil, false
	}
	if data.GetPortnum() == generated.PortNum_UNKNOWN_APP {
		return nil, false
	}

	return &data, true
}

func channelHash(name string, key []byte) byte {
	var h byte
	for i := 0; i < len(name); i++ {
		h ^= name[i]
	}
	for i := 0; i < len(key); i++ {
		h ^= key[i]
	}

	return h
}

func parseMapReportProtobuf(payload []byte, mapNodeHint string) (ParsedEvent, error) {
	var report generated.MapReport
	if err := proto.Unmarshal(payload, &report); err != nil {
		return ParsedEvent{}, fmt.Errorf("decode map report: %w", err)
	}
	out := ParsedEvent{Kind: ParsedMapReport, NodeID: normalizeNodeID(mapNodeHint), Portnum: generated.PortNum_MAP_REPORT_APP, Format: "protobuf"}
	m := &MapReportPayload{
		NodeID:                 out.NodeID,
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
		alt := float64(report.GetAltitude())
		m.AltitudeM = &alt
	}
	if report.GetPositionPrecision() > 0 {
		pp := report.GetPositionPrecision()
		m.PositionPrecision = &pp
	}
	if report.GetNumOnlineLocalNodes() > 0 {
		n := int(report.GetNumOnlineLocalNodes())
		m.NeighborNodesCount = &n
	}
	out.MapReport = m

	return out, nil
}

func parseMapReportEnvelope(payload []byte, channelHint, mapNodeHint string) (ParsedEvent, error) {
	var env generated.ServiceEnvelope
	if err := proto.Unmarshal(payload, &env); err != nil {
		return ParsedEvent{}, fmt.Errorf("decode service envelope: %w", err)
	}
	packet := env.GetPacket()
	if packet == nil {
		return ParsedEvent{}, fmt.Errorf("empty packet")
	}
	decoded := packet.GetDecoded()
	encrypted := decoded == nil
	wasDecrypted := false
	if decoded == nil {
		decryptedData, ok := decryptPacketIfPossible(packet, env.GetChannelId(), channelHint)
		if !ok {
			return ParsedEvent{
				Kind:      ParsedUnknownEncrypted,
				NodeID:    nodeIDFromNum(packet.GetFrom()),
				PacketID:  packet.GetId(),
				Portnum:   generated.PortNum_MAP_REPORT_APP,
				Format:    "protobuf",
				Encrypted: true,
				Decrypted: false,
			}, nil
		}
		decoded = decryptedData
		wasDecrypted = true
	}
	if decoded == nil || len(decoded.GetPayload()) == 0 {
		return ParsedEvent{}, fmt.Errorf("missing map report payload")
	}
	nodeID := nodeIDFromNum(packet.GetFrom())
	if nodeID == "" {
		nodeID = mapNodeHint
	}
	out, err := parseMapReportProtobuf(decoded.GetPayload(), nodeID)
	if err != nil {
		return ParsedEvent{}, err
	}
	out.PacketID = packet.GetId()
	out.Portnum = generated.PortNum_MAP_REPORT_APP
	out.Encrypted = encrypted
	out.Decrypted = wasDecrypted
	if rx := packet.GetRxTime(); rx > 0 {
		ts := time.Unix(int64(rx), 0).UTC()
		out.Timestamp = &ts
	}

	return out, nil
}

func nodeIDFromNum(v uint32) string {
	if v == 0 {
		return ""
	}

	return fmt.Sprintf("!%08x", v)
}

func normalizeNodeID(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "!") {
		return s
	}
	if len(s) == 8 {
		if _, err := hex.DecodeString(s); err == nil {
			return "!" + s
		}
	}
	if n, err := strconv.ParseUint(s, 10, 32); err == nil && n > 0 {
		return fmt.Sprintf("!%08x", uint32(n))
	}

	return s
}

func parseJSONFallback(kind TopicKind, payload []byte) (ParsedEvent, error) {
	var raw struct {
		Type       string              `json:"type"`
		NodeID     string              `json:"node_id"`
		PacketID   uint32              `json:"packet_id"`
		Portnum    int32               `json:"portnum"`
		Timestamp  *time.Time          `json:"timestamp"`
		Chat       ChatPayload         `json:"chat"`
		NodeInfo   NodeInfoPayload     `json:"node_info"`
		Position   PositionPayload     `json:"position"`
		Telemetry  TelemetryPayload    `json:"telemetry"`
		MapReport  MapReportPayload    `json:"map_report"`
		Traceroute TraceroutePayload   `json:"traceroute"`
		Neighbor   NeighborInfoPayload `json:"neighbor_info"`
		Routing    RoutingPayload      `json:"routing"`
		Other      OtherPortnumPayload `json:"other"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ParsedEvent{}, fmt.Errorf("decode payload: %w", err)
	}
	out := ParsedEvent{
		NodeID:    normalizeNodeID(raw.NodeID),
		PacketID:  raw.PacketID,
		Portnum:   generated.PortNum(raw.Portnum),
		Timestamp: raw.Timestamp,
		Format:    "json_fallback",
	}
	if kind == TopicKindMapReport || raw.Type == "map_report" {
		out.Kind = ParsedMapReport
		out.MapReport = &raw.MapReport
		if out.MapReport.NodeID == "" {
			out.MapReport.NodeID = out.NodeID
		}

		return out, nil
	}
	switch raw.Type {
	case "chat", "text_message":
		out.Kind = ParsedChat
		out.Chat = &raw.Chat
	case "node_info":
		out.Kind = ParsedNodeInfo
		out.NodeInfo = &raw.NodeInfo
	case "position":
		out.Kind = ParsedPosition
		out.Position = &raw.Position
	case "telemetry":
		out.Kind = ParsedTelemetry
		out.Telemetry = &raw.Telemetry
	case "traceroute":
		out.Kind = ParsedTraceroute
		out.Traceroute = &raw.Traceroute
	case "neighbor_info":
		out.Kind = ParsedNeighborInfo
		out.Neighbor = &raw.Neighbor
	case "routing":
		out.Kind = ParsedRouting
		out.Routing = &raw.Routing
	case "other_portnum":
		out.Kind = ParsedOtherPortnum
		out.Other = &raw.Other
	case "unknown_encrypted":
		out.Kind = ParsedUnknownEncrypted
		out.Encrypted = true
	default:
		return ParsedEvent{}, fmt.Errorf("unsupported event type %q", raw.Type)
	}

	return out, nil
}
