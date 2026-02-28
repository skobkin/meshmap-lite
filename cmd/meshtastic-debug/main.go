//go:build debugtools

package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"meshmap-lite/internal/meshtastic"
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
	ObservedAt string                 `json:"observed_at,omitempty"`
	Topic      string                 `json:"topic"`
	TopicInfo  meshtastic.TopicInfo   `json:"topic_info"`
	Event      meshtastic.ParsedEvent `json:"event,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

func main() {
	var (
		rootTopic  = flag.String("root-topic", "", "MQTT root topic prefix, for example msh/RU/ARKH")
		mapSuffix  = flag.String("map-suffix", "2/map", "Map-report suffix relative to root topic")
		topic      = flag.String("topic", "", "Single packet topic to decode")
		payloadHex = flag.String("payload-hex", "", "Single packet payload in hex")
		pretty     = flag.Bool("pretty", false, "Pretty-print JSON output")
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

		result := decodePacket("", *rootTopic, *mapSuffix, *topic, *payloadHex)
		if result.Error != "" {
			fail(result.Error)
		}

		writeJSON(os.Stdout, result, *pretty)

		return
	}

	if err := decodeStream(os.Stdin, os.Stdout, *rootTopic, *mapSuffix, *pretty); err != nil {
		fail(err.Error())
	}
}

func decodeStream(in *os.File, out *os.File, rootTopic, mapSuffix string, pretty bool) error {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		result := decodeLine(rootTopic, mapSuffix, line)
		writeJSON(out, result, pretty)
	}

	return scanner.Err()
}

func decodeLine(rootTopic, mapSuffix, line string) decodeResult {
	parts := strings.Split(line, "\t")
	switch len(parts) {
	case 2:
		return decodePacket("", rootTopic, mapSuffix, parts[0], parts[1])
	case 3:
		return decodePacket(parts[0], rootTopic, mapSuffix, parts[1], parts[2])
	default:
		return decodeResult{
			Error: fmt.Sprintf("unsupported input line format: expected topic<TAB>hex or observed_at<TAB>topic<TAB>hex, got %q", line),
		}
	}
}

func decodePacket(observedAt, rootTopic, mapSuffix, topic, payloadHex string) decodeResult {
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

	event, err := meshtastic.ParsePayload(result.TopicInfo.Kind, payload, result.TopicInfo.Channel, result.TopicInfo.MapNodeID)
	if err != nil {
		result.Error = fmt.Sprintf("parse payload: %v", err)

		return result
	}

	result.Event = event

	return result
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
