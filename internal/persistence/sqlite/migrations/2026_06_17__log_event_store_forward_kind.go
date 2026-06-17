package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	generated "meshmap-lite/internal/meshtasticpb"
)

// legacyStoreForwardDetails captures the v17 shape of an S&F details
// payload as it was emitted by the very first cut of the S&F code
// path. v18 walks existing kind=12 rows and rewrites them in the
// compact shape (integer `rr`, no `role`, `text` replaced by
// `text_bytes`).
//
// `rr` is typed as `json.RawMessage` rather than `string` so the
// migration can also tolerate the mixed shape a row may end up in
// when an operator (or an in-flight code path) writes an integer
// `rr` but leaves a stale `role` key behind. The v17 path always
// emitted a string `rr`, but the v18 detector flags role-bearing
// rows as legacy regardless of the `rr` type, so the decode side
// has to accept both.
type legacyStoreForwardDetails struct {
	RR         json.RawMessage `json:"rr"`
	Role       string          `json:"role"`
	FromNodeID string          `json:"from"`
	ToNodeID   string          `json:"to"`
	Stats      json.RawMessage `json:"stats"`
	History    json.RawMessage `json:"history"`
	Heartbeat  json.RawMessage `json:"heartbeat"`
	Text       string          `json:"text"`
}

// newStoreForwardDetails is the v18 shape: RR is an int32, role is
// derived on the fly, and the text body is replaced by its byte
// count. raw_rr / raw_role carry unknown enum values through so a
// newer firmware does not lose data when it ships a code the pinned
// proto has not seen yet.
type newStoreForwardDetails struct {
	RR         *int32          `json:"rr,omitempty"`
	RawRR      string          `json:"raw_rr,omitempty"`
	FromNodeID string          `json:"from,omitempty"`
	ToNodeID   string          `json:"to,omitempty"`
	Stats      json.RawMessage `json:"stats,omitempty"`
	History    json.RawMessage `json:"history,omitempty"`
	Heartbeat  json.RawMessage `json:"heartbeat,omitempty"`
	TextBytes  *int            `json:"text_bytes,omitempty"`
}

