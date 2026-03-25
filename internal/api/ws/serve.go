package ws

import (
	"errors"
	"net/http"

	"github.com/gorilla/websocket"

	"meshmap-lite/internal/domain"
)

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("ws upgrade failed", "err", err)

		return
	}

	client := h.register(conn, r)
	defer h.unregister(client)
	if h.opts.OnConnect != nil {
		if err := h.opts.OnConnect(r, func(event domain.RealtimeEvent) error {
			return h.emitToClient(client, event)
		}); err != nil {
			h.log.Warn("ws on-connect hook failed", "remote_addr", client.remoteAddr, "err", err)

			return
		}
	}

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) && !errors.Is(err, websocket.ErrCloseSent) {
				h.log.Debug("ws read failed", "remote_addr", client.remoteAddr, "err", err)
			}

			return
		}
	}
}
