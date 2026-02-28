// Package mqttclient owns MQTT connection lifecycle, reconnect handling, and
// subscription wiring for the ingest pipeline.
//
// The client uses a stable client ID by default so persistent sessions survive
// process restarts unless the caller opts into a different ID or clean session.
package mqttclient
