package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"

	"meshmap-lite/internal/domain"
)

// Emit broadcasts a realtime event to all connected clients.
func (h *Hub) Emit(event domain.RealtimeEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		h.log.Error("marshal ws event", "err", err)

		return
	}

	clients := h.snapshotClients()
	h.log.Debug("ws broadcast", "event_type", event.Type, "clients", len(clients))
	for _, client := range clients {
		if err := client.write(body, h.opts.WriteTimeout); err != nil {
			h.log.Warn("ws write failed; dropping client", "remote_addr", client.remoteAddr, "err", err)
			h.unregister(client)
		}
	}
}

func (c *client) write(body []byte, timeout time.Duration) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(timeout))

	return c.conn.WriteMessage(websocket.TextMessage, body)
}
