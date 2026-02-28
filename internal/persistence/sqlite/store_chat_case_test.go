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
}
