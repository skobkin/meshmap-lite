//go:build debugtools

package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"meshmap-lite/internal/meshtastic"
	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

type channelKeyFlags map[string]string

func (f channelKeyFlags) String() string {
	if len(f) == 0 {
		return ""
	}

	var parts []string
	for name, value := range f {
		parts = append(parts, name+"="+value)
	}

	return strings.Join(parts, ",")
}

func (f channelKeyFlags) Set(value string) error {
	name, psk, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("channel key must be in name=psk form")
	}

	name = strings.TrimSpace(name)
	psk = strings.TrimSpace(psk)
	if name == "" || psk == "" {
		return fmt.Errorf("channel key must include non-empty name and psk")
	}

	f[name] = psk

	return nil
}

type decodeResult struct {
	ObservedAt string                    `json:"observed_at,omitempty"`
	Topic      string                    `json:"topic"`
	TopicInfo  meshtastic.TopicInfo      `json:"topic_info"`
	Event      meshtastic.ParsedEvent    `json:"event,omitempty"`
	Inspect    *lowLevelPacketInspection `json:"inspect,omitempty"`
	Error      string                    `json:"error,omitempty"`
}

type lowLevelPacketInspection struct {
	Envelope *serviceEnvelopeInspection `json:"envelope,omitempty"`
}

type serviceEnvelopeInspection struct {
	ChannelID string                 `json:"channel_id,omitempty"`
	GatewayID string                 `json:"gateway_id,omitempty"`
	Packet    meshPacketInspection   `json:"packet"`
	Data      *decodedDataInspection `json:"data,omitempty"`
}

type meshPacketInspection struct {
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	PacketID   uint32 `json:"packet_id,omitempty"`
	HopStart   uint32 `json:"hop_start,omitempty"`
	HopLimit   uint32 `json:"hop_limit,omitempty"`
	Channel    uint32 `json:"channel,omitempty"`
	RxTime     uint32 `json:"rx_time,omitempty"`
	Priority   string `json:"priority,omitempty"`
	Encrypted  bool   `json:"encrypted,omitempty"`
	PkiEncrypt bool   `json:"pki_encrypted,omitempty"`
}

type decodedDataInspection struct {
	Portnum      string               `json:"portnum,omitempty"`
	WantResponse bool                 `json:"want_response,omitempty"`
	Dest         string               `json:"dest,omitempty"`
	Source       string               `json:"source,omitempty"`
	RequestID    uint32               `json:"request_id,omitempty"`
	ReplyID      uint32               `json:"reply_id,omitempty"`
	Emoji        uint32               `json:"emoji,omitempty"`
	Bitfield     uint32               `json:"bitfield,omitempty"`
	PayloadLen   int                  `json:"payload_len,omitempty"`
	Traceroute   *routeDiscoveryDebug `json:"traceroute,omitempty"`
	Routing      *routingDebug        `json:"routing,omitempty"`
}

type routeDiscoveryDebug struct {
	Route      []string `json:"route,omitempty"`
	SnrTowards []int32  `json:"snr_towards,omitempty"`
	RouteBack  []string `json:"route_back,omitempty"`
	SnrBack    []int32  `json:"snr_back,omitempty"`
}

type routingDebug struct {
	Variant      string               `json:"variant,omitempty"`
	ErrorReason  string               `json:"error_reason,omitempty"`
	RouteRequest *routeDiscoveryDebug `json:"route_request,omitempty"`
	RouteReply   *routeDiscoveryDebug `json:"route_reply,omitempty"`
}

func main() {
	var (
		rootTopic  = flag.String("root-topic", "", "MQTT root topic prefix, for example msh/RU/ARKH")
		mapSuffix  = flag.String("map-suffix", "2/map", "Map-report suffix relative to root topic")
		topic      = flag.String("topic", "", "Single packet topic to decode")
		payloadHex = flag.String("payload-hex", "", "Single packet payload in hex")
		pretty     = flag.Bool("pretty", false, "Pretty-print JSON output")
		inspect    = flag.Bool("inspect-protobuf", false, "Include low-level ServiceEnvelope/MeshPacket/Data inspection for protobuf MQTT packets")
	)
	channelKeys := channelKeyFlags{}
	flag.Var(channelKeys, "channel-key", "Channel PSK mapping in name=base64 form, repeatable")
	flag.Parse()

	if strings.TrimSpace(*rootTopic) == "" {
		fail("missing required -root-topic")
	}

	meshtastic.ConfigureChannelKeys(channelKeys)

	if strings.TrimSpace(*topic) != "" || strings.TrimSpace(*payloadHex) != "" {
		if strings.TrimSpace(*topic) == "" || strings.TrimSpace(*payloadHex) == "" {
			fail("single-packet mode requires both -topic and -payload-hex")
		}

		result := decodePacket("", *rootTopic, *mapSuffix, *topic, *payloadHex, *inspect)
		if result.Error != "" {
			fail(result.Error)
		}

		writeJSON(os.Stdout, result, *pretty)

		return
	}

	if err := decodeStream(os.Stdin, os.Stdout, *rootTopic, *mapSuffix, *pretty, *inspect); err != nil {
		fail(err.Error())
	}
}

