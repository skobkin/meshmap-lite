package mqttclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"meshmap-lite/internal/config"
)

// Handler handles incoming MQTT topic/payload messages.
type Handler func(topic string, payload []byte)

// Client wraps MQTT connection lifecycle and subscription handling.
type Client struct {
	cfg     config.MQTTConfig
	log     *slog.Logger
	handler Handler
	client  mqtt.Client
}

// New constructs an MQTT client wrapper.
func New(cfg config.MQTTConfig, log *slog.Logger, handler Handler) *Client {
	return &Client{cfg: cfg, log: log, handler: handler}
}

// Start connects to MQTT, subscribes to root topic, and blocks until context cancellation.
func (c *Client) Start(ctx context.Context) error {
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.cfg.Host, c.cfg.Port)
	clientID := fmt.Sprintf("meshmap-lite-%d", time.Now().UnixNano())
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetCleanSession(c.cfg.CleanSession).
		SetConnectRetry(true).
		SetConnectRetryInterval(c.cfg.ReconnectTimeout).
		SetAutoReconnect(true).
		SetKeepAlive(c.cfg.Keepalive)
	opts.SetConnectTimeout(c.cfg.ConnectTimeout)
	opts.SetConnectionAttemptHandler(func(broker *url.URL, tlsCfg *tls.Config) *tls.Config {
		c.log.Debug("mqtt connection attempt",
			"broker", broker.String(),
			"client_id", clientID,
			"tls", tlsCfg != nil,
		)

		return tlsCfg
	})
	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
		opts.SetPassword(c.cfg.Password)
	}
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		c.log.Info("mqtt connected", "broker", brokerURL, "client_id", clientID)
		topic := c.cfg.RootTopic + "/#"
		c.log.Debug("mqtt subscribe requested", "topic", topic, "qos", c.cfg.SubscribeQoS)
		if token := client.Subscribe(topic, c.cfg.SubscribeQoS, c.messageHandler); token.Wait() && token.Error() != nil {
			c.log.Error("mqtt subscribe failed", "topic", topic, "err", token.Error())

			return
		}
		c.log.Info("mqtt subscribed", "topic", topic)
	})
	opts.SetReconnectingHandler(func(_ mqtt.Client, opts *mqtt.ClientOptions) {
		c.log.Warn("mqtt reconnecting", "broker", opts.Servers[0].String(), "client_id", opts.ClientID)
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		c.log.Warn("mqtt connection lost", "err", err)
	})
	c.log.Info("mqtt client initializing",
		"broker", brokerURL,
		"client_id", clientID,
		"clean_session", c.cfg.CleanSession,
		"reconnect_timeout", c.cfg.ReconnectTimeout.String(),
		"connect_timeout", c.cfg.ConnectTimeout.String(),
		"keepalive", c.cfg.Keepalive.String(),
	)
	c.client = mqtt.NewClient(opts)
	t := c.client.Connect()
	if !t.WaitTimeout(c.cfg.ConnectTimeout) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if err := t.Error(); err != nil {
		return err
	}
	<-ctx.Done()
	c.log.Info("mqtt client disconnecting")
	c.client.Disconnect(250)
	c.log.Info("mqtt client stopped")

	return nil
}

func (c *Client) messageHandler(_ mqtt.Client, msg mqtt.Message) {
	c.log.Debug("mqtt message received", "topic", msg.Topic(), "bytes", len(msg.Payload()))
	if c.handler != nil {
		c.handler(msg.Topic(), msg.Payload())
	}
}
