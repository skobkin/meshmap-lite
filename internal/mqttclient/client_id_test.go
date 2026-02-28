package mqttclient

import (
	"strings"
	"testing"
)

func TestSanitizeClientID(t *testing.T) {
	t.Run("trims and replaces unsupported characters", func(t *testing.T) {
		got := sanitizeClientID("  bad id/with spaces  ")
		if got != "bad-id-with-spaces" {
			t.Fatalf("unexpected sanitized id: %q", got)
		}
	})

	t.Run("falls back for empty input", func(t *testing.T) {
		if got := sanitizeClientID("   "); got != fallbackClientID {
			t.Fatalf("expected fallback client id, got %q", got)
		}
	})
}

func TestSanitizeClientIDTruncatesToMQTTLimit(t *testing.T) {
	got := sanitizeClientID(strings.Repeat("a", 40))
	if len(got) != maxClientIDLength {
		t.Fatalf("expected max length %d, got %d", maxClientIDLength, len(got))
	}
}

func TestResolveClientIDUsesConfiguredValue(t *testing.T) {
	got := resolveClientID(Options{ClientID: " custom id/123 "})
	if got != "custom-id-123" {
		t.Fatalf("unexpected configured client id: %q", got)
	}
}

func TestResolveClientIDFallbackIsStableAcrossRestarts(t *testing.T) {
	opts := Options{RootTopic: "msh/test"}

	first := resolveClientID(opts)
	second := resolveClientID(opts)
	if first != second {
		t.Fatalf("expected stable fallback id, got %q and %q", first, second)
	}
	if !strings.HasPrefix(first, "mml-") {
		t.Fatalf("expected mml prefix, got %q", first)
	}
	if len(first) > maxClientIDLength {
		t.Fatalf("expected id to fit MQTT length limit, got %d", len(first))
	}
}
