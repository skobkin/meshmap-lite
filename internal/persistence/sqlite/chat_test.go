package sqlite

import (
	"context"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

func TestListChatEvents_ChannelCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	if _, err := s.InsertChatEvent(ctx, domain.ChatEvent{
		EventType:   domain.ChatEventMessage,
		ChannelName: "longfast",
		NodeID:      "!abcdef01",
		MessageText: "hello",
		MessageTime: now,
		ObservedAt:  now,
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("insert chat event: %v", err)
	}

	items, err := s.ListChatEvents(ctx, repo.ChatEventQuery{Channel: "LongFast", Limit: 50})
	if err != nil {
		t.Fatalf("list chat events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 chat event, got %d", len(items))
	}
	if items[0].ChannelName != "longfast" {
		t.Fatalf("expected stored channel preserved, got %q", items[0].ChannelName)
	}
	if items[0].NodeDisplay != "!abcdef01" {
		t.Fatalf("expected node display fallback to id, got %q", items[0].NodeDisplay)
	}
}

func TestListChatEvents_ResolvesNodeDisplayName(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	created, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!a55e5e56",
		LongName:           "skobkin-cap",
		ShortName:          "scap",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if !created {
		t.Fatalf("expected inserted node")
	}
	if _, err := s.UpsertNode(ctx, domain.Node{
		NodeID:             "!11223344",
		LongName:           "gateway",
		FirstSeenAt:        now,
		LastSeenAnyEventAt: now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("upsert gateway node: %v", err)
	}
	if _, err := s.InsertChatEvent(ctx, domain.ChatEvent{
		EventType:          domain.ChatEventMessage,
		ChannelName:        "LongFast",
		NodeID:             "!a55e5e56",
		MQTTUploaderNodeID: "!11223344",
		MessageText:        "hello",
		MessageTime:        now,
		ObservedAt:         now,
		CreatedAt:          now,
	}); err != nil {
		t.Fatalf("insert chat event: %v", err)
	}

	items, err := s.ListChatEvents(ctx, repo.ChatEventQuery{Channel: "longfast", Limit: 50})
	if err != nil {
		t.Fatalf("list chat events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 chat event, got %d", len(items))
	}
	if items[0].NodeDisplay != "skobkin-cap" {
		t.Fatalf("expected long-name display, got %q", items[0].NodeDisplay)
	}
	if items[0].MQTTUploaderNodeID != "!11223344" || items[0].MQTTUploaderDisplayName != "gateway" {
		t.Fatalf("expected uploader display, got %#v", items[0])
	}
}

func TestListChatEvents_FiltersByObservedSinceAt(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file:chat-history-window?mode=memory&cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cutoff := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	events := []domain.ChatEvent{
		{
			EventType:   domain.ChatEventMessage,
			ChannelName: "LongFast",
			NodeID:      "!old",
			MessageText: "old",
			ObservedAt:  cutoff.Add(-time.Second),
			CreatedAt:   cutoff.Add(-time.Second),
		},
		{
			EventType:   domain.ChatEventMessage,
			ChannelName: "LongFast",
			NodeID:      "!cutoff",
			MessageText: "cutoff",
			ObservedAt:  cutoff,
			CreatedAt:   cutoff,
		},
		{
			EventType:   domain.ChatEventMessage,
			ChannelName: "LongFast",
			NodeID:      "!new",
			MessageText: "new",
			ObservedAt:  cutoff.Add(time.Second),
			CreatedAt:   cutoff.Add(time.Second),
		},
	}
	for _, event := range events {
		if _, err := s.InsertChatEvent(ctx, event); err != nil {
			t.Fatalf("insert chat event: %v", err)
		}
	}

	items, err := s.ListChatEvents(ctx, repo.ChatEventQuery{
		Channel:         "LongFast",
		Limit:           50,
		ObservedSinceAt: cutoff,
	})
	if err != nil {
		t.Fatalf("list chat events: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 chat events, got %d", len(items))
	}
	if items[0].MessageText != "new" || items[1].MessageText != "cutoff" {
		t.Fatalf("unexpected messages: %#v", items)
	}
}

func TestListChatEvents_PaginatesWithinObservedSinceAt(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file:chat-history-window-before?mode=memory&cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cutoff := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	insert := func(text string, observedAt time.Time) int64 {
		t.Helper()
		id, err := s.InsertChatEvent(ctx, domain.ChatEvent{
			EventType:   domain.ChatEventMessage,
			ChannelName: "LongFast",
			NodeID:      "!" + text,
			MessageText: text,
			ObservedAt:  observedAt,
			CreatedAt:   observedAt,
		})
		if err != nil {
			t.Fatalf("insert chat event: %v", err)
		}

		return id
	}

	insert("old", cutoff.Add(-time.Second))
	insert("older-visible", cutoff)
	beforeID := insert("cursor", cutoff.Add(time.Second))
	insert("newer", cutoff.Add(2*time.Second))

	items, err := s.ListChatEvents(ctx, repo.ChatEventQuery{
		Channel:         "LongFast",
		Limit:           50,
		BeforeID:        beforeID,
		ObservedSinceAt: cutoff,
	})
	if err != nil {
		t.Fatalf("list chat events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 chat event, got %d", len(items))
	}
	if items[0].MessageText != "older-visible" {
		t.Fatalf("unexpected paginated message: %#v", items[0])
	}
}

func TestChatEventHopMetadataRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, config.SQLConfig{URL: "file::memory:?cache=shared", AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	hopStart := uint32(7)
	hopLimit := uint32(4)
	id, err := s.InsertChatEvent(ctx, domain.ChatEvent{
		EventType:   domain.ChatEventMessage,
		ChannelName: "LongFast",
		NodeID:      "!abcdef01",
		MessageText: "relayed",
		MessageTime: now,
		ObservedAt:  now,
		CreatedAt:   now,
		HopStart:    &hopStart,
		HopLimit:    &hopLimit,
	})
	if err != nil {
		t.Fatalf("insert chat event: %v", err)
	}

	items, err := s.ListChatEvents(ctx, repo.ChatEventQuery{Channel: "LongFast", Limit: 50})
	if err != nil {
		t.Fatalf("list chat events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 chat event, got %d", len(items))
	}
	if items[0].ID != id {
		t.Fatalf("expected id=%d, got %d", id, items[0].ID)
	}
	if items[0].HopStart == nil || *items[0].HopStart != 7 {
		t.Fatalf("expected HopStart=7, got %#v", items[0].HopStart)
	}
	if items[0].HopLimit == nil || *items[0].HopLimit != 4 {
		t.Fatalf("expected HopLimit=4, got %#v", items[0].HopLimit)
	}

	hopZero := uint32(0)
	if _, err := s.InsertChatEvent(ctx, domain.ChatEvent{
		EventType:   domain.ChatEventMessage,
		ChannelName: "LongFast",
		NodeID:      "!abcdef02",
		MessageText: "self upload",
		MessageTime: now,
		ObservedAt:  now,
		CreatedAt:   now,
		HopStart:    &hopZero,
		HopLimit:    &hopZero,
	}); err != nil {
		t.Fatalf("insert zero-hop chat event: %v", err)
	}

	items, err = s.ListChatEvents(ctx, repo.ChatEventQuery{Channel: "LongFast", Limit: 50})
	if err != nil {
		t.Fatalf("list chat events: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 chat events, got %d", len(items))
	}
	for _, item := range items {
		if item.NodeID == "!abcdef02" {
			if item.HopStart == nil || *item.HopStart != 0 {
				t.Fatalf("expected HopStart=0 for self-upload, got %#v", item.HopStart)
			}
			if item.HopLimit == nil || *item.HopLimit != 0 {
				t.Fatalf("expected HopLimit=0 for self-upload, got %#v", item.HopLimit)
			}
		}
	}
}
