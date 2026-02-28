package mqttclient

import (
	"context"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Start connects to MQTT, subscribes to the root topic, and blocks until context cancellation.
func (c *Client) Start(ctx context.Context) error {
	brokerURL := c.brokerURL()
	clientID := resolveClientID(c.opts)
	c.log.Info("mqtt client initializing",
		"broker", brokerURL,
		"client_id", clientID,
		"clean_session", c.opts.CleanSession,
		"reconnect_timeout", c.opts.ReconnectTimeout.String(),
		"connect_timeout", c.opts.ConnectTimeout.String(),
		"keepalive", c.opts.Keepalive.String(),
	)

	c.client = mqtt.NewClient(c.newClientOptions())
	token := c.client.Connect()
	if !token.WaitTimeout(c.opts.ConnectTimeout) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if err := token.Error(); err != nil {
		return err
	}

	<-ctx.Done()
	c.log.Info("mqtt client disconnecting")
	c.client.Disconnect(defaultDisconnectQuiesceMS)
	c.log.Info("mqtt client stopped")

	return nil
}
