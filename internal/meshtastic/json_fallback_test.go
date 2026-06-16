package meshtastic

import "testing"

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
	if sf.RR != "ROUTER_STATS" || sf.Role != "router" {
		t.Fatalf("unexpected rr/role: %q %q", sf.RR, sf.Role)
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
	if sf.RR != "CLIENT_HISTORY" || sf.Role != "client" {
		t.Fatalf("unexpected rr/role: %q %q", sf.RR, sf.Role)
	}
	if sf.History == nil || sf.History.HistoryMessages != 3 || sf.History.WindowMinutes != 60 {
		t.Fatalf("unexpected history payload: %#v", sf.History)
	}
}
