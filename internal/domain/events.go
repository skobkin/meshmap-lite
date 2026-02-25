package domain

import "time"

// RealtimeEvent is a websocket event envelope sent to UI clients.
type RealtimeEvent struct {
	Type    string      `json:"type"`
	TS      time.Time   `json:"ts"`
	Payload interface{} `json:"payload"`
}
