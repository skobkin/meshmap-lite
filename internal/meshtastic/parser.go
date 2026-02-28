package meshtastic

// ParsePayload decodes real Meshtastic MQTT protobuf payloads.
// JSON fallback remains for local synthetic tests.
func ParsePayload(kind TopicKind, payload []byte, channelHint, mapNodeHint string) (ParsedEvent, error) {
	switch kind {
	case TopicKindMapReport:
		if evt, err := parseMapReportProtobuf(payload, mapNodeHint); err == nil {
			return evt, nil
		}
		if evt, err := parseMapReportEnvelope(payload, channelHint, mapNodeHint); err == nil {
			return evt, nil
		}
	case TopicKindChannel:
		if evt, err := parseServiceEnvelope(payload, channelHint); err == nil {
			return evt, nil
		}
	}

	return parseJSONFallback(kind, payload)
}
