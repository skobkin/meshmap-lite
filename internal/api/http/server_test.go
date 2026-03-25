package httpapi

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"meshmap-lite/internal/mqttclient"
)

func TestHeartbeatEventUsesMQTTConnectionStatus(t *testing.T) {
	t.Parallel()

	srv := New(
		Config{},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		nil,
		func() mqttclient.ConnectionStatus { return mqttclient.ConnectionStatusConnected },
	)

	event := srv.HeartbeatEvent(time.Unix(1774670400, 0))
	payload, ok := event.Payload.(heartbeatPayload)
	if !ok {
		t.Fatalf("unexpected heartbeat payload type: %T", event.Payload)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %q", payload.Status)
	}
	if payload.MQTTConnectionStatus != mqttclient.ConnectionStatusConnected {
		t.Fatalf("expected mqtt status connected, got %q", payload.MQTTConnectionStatus)
	}
}

func TestHeartbeatEventCollapsesUnknownMQTTStateToDisconnected(t *testing.T) {
	t.Parallel()

	srv := New(
		Config{},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		nil,
		func() mqttclient.ConnectionStatus { return "reconnecting" },
	)

	event := srv.HeartbeatEvent(time.Unix(1774670400, 0))
	payload := event.Payload.(heartbeatPayload)
	if payload.MQTTConnectionStatus != mqttclient.ConnectionStatusDisconnected {
		t.Fatalf("expected mqtt status disconnected, got %q", payload.MQTTConnectionStatus)
	}
}
