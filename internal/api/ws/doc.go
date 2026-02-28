// Package ws provides the broadcast-only realtime websocket stream.
//
// Inbound client messages are ignored. The hub upgrades HTTP requests,
// registers clients, and broadcasts server-originated events to them.
package ws
