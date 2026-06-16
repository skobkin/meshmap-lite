package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
)

func TestListLogEvents_WithFiltersAndDisplayName(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!11111111",
		LongName:           "Alpha",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!99999999",
		LongName:           "Gateway",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert gateway node: %v", err)
	}

	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt:         now,
		NodeID:             "!11111111",
		MQTTUploaderNodeID: "!99999999",
		EventKind:          domain.LogEventKindPositionValue,
		Encrypted:          true,
		Channel:            "LongFast",
	}); err != nil {
		t.Fatalf("insert log event #1: %v", err)
	}
	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt: now.Add(1 * time.Second),
		NodeID:     "!11111111",
		EventKind:  domain.LogEventKindTelemetryValue,
		Encrypted:  false,
		Channel:    "PingPong",
	}); err != nil {
		t.Fatalf("insert log event #2: %v", err)
	}
	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt: now.Add(2 * time.Second),
		NodeID:     "!22222222",
		EventKind:  domain.LogEventKindPositionValue,
		Encrypted:  false,
		Channel:    "LongFast",
	}); err != nil {
		t.Fatalf("insert log event #3: %v", err)
	}

	items, err := s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50, Channel: "longfast", NodeID: "!11111111"})
	if err != nil {
		t.Fatalf("list log events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(items))
	}
	if items[0].EventKindValue != domain.LogEventKindPositionValue {
		t.Fatalf("unexpected kind: %d", items[0].EventKindValue)
	}
	if items[0].NodeDisplay != "Alpha" {
		t.Fatalf("expected node display from nodes table, got %q", items[0].NodeDisplay)
	}
	if items[0].MQTTUploaderNodeID != "!99999999" || items[0].MQTTUploaderDisplayName != "Gateway" {
		t.Fatalf("expected uploader display from nodes table, got %#v", items[0])
	}
}

