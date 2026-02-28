package meshtastic

import "testing"

func TestParseJSONFallbackMapReportInheritsNodeID(t *testing.T) {
	payload := []byte(`{
		"type":"map_report",
		"node_id":"9028d008",
		"map_report":{"long_name":"Node"}
	}`)

	evt, err := parseJSONFallback(TopicKindMapReport, payload)
	if err != nil {
		t.Fatal(err)
	}
	if evt.Kind != ParsedMapReport {
		t.Fatalf("expected map report, got %s", evt.Kind)
	}
	if evt.MapReport == nil || evt.MapReport.NodeID != "!9028d008" {
		t.Fatalf("unexpected map report node id: %#v", evt.MapReport)
	}
}
