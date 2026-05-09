package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
)

func TestActivityBuckets_AggregatesCompleteBucketsAndBoundaries(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	start := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	insertChat := func(observedAt time.Time) {
		t.Helper()
		if _, err := s.InsertChatEvent(ctx, domain.ChatEvent{
			EventType:   domain.ChatEventMessage,
			ChannelName: "LongFast",
			NodeID:      "!chat",
			MessageText: "hello",
			MessageTime: observedAt,
			ObservedAt:  observedAt,
			CreatedAt:   observedAt,
		}); err != nil {
			t.Fatalf("insert chat: %v", err)
		}
	}
	insertLog := func(observedAt time.Time, kind domain.LogEventKind) {
		t.Helper()
		if _, err := s.InsertLogEvent(ctx, domain.LogEvent{
			ObservedAt: observedAt,
			NodeID:     "!log",
			EventKind:  kind,
			Channel:    "LongFast",
		}); err != nil {
			t.Fatalf("insert log %d: %v", kind, err)
		}
	}

	insertChat(start)
	insertChat(start.Add(2 * time.Minute))
	insertLog(start.Add(5*time.Minute), domain.LogEventKindPKIValue)
	insertLog(start.Add(5*time.Minute), domain.LogEventKindNodeInfoValue)
	insertLog(start.Add(7*time.Minute), domain.LogEventKindTelemetryValue)
	insertLog(start.Add(10*time.Minute), domain.LogEventKindNeighborInfoValue)
	insertLog(start.Add(14*time.Minute), domain.LogEventKindRangeTestValue)
	insertLog(start.Add(14*time.Minute), domain.LogEventKindTracerouteValue)
	insertChat(start.Add(15 * time.Minute))

	buckets, err := s.ActivityBuckets(ctx, domain.ActivityQuery{
		Start:  start,
		End:    start.Add(15 * time.Minute),
		Bucket: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("activity buckets: %v", err)
	}
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	if buckets[0].TextMessages != 2 {
		t.Fatalf("expected first bucket text count 2, got %d", buckets[0].TextMessages)
	}
	if buckets[1].PKI != 1 || buckets[1].NodeInfo != 1 || buckets[1].Telemetry != 1 {
		t.Fatalf("unexpected second bucket counts: %+v", buckets[1])
	}
	if buckets[2].NeighborInfo != 1 || buckets[2].RangeTest != 1 || buckets[2].Traceroute != 1 {
		t.Fatalf("unexpected third bucket counts: %+v", buckets[2])
	}
	if buckets[2].TextMessages != 0 {
		t.Fatalf("end boundary chat should be excluded, got %d", buckets[2].TextMessages)
	}
}