func decodeStream(in *os.File, out *os.File, rootTopic, mapSuffix string, pretty, inspect bool) error {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		result := decodeLine(rootTopic, mapSuffix, line, inspect)
		writeJSON(out, result, pretty)
	}

	return scanner.Err()
}

func decodeLine(rootTopic, mapSuffix, line string, inspect bool) decodeResult {
	parts := strings.Split(line, "\t")
	switch len(parts) {
	case 2:
		return decodePacket("", rootTopic, mapSuffix, parts[0], parts[1], inspect)
	case 3:
		return decodePacket(parts[0], rootTopic, mapSuffix, parts[1], parts[2], inspect)
	default:
		return decodeResult{
			Error: fmt.Sprintf("unsupported input line format: expected topic<TAB>hex or observed_at<TAB>topic<TAB>hex, got %q", line),
		}
	}
}

func decodePacket(observedAt, rootTopic, mapSuffix, topic, payloadHex string, inspect bool) decodeResult {
	result := decodeResult{
		ObservedAt: strings.TrimSpace(observedAt),
		Topic:      strings.TrimSpace(topic),
	}

	result.TopicInfo = meshtastic.ClassifyTopic(rootTopic, mapSuffix, result.Topic)

	payload, err := hex.DecodeString(strings.TrimSpace(payloadHex))
	if err != nil {
		result.Error = fmt.Sprintf("decode payload hex: %v", err)

		return result
	}
	if inspect {
		result.Inspect = inspectProtobufPacket(payload, result.TopicInfo.Channel)
	}

	event, err := meshtastic.ParsePayload(result.TopicInfo.Kind, payload, result.TopicInfo.Channel, result.TopicInfo.MapNodeID)
	if err != nil {
		result.Error = fmt.Sprintf("parse payload: %v", err)

		return result
	}

	result.Event = event

	return result
}

func inspectProtobufPacket(payload []byte, topicChannel string) *lowLevelPacketInspection {
	var env generated.ServiceEnvelope
	if err := proto.Unmarshal(payload, &env); err != nil {
		return nil
	}

	packet := env.GetPacket()
	out := &lowLevelPacketInspection{
		Envelope: &serviceEnvelopeInspection{
			ChannelID: strings.TrimSpace(env.GetChannelId()),
			GatewayID: strings.TrimSpace(env.GetGatewayId()),
			Packet: meshPacketInspection{
				From:       normalizeNodeID(packet.GetFrom()),
				To:         normalizeNodeID(packet.GetTo()),
				PacketID:   packet.GetId(),
				HopStart:   uint32(packet.GetHopStart()),
				HopLimit:   uint32(packet.GetHopLimit()),
				Channel:    packet.GetChannel(),
				RxTime:     packet.GetRxTime(),
				Priority:   packet.GetPriority().String(),
				Encrypted:  len(packet.GetEncrypted()) > 0,
				PkiEncrypt: packet.GetPkiEncrypted(),
			},
		},
	}

	if decoded := packet.GetDecoded(); decoded != nil {
		out.Envelope.Data = inspectDecodedData(decoded)

		return out
	}

	if encrypted := packet.GetEncrypted(); len(encrypted) > 0 {
		if decoded, ok := decryptForInspection(packet, env.GetChannelId(), topicChannel); ok {
			out.Envelope.Data = inspectDecodedData(decoded)
		}
	}

	return out
}

func inspectDecodedData(data *generated.Data) *decodedDataInspection {
	if data == nil {
		return nil
	}

	out := &decodedDataInspection{
		Portnum:      data.GetPortnum().String(),
		WantResponse: data.GetWantResponse(),
		Dest:         normalizeNodeID(data.GetDest()),
		Source:       normalizeNodeID(data.GetSource()),
		RequestID:    data.GetRequestId(),
		ReplyID:      data.GetReplyId(),
		Emoji:        data.GetEmoji(),
		Bitfield:     data.GetBitfield(),
		PayloadLen:   len(data.GetPayload()),
	}

	switch data.GetPortnum() {
	case generated.PortNum_TRACEROUTE_APP:
		var route generated.RouteDiscovery
		if err := proto.Unmarshal(data.GetPayload(), &route); err == nil {
			out.Traceroute = &routeDiscoveryDebug{
				Route:      normalizeRoute(route.GetRoute()),
				SnrTowards: append([]int32(nil), route.GetSnrTowards()...),
				RouteBack:  normalizeRoute(route.GetRouteBack()),
				SnrBack:    append([]int32(nil), route.GetSnrBack()...),
			}
		}
	case generated.PortNum_ROUTING_APP:
		var routing generated.Routing
		if err := proto.Unmarshal(data.GetPayload(), &routing); err == nil {
			out.Routing = &routingDebug{
				ErrorReason: routing.GetErrorReason().String(),
			}
			if req := routing.GetRouteRequest(); req != nil {
				out.Routing.Variant = "route_request"
				out.Routing.RouteRequest = &routeDiscoveryDebug{
					Route:      normalizeRoute(req.GetRoute()),
					SnrTowards: append([]int32(nil), req.GetSnrTowards()...),
					RouteBack:  normalizeRoute(req.GetRouteBack()),
					SnrBack:    append([]int32(nil), req.GetSnrBack()...),
				}
			} else if reply := routing.GetRouteReply(); reply != nil {
				out.Routing.Variant = "route_reply"
				out.Routing.RouteReply = &routeDiscoveryDebug{
					Route:      normalizeRoute(reply.GetRoute()),
					SnrTowards: append([]int32(nil), reply.GetSnrTowards()...),
					RouteBack:  normalizeRoute(reply.GetRouteBack()),
					SnrBack:    append([]int32(nil), reply.GetSnrBack()...),
				}
			} else {
				out.Routing.Variant = "error"
			}
		}
	}

	return out
}

