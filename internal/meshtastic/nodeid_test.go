package meshtastic

import "testing"

func TestNormalizeNodeID(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already normalized", input: "!9028d008", want: "!9028d008"},
		{name: "hex without prefix", input: "9028D008", want: "!9028d008"},
		{name: "decimal", input: "2418593800", want: "!9028d008"},
		{name: "trimmed text", input: "  custom-node  ", want: "custom-node"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeNodeID(tc.input); got != tc.want {
				t.Fatalf("normalizeNodeID(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
