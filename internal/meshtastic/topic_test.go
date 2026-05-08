package meshtastic

import "testing"

func TestClassifyTopic(t *testing.T) {
	c := ClassifyTopic("msh/RU/ARKH", "2/map", "msh/RU/ARKH/2/map/")
	if c.Kind != TopicKindMapReport {
		t.Fatalf("expected map report")
	}
	m := ClassifyTopic("msh/RU/ARKH", "2/map", "msh/RU/ARKH/2/map/!11223344")
	if m.Kind != TopicKindMapReport || m.MapNodeID != "!11223344" {
		t.Fatalf("expected map report node id, got %#v", m)
	}
	e := ClassifyTopic("msh/RU/ARKH", "2/map", "msh/RU/ARKH/e/LongFast/gw1")
	if e.Kind != TopicKindChannel || e.Channel != "LongFast" {
		t.Fatalf("expected channel LongFast")
	}
}
