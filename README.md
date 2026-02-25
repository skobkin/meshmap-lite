# MeshMap Lite

Lightweight read-only Meshtastic regional map and chat viewer.

## Run locally

1. `go run ./cmd/server --config ./config.example.yaml`
2. Open `http://localhost:8080`

## Frontend dev

1. `cd web`
2. `npm install`
3. `npm run dev`

## Config

- YAML and `MML_` environment variables are supported.
- Env keys use `__` as nesting separator (example: `MML_MQTT__ROOT_TOPIC=msh/RU/ARKH`).
- Channel keys are normalized to lowercase internally.

## API

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/meta`
- `GET /api/v1/channels`
- `GET /api/v1/map/nodes`
- `GET /api/v1/chat/messages?channel=<name>&limit=<n>&before=<cursor>`
- `GET /api/v1/nodes`
- `GET /api/v1/nodes/{node_id}`
- `GET /api/v1/ws`

## Notes

MQTT ingest decodes real Meshtastic protobuf payloads (`ServiceEnvelope`/`MapReport`/`MeshPacket`), with JSON fallback kept for synthetic local tests.
