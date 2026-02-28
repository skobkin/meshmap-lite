package ws

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"meshmap-lite/internal/domain"
)

func TestHubConnectDisconnectAccounting(t *testing.T) {
	hub := NewHub(testLogger(), Options{})
	server := httptest.NewServer(hub)
	defer server.Close()

	conn := mustDialWS(t, server.URL)
	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("expected 1 client, got %d", got)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return hub.ClientCount() == 0 })
}

func TestHubBroadcastsEvents(t *testing.T) {
	hub := NewHub(testLogger(), Options{})
	server := httptest.NewServer(hub)
	defer server.Close()

	conn := mustDialWS(t, server.URL)
	defer conn.Close()

	hub.Emit(domain.RealtimeEvent{Type: "stats", TS: time.Unix(10, 0).UTC(), Payload: map[string]string{"status": "ok"}})

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, body, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" {
		t.Fatalf("expected broadcast payload")
	}
}

func TestHubHonorsOriginPolicy(t *testing.T) {
	hub := NewHub(testLogger(), Options{
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == "https://allowed.example"
		},
	})
	server := httptest.NewServer(hub)
	defer server.Close()

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(server.URL), http.Header{"Origin": []string{"https://blocked.example"}})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		_ = conn.Close()
		t.Fatalf("expected blocked origin dial to fail")
	}
}

func mustDialWS(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(serverURL), nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}

	return conn
}

func wsURL(serverURL string) string {
	u, _ := url.Parse(serverURL)
	u.Scheme = "ws"

	return u.String()
}

func waitFor(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met before timeout")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
