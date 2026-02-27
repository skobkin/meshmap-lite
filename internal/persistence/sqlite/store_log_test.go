package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/domain"
)

func TestListLogEvents_WithFiltersAndDisplayName(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, "file::memory:?cache=shared", true, nil)
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

	if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
		ObservedAt: now,
		NodeID:     "!11111111",
		EventKind:  domain.LogEventKindPositionValue,
		Encrypted:  true,
		Channel:    "LongFast",
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

	items, err := s.ListLogEvents(ctx, domain.LogEventQuery{Limit: 50, Channel: "longfast"})
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
}

func TestInsertLogEvent_PrunesByMaxRows(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, "file::memory:?cache=shared", true, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetLogMaxRows(2)

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
	s, err := Open(ctx, "file::memory:?cache=shared", true, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetLogMaxRows(2)
	s.SetLogPruneBatchRows(2)

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
