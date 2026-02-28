package mqttclient

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func (c *Client) onConnectHandler(brokerURL, clientID string) mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		c.log.Info("mqtt connected", "broker", brokerURL, "client_id", clientID)
		topic := c.subscriptionTopic()
		c.log.Debug("mqtt subscribe requested", "topic", topic, "qos", c.opts.SubscribeQoS)
		if token := client.Subscribe(topic, c.opts.SubscribeQoS, c.messageHandler); token.Wait() && token.Error() != nil {
			c.log.Error("mqtt subscribe failed", "topic", topic, "err", token.Error())

			return
		}
		c.log.Info("mqtt subscribed", "topic", topic)
	}
}

func (c *Client) reconnectingHandler() mqtt.ReconnectHandler {
	return func(_ mqtt.Client, opts *mqtt.ClientOptions) {
		c.log.Warn("mqtt reconnecting", "broker", opts.Servers[0].String(), "client_id", opts.ClientID)
	}
}

func (c *Client) connectionLostHandler() mqtt.ConnectionLostHandler {
	return func(_ mqtt.Client, err error) {
		c.log.Warn("mqtt connection lost", "err", err)
	}
}

func (c *Client) messageHandler(_ mqtt.Client, msg mqtt.Message) {
	c.log.Debug("mqtt message received", "topic", msg.Topic(), "bytes", len(msg.Payload()))
	if c.handler != nil {
		c.handler(msg.Topic(), msg.Payload())
	}
}
