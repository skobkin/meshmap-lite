package ws

import (
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
)

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("ws upgrade failed", "err", err)

		return
	}

	client := h.register(conn, r)
	defer h.unregister(client)

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) && !errors.Is(err, websocket.ErrCloseSent) {
				h.log.Debug("ws read failed", "remote_addr", client.remoteAddr, "err", err)
			}

			return
		}
	}
}
