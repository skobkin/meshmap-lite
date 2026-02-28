# Repository Guidelines

## Project Structure

- Backend entrypoint: `cmd/server`
- Runtime wiring: `internal/app`
- Config loading/validation: `internal/config`
- MQTT client and reconnect logic: `internal/mqttclient`
- Meshtastic parsing/decode helpers: `internal/meshtastic`
- Ingest pipeline: `internal/ingest`
- Domain logic and merge/liveness rules: `internal/domain`
- Dedup window: `internal/dedup`
- Repository interfaces: `internal/repo`
- SQLite persistence and migrations: `internal/persistence/sqlite`
- HTTP API: `internal/api/http`
- WebSocket hub/events: `internal/api/ws`
- Generated protobuf bindings: `internal/meshtasticpb`
- Frontend app: `web`
- Frontend pages/components/stores/api/maps/utils: `web/src/...`

## Working Rules

- Prefer small, composable modules with clear ownership.
- Keep dependencies lightweight; avoid introducing frameworks where the standard library or small libraries are enough.
- Use graceful degradation: partial or missing data should render as missing (or hidden), not crash the app.
- Do not assume writable project source directories at runtime.

## Backend Conventions

- Use `context.Context` on IO boundaries.
- Use structured logging with actionable fields such as operation, topic, channel, node id, packet id, and error.
- Keep SQLite-specific SQL inside SQLite persistence packages.
- Preserve storage portability in repository/domain APIs; isolate backend-specific optimizations in adapter implementations.

## Meshtastic / Ingest Rules

- Deduplicate by packet ID where applicable, at minimum for chat messages.
- Prefer a bounded in-memory dedup window to suppress duplicate writes/events.
- Do not overwrite telemetry fields with missing/null/empty values from newer packets.
- Treat `0` telemetry values as valid data.
- Track both `observed_at` and `reported_at` when available.
- Do not mark a node as MQTT-connected just because another gateway relayed its packet.

## Frontend Conventions

- Prefer functional components and hooks.
- Keep Zustand state split by domain (`chat`, `nodes`, `ws`, `meta`) rather than one global store.
- Keep Leaflet behind a thin adapter boundary.
- Avoid heavy UI frameworks; prefer PicoCSS plus small helpers.
- The UI must remain usable with stale snapshot data while WebSocket reconnect is in progress or fails.

## Testing Guidance

- Place backend tests next to code using `*_test.go`.
- Prefer table-driven backend tests for topic parsing, packet classification, dedup behavior, telemetry merges, liveness/status logic, and sender display fallbacks.
- Use `httptest` for HTTP/API contract tests.
- Add regression tests for bug fixes when practical.
- Prefer frontend tests for pure logic, stores, reducers, and formatters over brittle DOM-heavy tests.
- Keep map/UI interaction tests pragmatic.
- Run focused tests while iterating, then full checks before finishing.

## API / Realtime Discipline

- Keep REST and WebSocket payloads explicit and versioned under `/api/v1`.
- Do not change payload shapes ad hoc without updating typed frontend DTOs.
- Keep `docs/api.md` in sync with handler behavior, query params, payload shapes, and event types.
- Use one WebSocket stream for live events (`chat.*`, `node.*`, `stats`, heartbeat).
- Preserve backwards compatibility within a change unless the break is intentional and documented.

## Configuration Discipline

- Config supports YAML and ENV; ENV overrides YAML.
- YAML fields should be representable via ENV with the `MML_` prefix and `__` nesting separator.
- Any config schema change must update `README.md` and `config.example.yaml` in the same change.
- Support ENV-only configuration.
- Do not hard-code broker hosts, topics, PSKs, or credentials in code/tests.
- Keep secrets out of the repository.
- Use clearly fake values in sample config.
- Key expectations:
  - `mqtt.root_topic` is required.
  - `channels` is a map keyed by channel name, not a list.
  - At most one `channels.*.primary: true`.
  - `storage.sql.driver=sqlite` is the default.
  - `storage.kv.driver=memory` is the default dedup store.

## Performance / Reliability

- Optimize for small/medium deployments without obvious blockers at higher counts.
- Avoid unnecessary DB writes from duplicate packets.
- Keep the initial map payload compact.
- WebSocket reconnect must use exponential backoff and visible UI status.
- HTTP snapshot endpoints should remain useful even when live updates are degraded.

## Completion Checklist

Before considering work finished, run the same baseline checks as CI, adjusted to what exists in the repo.

### Backend

- `go fmt ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `go test ./...`

### Frontend (if changed)

- `cd web && npm run typecheck`
- `cd web && npm run lint` (if configured)
- `cd web && npm run test` (if configured)
- `cd web && npm run build`

### Project hygiene

- If `PLAN.md` exists and is relevant to the task, update it before finishing.
- Do not claim work is done if required checks fail.
- If API/WS payloads changed, update backend and frontend together.
- If API routes, query params, WebSocket events, or payload shapes changed, update `docs/api.md` in the same change.

## Commits

- The repository uses Conventional Commits. Keep commit subjects scoped and check recent history for local examples.
- If a change updates the DB schema, include migrations in the same change.
- Mention migration or manual operator actions only when they are actually required.

## When in Doubt

- Prefer simpler architecture over speculative abstraction unless the abstraction directly protects a confirmed requirement.
- If repository conventions or product intent are unclear, ask the user instead of guessing.
- Call out weak technical decisions early and propose concrete alternatives.
