package ws

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub manages websocket clients and broadcasts realtime events to all of them.
type Hub struct {
	upgrader websocket.Upgrader
	log      *slog.Logger
	opts     Options
	mu       sync.RWMutex
	clients  map[*client]struct{}
}

// NewHub builds a websocket hub.
func NewHub(log *slog.Logger, opts Options) *Hub {
	opts = opts.withDefaults()

	return &Hub{
		upgrader: websocket.Upgrader{CheckOrigin: opts.CheckOrigin},
		log:      log,
		opts:     opts,
		clients:  make(map[*client]struct{}),
	}
}

// ClientCount returns the number of currently connected websocket clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

func (h *Hub) register(conn *websocket.Conn, r *http.Request) *client {
	client := &client{
		conn:       conn,
		remoteAddr: r.RemoteAddr,
		userAgent:  r.UserAgent(),
	}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	count := len(h.clients)
	h.mu.Unlock()
	h.log.Info("ws client connected", "remote_addr", client.remoteAddr, "user_agent", client.userAgent, "clients", count)

	return client
}

func (h *Hub) unregister(client *client) {
	h.mu.Lock()
	delete(h.clients, client)
	count := len(h.clients)
	h.mu.Unlock()
	h.log.Info("ws client disconnected", "remote_addr", client.remoteAddr, "clients", count)
	_ = client.close()
}

func (h *Hub) snapshotClients() []*client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]*client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}

	return clients
}
