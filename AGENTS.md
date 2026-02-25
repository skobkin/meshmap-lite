# Repository Guidelines

## Project Structure & Module Organization

### Backend (Go)
- `cmd/server`: application entrypoint (HTTP API, WebSocket, MQTT ingest, static frontend serving).
- `internal/app`: runtime wiring, startup/shutdown orchestration, version/build info.
- `internal/config`: YAML + ENV config loading, validation, defaults.
- `internal/mqttclient`: MQTT connection/subscription/reconnect wrapper.
- `internal/meshtastic`: Meshtastic packet/topic parsing, protobuf decode/decrypt helpers.
- `internal/ingest`: ingest pipeline and event handlers (chat/node info/position/telemetry/map reports).
- `internal/domain`: domain models, merge rules, liveness/status logic.
- `internal/dedup`: packet-id dedup window (KV/FIFO/LRU-like in-memory store).
- `internal/repo`: storage interfaces (DB-agnostic repositories).
- `internal/persistence/sqlite`: SQLite schema, migrations, repository implementations.
- `internal/api/http`: REST API handlers (`/api/v1/...`, `/healthz`, `/readyz`).
- `internal/api/ws`: WebSocket hub, subscriptions, fanout event encoding.
- `internal/frontend`: embedded frontend assets (if embedding is used).
- `internal/meshtasticpb` (or equivalent): generated protobuf bindings from official Meshtastic protobufs.

### Frontend (TypeScript / Preact)
- `web/`: frontend app root (Vite or similar).
- `web/src/pages`: page-level views (`Map`, `Nodes`).
- `web/src/components`: reusable UI components (header, chat, node tooltip, status badges).
- `web/src/stores`: Zustand stores (chat, nodes, websocket, app meta).
- `web/src/api`: HTTP and WebSocket client code, DTOs/event types.
- `web/src/maps`: map adapter layer (Leaflet integration isolated behind interfaces).
- `web/src/utils`: formatting helpers (time, relative time, display names).
- `web/public`: static assets/icons/placeholders.

### Packaging / Ops
- `migrations/` (if separate from `internal/persistence/sqlite/migrations`)
- `Dockerfile`
- `configs/` or `config.example.yaml`
- `dist/` for local build artifacts only (Git-ignored)

---

## Build, Test, and Development Commands

### Backend
- `go build ./...`: build all backend packages/binaries.
- `go test ./...`: run backend unit/integration tests.
- `go run ./cmd/server --config ./config.example.yaml`: run the server locally.
- `go fmt ./...`: format Go code.
- `go vet ./...`: static checks.
- `golangci-lint run ./...`: lint backend code.

### Frontend (from `web/`)
- `npm install` (or project package manager equivalent): install dependencies.
- `npm run dev`: start dev server.
- `npm run build`: production build.
- `npm run typecheck`: TypeScript checks.
- `npm run test` (if configured): frontend tests.
- `npm run lint` (if configured): lint frontend code.

### Docker
- `docker build -t meshtastic-map-lite .`: build runtime image.
- `docker run --rm -p 8080:8080 -v $(pwd)/config.yaml:/app/config.yaml -v $(pwd)/data:/data meshtastic-map-lite`: local container run example (adjust paths as needed).

---

## Coding Style & Naming Conventions

### General
- Prefer small, composable modules with clear ownership.
- Keep dependencies lightweight; avoid introducing frameworks when stdlib/small libs are enough.
- Use graceful degradation: missing/partial data should be shown as missing, not cause crashes.

### Go
- Language: Go (latest stable in `go.mod`).
- Formatting is mandatory: run `gofmt -w` (or `go fmt ./...`) on changed files.
- Package names are short lowercase nouns (`config`, `domain`, `repo`, `ingest`).
- Exported identifiers: `PascalCase`; internal helpers: `camelCase`.
- Use `context.Context` for IO boundaries and cancellation.
- Use structured logging (`slog`) with actionable fields (operation, topic, node id, channel, packet id, error).
- Keep SQLite-specific SQL inside persistence/repository implementation packages only.
- Preserve storage portability: repository/domain APIs must not assume SQLite quirks if PostgreSQL support is planned.
  - If SQLite quirks can improve performance, they should be implemented in a specific SQLite adapter (same goes for PostgreSQL later).

### Meshtastic / Ingest Logic (important)
- Deduplicate by packet ID where applicable (at minimum for chat messages).
- Prefer bounded in-memory dedup window for suppressing duplicate writes/events.
- Do not overwrite telemetry fields with missing/null/empty values from newer packets.
- `0` values are valid telemetry values and must not be treated as empty.
- Track both `observed_at` (server receive time) and `reported_at` (device time) when available.
- Do not mark a node as MQTT-connected just because another gateway relayed its packet; preserve correct liveness semantics.