// migrateV18LogEventStoreForwardKind recreates log_events with an
// extended CHECK bound, reclassifies S&F `event_kind = 8` rows to the
// new `event_kind = 12`, and rewrites the details_json of every
// kind=12 row into the compact shape. The SQL portion is bulk; the
// per-row JSON rewrite runs after the table swap, in the same
// transaction.
func migrateV18LogEventStoreForwardKind(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExistsTx(ctx, tx, "log_events")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	if err := applyStatements(ctx, tx, "log_event_store_forward_kind", []string{
		`CREATE TABLE log_events_v18 (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  observed_at TEXT NOT NULL,
  node_id TEXT,
  event_kind INTEGER NOT NULL,
  encrypted INTEGER NOT NULL,
  channel_id INTEGER REFERENCES log_channels(id) ON DELETE SET NULL,
  details_json TEXT,
  mqtt_uploader_node_id TEXT,
  hop_start INTEGER,
  hop_limit INTEGER,
  -- event_kind values:
  -- 1 map_report
  -- 2 node_info
  -- 3 position
  -- 4 telemetry
  -- 5 traceroute
  -- 6 neighbor_info
  -- 7 routing
  -- 8 other_portnum
  -- 9 unknown_encrypted
  -- 10 range_test
  -- 11 pki
  -- 12 store_forward
  CHECK (event_kind BETWEEN 1 AND 12),
  CHECK (encrypted IN (0, 1)),
  CHECK (details_json IS NULL OR json_valid(details_json))
);`,
		`INSERT INTO log_events_v18(
  id, observed_at, node_id, event_kind, encrypted, channel_id, details_json,
  mqtt_uploader_node_id, hop_start, hop_limit
)
SELECT
  id,
  observed_at,
  node_id,
  CASE
    WHEN event_kind = 8 AND (
      json_extract(details_json, '$.portnum_name') = 'STORE_FORWARD_APP' OR
      json_extract(details_json, '$.portnum_value') = 65
    ) THEN 12
    ELSE event_kind
  END,
  encrypted,
  channel_id,
  CASE
    WHEN event_kind = 8 AND (
      json_extract(details_json, '$.portnum_name') = 'STORE_FORWARD_APP' OR
      json_extract(details_json, '$.portnum_value') = 65
    ) THEN NULL
    ELSE details_json
  END,
  mqtt_uploader_node_id,
  hop_start,
  hop_limit
FROM log_events;`,
		`DROP TABLE log_events;`,
		`ALTER TABLE log_events_v18 RENAME TO log_events;`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_id ON log_events(id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_kind_id ON log_events(event_kind, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_kind_observed_at ON log_events(event_kind, observed_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_channel_id ON log_events(channel_id, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_node_id ON log_events(node_id, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_log_events_mqtt_uploader_node_id ON log_events(mqtt_uploader_node_id);`,
		`UPDATE sqlite_sequence SET seq = COALESCE((SELECT MAX(id) FROM log_events), 0) WHERE name = 'log_events';`,
	}); err != nil {
		return err
	}

	return compactStoreForwardDetailsInPlace(ctx, tx)
}

// compactStoreForwardDetailsInPlace walks every S&F row in log_events
// and rewrites its details_json into the v18 shape. Rows that have
// no details_json (the typical case after the kind=8 → kind=12
// reclassification nulled them) are left alone. Rows that are
// already in the v18 shape pass through unchanged so the migration
// is idempotent.
func compactStoreForwardDetailsInPlace(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, details_json FROM log_events WHERE event_kind = 12 AND details_json IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("scan s&f rows: %w", err)
	}

	type pending struct {
		id      int64
		details string
	}
	var toUpdate []pending

	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan s&f row %d: %w", id, err)
		}

		rewritten, err := rewriteStoreForwardDetails(raw)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("rewrite s&f row %d: %w", id, err)
		}
		if rewritten == raw {
			continue
		}
		toUpdate = append(toUpdate, pending{id: id, details: rewritten})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate s&f rows: %w", err)
	}
	_ = rows.Close()

	for _, p := range toUpdate {
		if _, err := tx.ExecContext(ctx,
			`UPDATE log_events SET details_json = ? WHERE id = ?`,
			p.details, p.id,
		); err != nil {
			return fmt.Errorf("update s&f row %d: %w", p.id, err)
		}
	}

	return nil
}

// rewriteStoreForwardDetails converts a single legacy details blob
// into the v18 shape. If the input is already in the v18 shape, it
// is returned unchanged so the migration is idempotent.
func rewriteStoreForwardDetails(raw string) (string, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		// Not a JSON object — leave it alone for the generic fallback
		// renderer to display.
		return raw, nil
	}

	// New shape detection: the legacy v17 code path emitted `rr` as
	// a JSON string (the proto enum name) and stored a `role` key.
	// The v18 code path emits `rr` as a JSON number (the proto enum
	// value) and never stores a `role`. Treat a string-typed `rr`
	// (or any `role` key) as the legacy shape; everything else is
	// new shape and passes through unchanged for idempotency.
	if isLegacyStoreForwardShape(probe) {
		return compactLegacyStoreForwardDetails(raw)
	}

	return raw, nil
}

func isLegacyStoreForwardShape(probe map[string]json.RawMessage) bool {
	if _, hasRole := probe["role"]; hasRole {
		return true
	}
	if rrRaw, ok := probe["rr"]; ok {
		var s string
		if err := json.Unmarshal(rrRaw, &s); err == nil {
			// rr decodes to a string — that's the legacy encoding.
			return true
		}
	}

	return false
}

func compactLegacyStoreForwardDetails(raw string) (string, error) {
	var legacy legacyStoreForwardDetails
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return "", fmt.Errorf("decode legacy shape: %w", err)
	}

	out := newStoreForwardDetails{
		FromNodeID: legacy.FromNodeID,
		ToNodeID:   legacy.ToNodeID,
		Stats:      nonEmptyRaw(legacy.Stats),
		History:    nonEmptyRaw(legacy.History),
		Heartbeat:  nonEmptyRaw(legacy.Heartbeat),
	}

	// Resolve rr. The v17 path always emitted a string `rr`, but a
	// row may have ended up in a mixed shape (int `rr` + stale
	// `role` key) — e.g. an operator hand-edited the database, or
	// an in-flight code path wrote a numeric rr but left the
	// legacy role key. Accept either type and preserve the int
	// verbatim. A genuinely missing or unparseable rr is still an
	// error: there is nothing to migrate.
	rrValue, rawRR, err := resolveLegacyRR(legacy.RR)
	if err != nil {
		return "", err
	}
	if rawRR != "" {
		// Forward-compat: a legacy row with a code the pinned proto
		// does not know about. Mirror the Go-side decoder's behaviour
		// and preserve the original string in raw_rr with rr = -1
		// so we never silently lose data when a newer firmware has
		// shipped a value the proto has not caught up to.
		sentinel := int32(-1)
		out.RR = &sentinel
		out.RawRR = rawRR
	} else {
		out.RR = &rrValue
	}
	if legacy.Text != "" {
		n := len(legacy.Text)
		out.TextBytes = &n
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode new shape: %w", err)
	}
	return string(encoded), nil
}

// mapLegacyRR looks up a legacy string-typed `rr` value in the pinned
// proto. For known values it returns the numeric enum code. For
// values the proto has not been updated for, it returns the -1
// sentinel plus the original string so the caller can surface it as
// raw_rr. A genuinely empty `rr` is treated as an error because there
// is nothing to preserve.
func mapLegacyRR(rr string) (int32, string, error) {
	if rr == "" {
		return 0, "", fmt.Errorf("missing rr in legacy s&f row")
	}
	if v, ok := generated.StoreAndForward_RequestResponse_value[rr]; ok {
		return v, "", nil
	}
	return -1, rr, nil
}

// resolveLegacyRR accepts the on-disk encoding of `rr` in a legacy
// row and returns the value to persist in the v18 shape. Three
// shapes are tolerated: (a) a string, looked up against the pinned
// proto — known values become the numeric enum code, unknown
// values become the -1 sentinel + the original string in raw_rr;
// (b) a number, preserved verbatim (the row was already partially
// migrated — a stale `role` key is the only legacy bit left); (c)
// missing or unparseable, which is a loud-failure because there is
// nothing meaningful to preserve.
func resolveLegacyRR(raw json.RawMessage) (int32, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return mapLegacyRR("")
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return mapLegacyRR(asString)
	}
	var asInt int32
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt, "", nil
	}

	return 0, "", fmt.Errorf("rr in legacy s&f row is neither string nor int: %s", string(raw))
}

func nonEmptyRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	// Treat JSON "null" and empty objects/arrays as absent to keep
	// stored details compact.
	s := string(raw)
	if s == "null" || s == "{}" || s == "[]" {
		return nil
	}

	return raw
}