func decryptForInspection(packet *generated.MeshPacket, envelopeChannelID, topicChannel string) (*generated.Data, bool) {
	if packet == nil || packet.GetPkiEncrypted() || len(packet.GetEncrypted()) == 0 {
		return nil, false
	}

	keys := configuredChannelKeysForInspection()
	candidates := buildInspectionCandidates(keys, envelopeChannelID, topicChannel)
	if len(candidates) == 0 {
		return nil, false
	}

	hash := byte(packet.GetChannel() & 0xff)
	for _, candidate := range candidates {
		if len(candidate.key) == 0 || candidate.hash != hash {
			continue
		}
		if data, ok := tryDecryptForInspection(packet, candidate.key); ok {
			return data, true
		}
	}
	for _, candidate := range candidates {
		if len(candidate.key) == 0 || candidate.hash == hash {
			continue
		}
		if data, ok := tryDecryptForInspection(packet, candidate.key); ok {
			return data, true
		}
	}

	return nil, false
}

type inspectionCandidate struct {
	key  []byte
	hash byte
}

func configuredChannelKeysForInspection() map[string]string {
	return meshtastic.CurrentChannelKeys()
}

func buildInspectionCandidates(keys map[string]string, envelopeChannelID, topicChannel string) []inspectionCandidate {
	names := make([]string, 0, len(keys)+2)
	seen := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	add(envelopeChannelID)
	add(topicChannel)
	for name := range keys {
		add(name)
	}

	out := make([]inspectionCandidate, 0, len(names))
	for _, name := range names {
		psk, ok := keyForInspection(keys, name)
		if !ok {
			continue
		}
		key, ok := decodeAndExpandPSKForInspection(psk)
		if !ok {
			continue
		}
		out = append(out, inspectionCandidate{
			key:  key,
			hash: channelHashForInspection(name, key),
		})
	}

	return out
}

func keyForInspection(keys map[string]string, name string) (string, bool) {
	if psk, ok := keys[name]; ok {
		return psk, true
	}
	for key, value := range keys {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(name)) {
			return value, true
		}
	}

	return "", false
}

func decodeAndExpandPSKForInspection(encoded string) ([]byte, bool) {
	key, keyLen, ok := meshtastic.DecodeAndExpandPSKForDebug(strings.TrimSpace(encoded))
	if !ok || keyLen <= 0 {
		return nil, false
	}

	out := make([]byte, keyLen)
	copy(out, key[:keyLen])

	return out, true
}

func channelHashForInspection(name string, key []byte) byte {
	var hash byte
	for i := 0; i < len(name); i++ {
		hash ^= name[i]
	}
	for i := 0; i < len(key); i++ {
		hash ^= key[i]
	}

	return hash
}

func tryDecryptForInspection(packet *generated.MeshPacket, key []byte) (*generated.Data, bool) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}

	iv := make([]byte, 16)
	binary.LittleEndian.PutUint64(iv[0:8], uint64(packet.GetId()))
	binary.LittleEndian.PutUint32(iv[8:12], packet.GetFrom())

	plaintext := make([]byte, len(packet.GetEncrypted()))
	copy(plaintext, packet.GetEncrypted())
	cipher.NewCTR(block, iv).XORKeyStream(plaintext, plaintext)

	var data generated.Data
	if err := proto.Unmarshal(plaintext, &data); err != nil {
		return nil, false
	}
	if data.GetPortnum() == generated.PortNum_UNKNOWN_APP {
		return nil, false
	}

	return &data, true
}

func normalizeRoute(route []uint32) []string {
	if len(route) == 0 {
		return nil
	}

	out := make([]string, 0, len(route))
	for _, hop := range route {
		out = append(out, normalizeNodeID(hop))
	}

	return out
}

func normalizeNodeID(nodeNum uint32) string {
	if nodeNum == 0 {
		return ""
	}

	return fmt.Sprintf("!%08x", nodeNum)
}

func writeJSON(out *os.File, value any, pretty bool) {
	encoder := json.NewEncoder(out)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
