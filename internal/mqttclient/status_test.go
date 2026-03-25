package mqttclient

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestConnectionStatusDefaultsToDisconnected(t *testing.T) {
	t.Parallel()

	client := New(Options{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if client.ConnectionStatus() != ConnectionStatusDisconnected {
		t.Fatalf("expected disconnected, got %q", client.ConnectionStatus())
	}
}

func TestConnectionStatusTracksLifecycleCallbacks(t *testing.T) {
	t.Parallel()

	client := New(Options{RootTopic: "msh"}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	client.setLifecycleState(lifecycleStateConnecting)
	if client.ConnectionStatus() != ConnectionStatusDisconnected {
		t.Fatalf("expected connecting to simplify to disconnected, got %q", client.ConnectionStatus())
	}

	client.setLifecycleState(lifecycleStateConnected)
	if client.ConnectionStatus() != ConnectionStatusConnected {
		t.Fatalf("expected connected after state update, got %q", client.ConnectionStatus())
	}

	opts := mqtt.NewClientOptions().AddBroker("tcp://localhost:1883").SetClientID("meshmap-lite")
	client.reconnectingHandler()(nil, opts)
	if client.ConnectionStatus() != ConnectionStatusDisconnected {
		t.Fatalf("expected reconnecting to simplify to disconnected, got %q", client.ConnectionStatus())
	}

	client.connectionLostHandler()(nil, errors.New("boom"))
	if client.ConnectionStatus() != ConnectionStatusDisconnected {
		t.Fatalf("expected disconnected after connection loss, got %q", client.ConnectionStatus())
	}
}
