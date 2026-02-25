package meshtastic

import "testing"

func TestClassifyTopic(t *testing.T) {
	c := ClassifyTopic("msh/RU/ARKH", "2/map", "msh/RU/ARKH/2/map/")
	if c.Kind != TopicKindMapReport {
		t.Fatalf("expected map report")
	}
	e := ClassifyTopic("msh/RU/ARKH", "2/map", "msh/RU/ARKH/e/LongFast/gw1")
	if e.Kind != TopicKindChannel || e.Channel != "LongFast" {
		t.Fatalf("expected channel LongFast")
	}
}