func TestInsertLogEvent_CachesChannelIDs(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
			ObservedAt: now.Add(time.Duration(i) * time.Second),
			NodeID:     "!cccc3333",
			EventKind:  domain.LogEventKindTelemetryValue,
			Channel:    "LongFast",
		}); err != nil {
			t.Fatalf("insert log event #%d: %v", i+1, err)
		}
	}

	var channels int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM log_channels`).Scan(&channels); err != nil {
		t.Fatalf("count log channels: %v", err)
	}
	if channels != 1 {
		t.Fatalf("expected exactly one log channel row, got %d", channels)
	}
	if len(s.logChannelIDs) != 1 {
		t.Fatalf("expected one cached log channel id, got %d", len(s.logChannelIDs))
	}
}

func TestInsertLogEvent_PrunesByMaxRows(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{
		URL:         "file::memory:?cache=shared",
		AutoMigrate: true,
		LogMaxRows:  2,
	}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
			ObservedAt: now.Add(time.Duration(i) * time.Second),
			NodeID:     "!22222222",
			EventKind:  domain.LogEventKindTelemetryValue,
			Encrypted:  false,
			Channel:    "LongFast",
		}); err != nil {
			t.Fatalf("insert log event #%d: %v", i+1, err)
		}
	}

	items, err := s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50})
	if err != nil {
		t.Fatalf("list log events: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 events after prune, got %d", len(items))
	}
}

func TestInsertLogEvent_PrunesInBatches(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{
		URL:               "file::memory:?cache=shared",
		AutoMigrate:       true,
		LogMaxRows:        2,
		LogPruneBatchRows: 2,
	}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
			ObservedAt: now.Add(time.Duration(i) * time.Second),
			NodeID:     "!33333333",
			EventKind:  domain.LogEventKindTelemetryValue,
			Encrypted:  false,
			Channel:    "LongFast",
		}); err != nil {
			t.Fatalf("insert log event #%d: %v", i+1, err)
		}
	}

	items, err := s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50})
	if err != nil {
		t.Fatalf("list log events before batch prune: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected no prune before crossing cap+batch, got %d rows", len(items))
	}

	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt: now.Add(4 * time.Second),
		NodeID:     "!33333333",
		EventKind:  domain.LogEventKindTelemetryValue,
		Encrypted:  false,
		Channel:    "LongFast",
	}); err != nil {
		t.Fatalf("insert log event #5: %v", err)
	}

	items, err = s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50})
	if err != nil {
		t.Fatalf("list log events after batch prune: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected prune down to max rows, got %d rows", len(items))
	}
}

func TestLogEventHopMetadataRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	hopStart := uint32(5)
	hopLimit := uint32(2)
	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt: now,
		NodeID:     "!abcdef01",
		EventKind:  domain.LogEventKindTelemetryValue,
		Encrypted:  false,
		Channel:    "LongFast",
		HopStart:   &hopStart,
		HopLimit:   &hopLimit,
	}); err != nil {
		t.Fatalf("insert log event: %v", err)
	}

	items, err := s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50})
	if err != nil {
		t.Fatalf("list log events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 log event, got %d", len(items))
	}
	if items[0].HopStart == nil || *items[0].HopStart != 5 {
		t.Fatalf("expected HopStart=5, got %#v", items[0].HopStart)
	}
	if items[0].HopLimit == nil || *items[0].HopLimit != 2 {
		t.Fatalf("expected HopLimit=2, got %#v", items[0].HopLimit)
	}

	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt: now.Add(time.Second),
		NodeID:     "!abcdef02",
		EventKind:  domain.LogEventKindTelemetryValue,
		Encrypted:  false,
		Channel:    "LongFast",
	}); err != nil {
		t.Fatalf("insert log event without hop metadata: %v", err)
	}

	items, err = s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50})
	if err != nil {
		t.Fatalf("list log events: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 log events, got %d", len(items))
	}
	for _, item := range items {
		if item.NodeID == "!abcdef02" {
			if item.HopStart != nil {
				t.Fatalf("expected nil HopStart when omitted, got %#v", *item.HopStart)
			}
			if item.HopLimit != nil {
				t.Fatalf("expected nil HopLimit when omitted, got %#v", *item.HopLimit)
			}
		}
	}
}

func TestListLogEventsFiltersByTraversedHops(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	insert := func(name, nodeID, uploaderID string, hopStart, hopLimit *uint32) {
		t.Helper()
		if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
			ObservedAt:         now.Add(time.Duration(len(name)) * time.Second),
			NodeID:             nodeID,
			MQTTUploaderNodeID: uploaderID,
			EventKind:          domain.LogEventKindTelemetryValue,
			Encrypted:          false,
			Channel:            "LongFast",
			Details:            map[string]any{"name": name},
			HopStart:           hopStart,
			HopLimit:           hopLimit,
		}); err != nil {
			t.Fatalf("insert %s: %v", name, err)
		}
	}

	insert("zero", "!zero", "!gateway", uint32Ptr(7), uint32Ptr(7))
	insert("three", "!three", "!gateway", uint32Ptr(5), uint32Ptr(2))
	insert("exhausted", "!exhausted", "!gateway", uint32Ptr(5), uint32Ptr(0))
	insert("missing", "!missing", "!gateway", nil, nil)
	insert("self", "!self", "!self", uint32Ptr(7), uint32Ptr(7))

	items, err := s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50, HopsMin: intPtr(0), HopsMax: intPtr(3)})
	if err != nil {
		t.Fatalf("list log events: %v", err)
	}
	if got := logEventNames(items); len(got) != 2 || got[0] != "three" || got[1] != "zero" {
		t.Fatalf("expected three and zero hop events, got %#v", got)
	}

	items, err = s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50, HopsMin: intPtr(5)})
	if err != nil {
		t.Fatalf("list log events: %v", err)
	}
	if got := logEventNames(items); len(got) != 1 || got[0] != "exhausted" {
		t.Fatalf("expected exhausted hop event, got %#v", got)
	}
}

func intPtr(v int) *int {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func logEventNames(items []domain.LogEventView) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if name, ok := item.Details["name"].(string); ok {
			out = append(out, name)
		}
	}

	return out
}
