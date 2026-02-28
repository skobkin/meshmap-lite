package mqttclient

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	defaultDisconnectQuiesceMS = 250
	clientIDHostnamePrefixLen  = 10
	maxClientIDLength          = 23
	fallbackClientID           = "mml-client"
	fallbackClientHostname     = "host"
)

// Handler handles incoming MQTT topic/payload messages.
type Handler func(topic string, payload []byte)

// Options contains the MQTT settings owned by this package.
type Options struct {
	Host             string
	Port             int
	TLS              bool
	ClientID         string
	Username         string
	Password         string
	RootTopic        string
	SubscribeQoS     byte
	CleanSession     bool
	ReconnectTimeout time.Duration
	ConnectTimeout   time.Duration
	Keepalive        time.Duration
}

// Client wraps MQTT connection lifecycle and subscription handling.
type Client struct {
	opts    Options
	log     *slog.Logger
	handler Handler
	client  mqtt.Client
}

// New constructs an MQTT client wrapper.
func New(opts Options, log *slog.Logger, handler Handler) *Client {
	return &Client{opts: opts, log: log, handler: handler}
}

func (c *Client) newClientOptions() *mqtt.ClientOptions {
	brokerURL := c.brokerURL()
	clientID := resolveClientID(c.opts)
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetCleanSession(c.opts.CleanSession).
		SetConnectRetry(true).
		SetConnectRetryInterval(c.opts.ReconnectTimeout).
		SetAutoReconnect(true).
		SetKeepAlive(c.opts.Keepalive)
	opts.SetConnectTimeout(c.opts.ConnectTimeout)
	opts.SetConnectionAttemptHandler(func(broker *url.URL, tlsCfg *tls.Config) *tls.Config {
		c.log.Debug("mqtt connection attempt",
			"broker", broker.String(),
			"client_id", clientID,
			"tls", tlsCfg != nil,
		)

		return tlsCfg
	})
	if c.opts.Username != "" {
		opts.SetUsername(c.opts.Username)
		opts.SetPassword(c.opts.Password)
	}
	opts.SetOnConnectHandler(c.onConnectHandler(brokerURL, clientID))
	opts.SetReconnectingHandler(c.reconnectingHandler())
	opts.SetConnectionLostHandler(c.connectionLostHandler())

	return opts
}

func (c *Client) brokerURL() string {
	scheme := "tcp"
	if c.opts.TLS {
		scheme = "ssl"
	}

	return fmt.Sprintf("%s://%s:%d", scheme, c.opts.Host, c.opts.Port)
}

func (c *Client) subscriptionTopic() string {
	return c.opts.RootTopic + "/#"
}
