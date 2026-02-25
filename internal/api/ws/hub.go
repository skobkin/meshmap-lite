package ws

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"meshmap-lite/internal/domain"
)

// Hub manages websocket clients and broadcasts realtime events to all of them.
type Hub struct {
	upgrader websocket.Upgrader
	log      *slog.Logger
	mu       sync.RWMutex
	clients  map[*websocket.Conn]struct{}
}

// NewHub builds a websocket hub with permissive origin checks for local deployment.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }},
		log:      log,
		clients:  make(map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("ws upgrade failed", "err", err)

		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	count := len(h.clients)
	h.mu.Unlock()
	h.log.Info("ws client connected", "remote_addr", r.RemoteAddr, "user_agent", r.UserAgent(), "clients", count)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) && !errors.Is(err, websocket.ErrCloseSent) {
				h.log.Debug("ws read failed", "remote_addr", r.RemoteAddr, "err", err)
			}

			break
		}
	}
	h.mu.Lock()
	delete(h.clients, conn)
	count = len(h.clients)
	h.mu.Unlock()
	h.log.Info("ws client disconnected", "remote_addr", r.RemoteAddr, "clients", count)
	_ = conn.Close()
}

// Emit broadcasts a realtime event to all connected clients.
func (h *Hub) Emit(event domain.RealtimeEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		h.log.Error("marshal ws event", "err", err)

		return
	}
	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	h.log.Debug("ws broadcast", "event_type", event.Type, "clients", len(clients))
	for _, c := range clients {
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, body); err != nil {
			h.log.Warn("ws write failed; dropping client", "err", err)
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			_ = c.Close()
		}
	}
}

// ClientCount returns the number of currently connected websocket clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}
