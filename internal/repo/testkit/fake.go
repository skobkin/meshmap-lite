package testkit

import (
	"context"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// FakeStore is a lightweight configurable fake for repository-facing tests.
type FakeStore struct {
	UpsertNodeFn          func(context.Context, domain.Node) (bool, error)
	UpsertPositionFn      func(context.Context, domain.NodePosition) error
	MergeTelemetryFn      func(context.Context, domain.NodeTelemetrySnapshot) (domain.NodeTelemetrySnapshot, error)
	UpsertTopologyEdgesFn func(context.Context, []domain.TopologyEdge) error
	InsertChatEventFn     func(context.Context, domain.ChatEvent) (int64, error)
	InsertLogEventFn      func(context.Context, domain.LogEvent) (int64, error)
	ResolveNodeDisplayFn  func(context.Context, string) (string, error)

	GetMapNodesFn       func(context.Context, repo.MapNodeQuery) ([]repo.MapNode, error)
	ListNodesFn         func(context.Context, repo.NodeListQuery) ([]repo.NodeSummary, error)
	GetNodeDetailsFn    func(context.Context, repo.NodeDetailsQuery) (repo.NodeDetails, error)
	ListTopologyEdgesFn func(context.Context, repo.TopologyEdgeQuery) ([]domain.TopologyEdge, error)
	ListChatEventsFn    func(context.Context, repo.ChatEventQuery) ([]domain.ChatEvent, error)
	ListLogEventsFn     func(context.Context, domain.LogEventQuery) ([]domain.LogEventView, error)
	ActivityBucketsFn   func(context.Context, domain.ActivityQuery) ([]domain.ActivityBucket, error)
	StatsFn             func(context.Context, time.Duration) (domain.Stats, error)
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
func (f *FakeStore) MergeTelemetry(ctx context.Context, snapshot domain.NodeTelemetrySnapshot) (domain.NodeTelemetrySnapshot, error) {
	if f.MergeTelemetryFn != nil {
		return f.MergeTelemetryFn(ctx, snapshot)
	}

	return snapshot, nil
}

// UpsertTopologyEdges implements repo.WriteStore.
func (f *FakeStore) UpsertTopologyEdges(ctx context.Context, edges []domain.TopologyEdge) error {
	if f.UpsertTopologyEdgesFn != nil {
		return f.UpsertTopologyEdgesFn(ctx, edges)
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

// ResolveNodeDisplay implements repo.WriteStore.
func (f *FakeStore) ResolveNodeDisplay(ctx context.Context, nodeID string) (string, error) {
	if f.ResolveNodeDisplayFn != nil {
		return f.ResolveNodeDisplayFn(ctx, nodeID)
	}

	return nodeID, nil
}

// GetMapNodes implements repo.ReadStore.
func (f *FakeStore) GetMapNodes(ctx context.Context, q repo.MapNodeQuery) ([]repo.MapNode, error) {
	if f.GetMapNodesFn != nil {
		return f.GetMapNodesFn(ctx, q)
	}

	return nil, nil
}

// ListNodes implements repo.ReadStore.
func (f *FakeStore) ListNodes(ctx context.Context, q repo.NodeListQuery) ([]repo.NodeSummary, error) {
	if f.ListNodesFn != nil {
		return f.ListNodesFn(ctx, q)
	}

	return nil, nil
}

// GetNodeDetails implements repo.ReadStore.
func (f *FakeStore) GetNodeDetails(ctx context.Context, q repo.NodeDetailsQuery) (repo.NodeDetails, error) {
	if f.GetNodeDetailsFn != nil {
		return f.GetNodeDetailsFn(ctx, q)
	}

	return repo.NodeDetails{}, nil
}

// ListTopologyEdges implements repo.ReadStore.
func (f *FakeStore) ListTopologyEdges(ctx context.Context, q repo.TopologyEdgeQuery) ([]domain.TopologyEdge, error) {
	if f.ListTopologyEdgesFn != nil {
		return f.ListTopologyEdgesFn(ctx, q)
	}

	return nil, nil
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

// ActivityBuckets implements repo.ReadStore.
func (f *FakeStore) ActivityBuckets(ctx context.Context, q domain.ActivityQuery) ([]domain.ActivityBucket, error) {
	if f.ActivityBucketsFn != nil {
		return f.ActivityBucketsFn(ctx, q)
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
