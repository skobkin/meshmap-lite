package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"meshmap-lite/internal/domain"
)

const defaultWriteTimeout = 5 * time.Second

// OriginPolicy decides whether a websocket upgrade is allowed.
type OriginPolicy func(*http.Request) bool

// ConnectHandler is called once after a websocket client is registered.
type ConnectHandler func(*http.Request, func(domain.RealtimeEvent) error) error

// Options contains the websocket settings owned by this package.
type Options struct {
	CheckOrigin  OriginPolicy
	OnConnect    ConnectHandler
	WriteTimeout time.Duration
}

type client struct {
	conn       *websocket.Conn
	remoteAddr string
	userAgent  string
	writeMu    sync.Mutex
	closeOnce  sync.Once
}

func (c *client) close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.conn.Close()
	})

	return err
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
