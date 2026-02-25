package meshtastic

import "strings"

// TopicKind classifies MQTT topics used by Meshtastic ingest.
type TopicKind string

// Supported MQTT topic classifications.
const (
	TopicKindUnknown   TopicKind = "unknown"
	TopicKindChannel   TopicKind = "channel"
	TopicKindMapReport TopicKind = "map_report"
)

// TopicInfo is parsed metadata extracted from an MQTT topic string.
type TopicInfo struct {
	Kind       TopicKind
	Channel    string
	GatewayID  string
	MapNodeID  string
	IsFromMQTT bool
}

// ClassifyTopic parses an MQTT topic into normalized Meshtastic topic metadata.
func ClassifyTopic(rootTopic, mapSuffix, topic string) TopicInfo {
	root := strings.Trim(rootTopic, "/")
	fullMap := strings.Trim(root+"/"+strings.Trim(mapSuffix, "/"), "/")
	current := strings.Trim(topic, "/")
	if current == fullMap {
		return TopicInfo{Kind: TopicKindMapReport}
	}
	if strings.HasPrefix(current, fullMap+"/") {
		tail := strings.TrimPrefix(current, fullMap+"/")
		tail = strings.Trim(tail, "/")

		return TopicInfo{Kind: TopicKindMapReport, MapNodeID: tail}
	}
	if !strings.HasPrefix(current, root+"/") {
		return TopicInfo{Kind: TopicKindUnknown}
	}
	parts := strings.Split(current, "/")
	for i := range parts {
		if parts[i] != "e" || i+2 >= len(parts) {
			continue
		}

		return TopicInfo{Kind: TopicKindChannel, Channel: parts[i+1], GatewayID: parts[i+2], IsFromMQTT: true}
	}

	return TopicInfo{Kind: TopicKindUnknown}
}