### Frontend (Preact/TypeScript)
- Prefer functional components and hooks.
- Keep state in small Zustand stores by domain (chat/nodes/ws/meta), not one global monolith.
- Keep Leaflet integration behind a thin adapter boundary so it can be replaced/upgraded later.
- Avoid heavy UI frameworks; stick to PicoCSS + small helpers.
- UI must continue to function with stale snapshot data if WebSocket reconnect is in progress/fails.

---

## Testing Guidelines

### Backend
- Place tests next to code using `*_test.go`.
- Prefer table-driven tests for:
    - topic parsing
    - packet classification
    - dedup logic
    - telemetry merge behavior
    - liveness/status calculations
    - sender display fallback logic
- Use `httptest` for HTTP handler/API contract tests.
- Add regression tests for bug fixes whenever practical.

### Frontend
- Prefer tests for pure logic/stores/formatters over brittle DOM-heavy tests.
- Test WebSocket event reducers/store updates with fixture payloads.
- Keep map/UI interaction tests pragmatic; avoid overly fragile snapshot tests.

### Local Iteration
- Run focused tests while iterating.
- Run full backend + frontend checks before declaring work complete.

---

## API & Realtime Contract Discipline

- Keep REST and WebSocket payloads explicit and versioned under `/api/v1`.
- Do not introduce ad-hoc payload shape changes without updating the typed client DTOs.
- Use one WebSocket stream for all live events (`chat.*`, `node.*`, `stats`, heartbeat).
- Preserve backwards compatibility within the same PR unless the change is intentional and documented.

---

## Configuration Guidelines

- Config supports YAML and ENV; ENV overrides YAML.
- All YAML fields should be representable via ENV with `MML_` prefix and `__` nesting separator.
- Support running with ENV-only config (no file).
- Do not hard-code broker hosts, topics, PSKs, or credentials in code/tests.
- Keep secrets out of the repository (`mqtt.password`, etc.).
- If sample configs are added, use clearly fake values.

### Key config expectations (MVP)
- `mqtt.root_topic` is required.
- `channels` is a map keyed by channel name (not a list).
- At most one `channels.*.primary: true`.
- `map_reports.enabled` and `map_reports.topic_suffix` should be supported.
- `storage.sql.driver=sqlite` is MVP default.
- `storage.kv.driver=memory` is MVP dedup store default.

---

## Performance & Reliability Expectations

- Optimize for small/medium deployments (10–200 nodes typical), without obvious blockers at higher counts.
- Avoid unnecessary DB writes from duplicate packets.
- Keep initial map payload compact.
- WebSocket reconnect must use exponential backoff and visible UI status.
- Backend should remain useful (HTTP snapshot endpoints) even when live updates are degraded.

---

## Completion Checklist

Before finishing work and saying it is done, run the same baseline checks as CI (adjust to what exists in the repo):

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
- If `PLAN.md` exists in the repository root and is relevant to the task, update it before finishing:
    - mark completed task checkboxes
    - and/or update `Current Status` with a short progress summary
- Do not claim work is done if required checks fail.
- If API/WS payloads changed, ensure frontend and backend are updated together.

---

## Commit & Pull Request Guidelines

- Follow Conventional Commits:
    - `feat(api): ...`
    - `feat(ui): ...`
    - `fix(mqtt): ...`
    - `fix(persistence): ...`
    - `chore: ...`
- Keep commits scoped and explain behavioral impact in the subject.
- PRs should include:
    - clear summary of user-visible changes
    - testing performed (commands run)
    - screenshots/GIFs for UI changes (Map/Nodes/chat/sidebar/header states)
    - sample API payloads when REST/WS contract changes are introduced
- If a PR changes DB schema, include migrations in the same PR.
- Mention migration/manual actions only when operators must do something explicitly.

---

## Data Paths & Runtime Files

- In Docker deployments, expect persistent data under `/data` (SQLite DB, future tile cache).
- Default SQLite URL should point to a persistent path (e.g. `/data/db.sqlite`) unless running tests/dev with in-memory SQLite.
- Runtime logs should go to stdout/stderr by default (container-friendly); file logging is optional.
- Do not assume writable project source directories at runtime.

---

## When in Doubt

- Prefer simpler architecture over speculative abstraction, unless the abstraction directly protects a confirmed future requirement (e.g. SQLite -> PostgreSQL portability, map renderer adapter boundary).
- Call out weak technical decisions early and propose concrete alternatives.
- Keep the MVP read-only and lightweight; do not smuggle in non-goal features.