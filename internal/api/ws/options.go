package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const defaultWriteTimeout = 5 * time.Second

// OriginPolicy decides whether a websocket upgrade is allowed.
type OriginPolicy func(*http.Request) bool

// Options contains the websocket settings owned by this package.
type Options struct {
	CheckOrigin  OriginPolicy
	WriteTimeout time.Duration
}

type client struct {
	conn       *websocket.Conn
	remoteAddr string
	userAgent  string
}

func (c *client) close() error {
	return c.conn.Close()
}

func (o Options) withDefaults() Options {
	if o.CheckOrigin == nil {
		o.CheckOrigin = func(_ *http.Request) bool { return true }
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = defaultWriteTimeout
	}

	return o
}
