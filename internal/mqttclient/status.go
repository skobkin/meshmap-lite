package mqttclient

import "sync/atomic"

// ConnectionStatus is the simplified MQTT connectivity state exposed to the UI.
type ConnectionStatus string

const (
	// ConnectionStatusConnected means the app is currently connected to the broker.
	ConnectionStatusConnected ConnectionStatus = "connected"
	// ConnectionStatusDisconnected covers all non-connected MQTT lifecycle states.
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
)

type lifecycleState uint32

const (
	lifecycleStateDisconnected lifecycleState = iota
	lifecycleStateConnecting
	lifecycleStateConnected
	lifecycleStateReconnecting
)

func (s lifecycleState) frontendStatus() ConnectionStatus {
	if s == lifecycleStateConnected {
		return ConnectionStatusConnected
	}

	return ConnectionStatusDisconnected
}

func (c *Client) setLifecycleState(state lifecycleState) {
	c.state.Store(uint32(state))
}

func (c *Client) lifecycleState() lifecycleState {
	return lifecycleState(c.state.Load())
}

// ConnectionStatus returns the simplified connectivity state for the MQTT broker.
func (c *Client) ConnectionStatus() ConnectionStatus {
	return c.lifecycleState().frontendStatus()
}

type atomicLifecycleState struct {
	value atomic.Uint32
}

func (s *atomicLifecycleState) Store(state uint32) {
	s.value.Store(state)
}

func (s *atomicLifecycleState) Load() uint32 {
	return s.value.Load()
}
