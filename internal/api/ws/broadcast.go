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
		_ = client.conn.SetWriteDeadline(time.Now().Add(h.opts.WriteTimeout))
		if err := client.conn.WriteMessage(websocket.TextMessage, body); err != nil {
			h.log.Warn("ws write failed; dropping client", "remote_addr", client.remoteAddr, "err", err)
			h.unregister(client)
		}
	}
}
