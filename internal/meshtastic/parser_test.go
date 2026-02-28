package meshtastic

import "testing"

func TestParsePayloadFallsBackToJSON(t *testing.T) {
	ConfigureChannelKeys(nil)

	payload := []byte(`{
		"type":"chat",
		"node_id":"9028d008",
		"packet_id":42,
		"chat":{"text":"hello"}
	}`)

	evt, err := ParsePayload(TopicKindChannel, payload, "LongFast", "")
	if err != nil {
		t.Fatal(err)
	}
	if evt.Format != "json_fallback" {
		t.Fatalf("expected json fallback format, got %q", evt.Format)
	}
	if evt.Kind != ParsedChat {
		t.Fatalf("expected chat, got %s", evt.Kind)
	}
	if evt.NodeID != "!9028d008" {
		t.Fatalf("unexpected node id: %q", evt.NodeID)
	}
	if evt.Chat == nil || evt.Chat.Text != "hello" {
		t.Fatalf("unexpected chat payload: %#v", evt.Chat)
	}
}
