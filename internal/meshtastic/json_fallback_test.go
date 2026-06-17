package meshtastic

import (
	"testing"

	generated "meshmap-lite/internal/meshtasticpb"
)

func TestParseJSONFallbackMapReportInheritsNodeID(t *testing.T) {
	payload := []byte(`{
		"type":"map_report",
		"node_id":"11223344",
		"map_report":{"long_name":"Node"}
	}`)

	evt, err := parseJSONFallback(TopicKindMapReport, payload)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedMapReport {
		t.Fatalf("expected map report, got %s", evt.Kind)
	}
	if evt.MapReport == nil || evt.MapReport.NodeID != "!11223344" {
		t.Fatalf("unexpected map report node id: %#v", evt.MapReport)
	}
}

func TestParseJSONFallbackStoreForward(t *testing.T) {
	payload := []byte(`{
		"type":"store_forward",
		"node_id":"12345678",
		"portnum":65,
		"store_forward":{
			"rr":"ROUTER_STATS",
			"role":"router",
			"stats":{
				"messages_total":42,
				"messages_saved":10,
				"up_time":3600,
				"heartbeat":true
			}
		}
	}`)

	evt, err := parseJSONFallback(TopicKindChannel, payload)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedStoreForward {
		t.Fatalf("expected store_forward, got %s", evt.Kind)
	}
	if evt.NodeID != "!12345678" {
		t.Fatalf("unexpected node id: %q", evt.NodeID)
	}
	if int32(evt.Portnum) != 65 {
		t.Fatalf("unexpected portnum: %v", evt.Portnum)
	}
	sf := evt.StoreForward
	if sf == nil {
		t.Fatal("expected non-nil store forward payload")
	}
	if sf.RR != int32(generated.StoreAndForward_ROUTER_STATS) || sf.Role != StoreForwardRoleRouter {
		t.Fatalf("unexpected rr/role: %d %q", sf.RR, sf.Role)
	}
	if sf.Stats == nil || sf.Stats.MessagesTotal != 42 || sf.Stats.MessagesSaved != 10 || sf.Stats.UpTimeSeconds != 3600 || !sf.Stats.HeartbeatEnabled {
		t.Fatalf("unexpected stats payload: %#v", sf.Stats)
	}
	if sf.History != nil || sf.Heartbeat != nil || sf.Text != "" {
		t.Fatalf("expected only stats sub-payload")
	}
}

func TestParseJSONFallbackStoreForwardClientHistory(t *testing.T) {
	payload := []byte(`{
		"type":"store_forward",
		"node_id":"deadbeef",
		"store_forward":{
			"rr":"CLIENT_HISTORY",
			"role":"client",
			"history":{
				"history_messages":3,
				"window":60
			}
		}
	}`)

	evt, err := parseJSONFallback(TopicKindChannel, payload)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedStoreForward {
		t.Fatalf("expected store_forward, got %s", evt.Kind)
	}
	sf := evt.StoreForward
	if sf == nil {
		t.Fatal("expected non-nil store forward payload")
	}
	if sf.RR != int32(generated.StoreAndForward_CLIENT_HISTORY) || sf.Role != StoreForwardRoleClient {
		t.Fatalf("unexpected rr/role: %d %q", sf.RR, sf.Role)
	}
	if sf.History == nil || sf.History.HistoryMessages != 3 || sf.History.WindowMinutes != 60 {
		t.Fatalf("unexpected history payload: %#v", sf.History)
	}
}

func TestParseJSONFallbackStoreForwardLegacyStringRR(t *testing.T) {
	payload := []byte(`{
		"type":"store_forward",
		"node_id":"12345678",
		"portnum":65,
		"store_forward":{
			"rr":"ROUTER_TEXT_BROADCAST",
			"role":"router",
			"text":"legacy"
		}
	}`)

	evt, err := parseJSONFallback(TopicKindChannel, payload)
	if err != nil {
		t.Fatal(err)
	}
	sf := evt.StoreForward
	if sf == nil {
		t.Fatal("expected non-nil store forward payload")
	}
	if sf.RR != int32(generated.StoreAndForward_ROUTER_TEXT_BROADCAST) {
		t.Fatalf("expected legacy string RR to be mapped, got %d", sf.RR)
	}
	if sf.Role != StoreForwardRoleRouter {
		t.Fatalf("expected role to be derived from RR, got %q", sf.Role)
	}
	if sf.Text != "legacy" {
		t.Fatalf("expected legacy text body, got %q", sf.Text)
	}
}

func TestParseJSONFallbackStoreForwardUnknownRRTolerant(t *testing.T) {
	// Simulate a publisher shipping a RequestResponse value that the
	// pinned proto does not know about. The unmarshaller must NOT
	// reject it — it should land in RawRR with the sentinel RR
	// value, and Role should be Unknown.
	payload := []byte(`{
		"type":"store_forward",
		"node_id":"12345678",
		"store_forward":{
			"rr":"ROUTER_PING_PONG_2027"
		}
	}`)

	evt, err := parseJSONFallback(TopicKindChannel, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sf := evt.StoreForward
	if sf == nil {
		t.Fatal("expected non-nil store forward payload")
	}
	if sf.RR != StoreForwardRRUnknown {
		t.Fatalf("expected RR sentinel, got %d", sf.RR)
	}
	if sf.RawRR != "ROUTER_PING_PONG_2027" {
		t.Fatalf("expected raw_rr preserved, got %q", sf.RawRR)
	}
	if sf.Role != StoreForwardRoleUnknown {
		t.Fatalf("expected unknown role for unknown RR, got %q", sf.Role)
	}
}

func TestParseJSONFallbackStoreForwardUnknownRoleTolerant(t *testing.T) {
	// Legacy JSON carrying a `role` value the decoder does not
	// recognise. The unknown value should be preserved in RawRole
	// and the typed Role should be Unknown.
	payload := []byte(`{
		"type":"store_forward",
		"node_id":"12345678",
		"store_forward":{
			"rr":"ROUTER_STATS",
			"role":"repeater"
		}
	}`)

	evt, err := parseJSONFallback(TopicKindChannel, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sf := evt.StoreForward
	if sf == nil {
		t.Fatal("expected non-nil store forward payload")
	}
	if sf.RR != int32(generated.StoreAndForward_ROUTER_STATS) {
		t.Fatalf("expected known RR, got %d", sf.RR)
	}
	if sf.Role != StoreForwardRoleUnknown {
		t.Fatalf("expected unknown role, got %q", sf.Role)
	}
	if sf.RawRole != "repeater" {
		t.Fatalf("expected raw role preserved, got %q", sf.RawRole)
	}
}
