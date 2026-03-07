package testkit

import (
	"context"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// FakeStore is a lightweight configurable fake for repository-facing tests.
type FakeStore struct {
	UpsertNodeFn      func(context.Context, domain.Node) (bool, error)
	UpsertPositionFn  func(context.Context, domain.NodePosition) error
	MergeTelemetryFn  func(context.Context, domain.NodeTelemetrySnapshot) error
	InsertChatEventFn func(context.Context, domain.ChatEvent) (int64, error)
	InsertLogEventFn  func(context.Context, domain.LogEvent) (int64, error)

	GetMapNodesFn    func(context.Context, time.Duration) ([]repo.MapNode, error)
	ListNodesFn      func(context.Context) ([]repo.NodeSummary, error)
	GetNodeDetailsFn func(context.Context, string) (repo.NodeDetails, error)
	ListChatEventsFn func(context.Context, repo.ChatEventQuery) ([]domain.ChatEvent, error)
	ListLogEventsFn  func(context.Context, domain.LogEventQuery) ([]domain.LogEventView, error)
	StatsFn          func(context.Context, time.Duration) (domain.Stats, error)
}

// UpsertNode implements repo.WriteStore.
func (f *FakeStore) UpsertNode(ctx context.Context, node domain.Node) (bool, error) {
	if f.UpsertNodeFn != nil {
		return f.UpsertNodeFn(ctx, node)
	}

	return false, nil
}

// UpsertPosition implements repo.WriteStore.
func (f *FakeStore) UpsertPosition(ctx context.Context, pos domain.NodePosition) error {
	if f.UpsertPositionFn != nil {
		return f.UpsertPositionFn(ctx, pos)
	}

	return nil
}

// MergeTelemetry implements repo.WriteStore.
func (f *FakeStore) MergeTelemetry(ctx context.Context, snapshot domain.NodeTelemetrySnapshot) error {
	if f.MergeTelemetryFn != nil {
		return f.MergeTelemetryFn(ctx, snapshot)
	}

	return nil
}

// InsertChatEvent implements repo.WriteStore.
func (f *FakeStore) InsertChatEvent(ctx context.Context, event domain.ChatEvent) (int64, error) {
	if f.InsertChatEventFn != nil {
		return f.InsertChatEventFn(ctx, event)
	}

	return 0, nil
}

// InsertLogEvent implements repo.WriteStore.
func (f *FakeStore) InsertLogEvent(ctx context.Context, event domain.LogEvent) (int64, error) {
	if f.InsertLogEventFn != nil {
		return f.InsertLogEventFn(ctx, event)
	}

	return 0, nil
}

// GetMapNodes implements repo.ReadStore.
func (f *FakeStore) GetMapNodes(ctx context.Context, hidePositionAfter time.Duration) ([]repo.MapNode, error) {
	if f.GetMapNodesFn != nil {
		return f.GetMapNodesFn(ctx, hidePositionAfter)
	}

	return nil, nil
}

// ListNodes implements repo.ReadStore.
func (f *FakeStore) ListNodes(ctx context.Context) ([]repo.NodeSummary, error) {
	if f.ListNodesFn != nil {
		return f.ListNodesFn(ctx)
	}

	return nil, nil
}

// GetNodeDetails implements repo.ReadStore.
func (f *FakeStore) GetNodeDetails(ctx context.Context, nodeID string) (repo.NodeDetails, error) {
	if f.GetNodeDetailsFn != nil {
		return f.GetNodeDetailsFn(ctx, nodeID)
	}

	return repo.NodeDetails{}, nil
}

// ListChatEvents implements repo.ReadStore.
func (f *FakeStore) ListChatEvents(ctx context.Context, q repo.ChatEventQuery) ([]domain.ChatEvent, error) {
	if f.ListChatEventsFn != nil {
		return f.ListChatEventsFn(ctx, q)
	}

	return nil, nil
}

// ListLogEvents implements repo.ReadStore.
func (f *FakeStore) ListLogEvents(ctx context.Context, q domain.LogEventQuery) ([]domain.LogEventView, error) {
	if f.ListLogEventsFn != nil {
		return f.ListLogEventsFn(ctx, q)
	}

	return nil, nil
}

// Stats implements repo.ReadStore.
func (f *FakeStore) Stats(ctx context.Context, threshold time.Duration) (domain.Stats, error) {
	if f.StatsFn != nil {
		return f.StatsFn(ctx, threshold)
	}

	return domain.Stats{}, nil
}
