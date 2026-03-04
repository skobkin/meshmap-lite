package ws

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
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
	waitForClientCount(t, hub, 1)

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	waitForClientCount(t, hub, 0)
}

func TestHubBroadcastsEvents(t *testing.T) {
	hub := NewHub(testLogger(), Options{})
	server := httptest.NewServer(hub)
	defer server.Close()

	conn := mustDialWS(t, server.URL)
	defer conn.Close()
	waitForClientCount(t, hub, 1)

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

func TestHubConcurrentBroadcastsDoNotRaceWriters(t *testing.T) {
	hub := NewHub(testLogger(), Options{})
	server := httptest.NewServer(hub)
	defer server.Close()

	conn := mustDialWS(t, server.URL)
	defer conn.Close()
	waitForClientCount(t, hub, 1)

	const emits = 64

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(emits)
	for i := range emits {
		go func(i int) {
			defer wg.Done()
			<-start
			hub.Emit(domain.RealtimeEvent{
				Type: "stats",
				TS:   time.Unix(int64(i), 0).UTC(),
				Payload: map[string]int{
					"seq": i,
				},
			})
		}(i)
	}

	close(start)
	wg.Wait()

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("expected client to remain connected, got %d clients", got)
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}

	for range emits {
		if _, body, err := conn.ReadMessage(); err != nil {
			t.Fatalf("expected %d broadcast messages, read failed: %v", emits, err)
		} else if len(body) == 0 {
			t.Fatalf("expected non-empty broadcast payload")
		}
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

func waitForClientCount(t *testing.T, hub *Hub, want int) {
	t.Helper()

	waitFor(t, func() bool { return hub.ClientCount() == want })
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
