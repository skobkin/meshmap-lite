# API Reference

This document is the source of truth for the public HTTP and WebSocket contract exposed by MeshMap Lite.

## HTTP endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/meta`
- `GET /api/v1/channels`
- `GET /api/v1/map/nodes`
- `GET /api/v1/chat/messages?channel=<name>&limit=<n>&before=<cursor>`
- `GET /api/v1/log/events?limit=<n>&before=<id>&channel=<name>&event_kind=<kind>`
- `GET /api/v1/nodes`
- `GET /api/v1/nodes/{node_id}`
- `GET /api/v1/ws`

## Endpoint notes

- `GET /healthz`: returns `200 {"status":"ok"}`.
- `GET /readyz`: returns `200 {"status":"ready"}` when the app is ready, otherwise `503 {"status":"not_ready"}`.
- `GET /api/v1/meta`: returns UI/runtime metadata including `websocket_path`, chat defaults, log settings, and map defaults.
- `GET /api/v1/channels`: returns configured channels as `{name, chat_enabled, is_primary}` items.
- `GET /api/v1/map/nodes`: returns map snapshot items as `{node, position?}`.
- `GET /api/v1/chat/messages`: returns chat history ordered newest-first. `channel` defaults to `meta.default_chat_channel`; `limit` defaults to `meta.show_recent_messages`; `before` paginates by chat row ID.
- `GET /api/v1/log/events`: returns non-chat activity log rows ordered newest-first. `limit` defaults to `meta.log_page_size_default`; `before` paginates by log row ID; `channel` filters by channel name; `event_kind` may be repeated or passed as a comma-separated list. `event_kinds` is also accepted as an alias for compatibility.
  - `details` is event-specific JSON. New traceroute rows use semantic fields such as `role`, `status`, `request_id`, `from`, `to`, `forward_path`, `return_path`, `forward_snr`, `return_snr`, and `inferred_*` markers instead of only hop counts.
  - Correlated traceroute lifecycle rows are emitted as traceroute log events with `details.scope="lifecycle"`. For matched runs, the app stores terminal lifecycle rows instead of separate raw traceroute/routing packet rows. These rows may include lifecycle `status` values such as `partial`, `completed`, `failed`, or `timed_out`, plus `started_at`, `updated_at`, `completed_at`, `source_packets`, and `steps` with intermediate event types/timestamps/packet IDs.
  - Unmatched traceroute replies or routing packets remain visible as raw packet rows when they cannot be correlated to a tracked request.
  - Routing rows may include `request_id`, route arrays, and `traceroute_status="failed"` when a `ROUTING_APP` error packet refers to a traceroute request and `error_reason != "NONE"`.
  - MQTT-derived traceroute paths can be partial compared to a directly connected radio client; missing reply-side data must be treated as absent rather than fabricated.
- `GET /api/v1/nodes`: returns node list items for the Nodes view.
- `GET /api/v1/nodes/{node_id}`: returns `{node, position?, telemetry?}` for one node, or `404 {"error":"not_found"}` if absent.
- `GET /api/v1/ws`: single WebSocket stream for live events.

## WebSocket events

- `chat.message`: chat message payload matching the chat history item shape.
- `chat.system`: system chat payload matching the chat history item shape.
- `node.upsert`: full node payload for identity/liveness updates.
- `node.position`: node position payload for map updates.
- `log.event`: log event payload matching `GET /api/v1/log/events`.
- `stats`: runtime counters `{known_nodes_count, online_nodes_count, ws_clients_count, last_ingest_at?}`.
- `ws.heartbeat`: heartbeat payload `{"status":"ok"}` emitted on the heartbeat interval.

## Log event kind values

- `1`: Map report
- `2`: Node info
- `3`: Position
- `4`: Telemetry
- `5`: Traceroute
- `6`: Neighbor info
- `7`: Routing
- `8`: Other app packet
- `9`: Encrypted (undecryptable)
