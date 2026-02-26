package ingest

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
	"meshmap-lite/internal/repo"
)

type testStore struct {
	lastNode     *domain.Node
	lastPosition *domain.NodePosition
}

func (s *testStore) UpsertNode(_ context.Context, node domain.Node) (bool, error) {
	n := node
	s.lastNode = &n

	return false, nil
}

func (s *testStore) UpsertPosition(_ context.Context, pos domain.NodePosition) error {
	p := pos
	s.lastPosition = &p

	return nil
}

func (*testStore) MergeTelemetry(context.Context, domain.NodeTelemetrySnapshot) error {
	return nil
}

func (*testStore) InsertChatEvent(context.Context, domain.ChatEvent) (int64, error) {
	return 0, nil
}

func (*testStore) GetMapNodes(context.Context) ([]repo.MapNode, error) {
	return nil, nil
}

func (*testStore) ListNodes(context.Context) ([]repo.NodeSummary, error) {
	return nil, nil
}

func (*testStore) GetNodeDetails(context.Context, string) (repo.NodeDetails, error) {
	return repo.NodeDetails{}, nil
}

func (*testStore) ListChatEvents(context.Context, string, int, int64) ([]domain.ChatEvent, error) {
	return nil, nil
}

func (*testStore) Stats(context.Context, time.Duration) (domain.Stats, error) {
	return domain.Stats{}, nil
}

type testEmitter struct{}

func (testEmitter) Emit(domain.RealtimeEvent) {}

func TestHandleMapReportMergesNodeAndPositionFields(t *testing.T) {
	store := &testStore{}
	svc := &Service{
		store:   store,
		emitter: testEmitter{},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	neighbors := 9
	precision := uint32(12)
	alt := 131.0
	now := time.Now().UTC()
	evt := meshtastic.ParsedEvent{
		NodeID: "!9028d008",
		MapReport: &meshtastic.MapReportPayload{
			LongName:               "arkh-07",
			ShortName:              "am07",
			Role:                   "CLIENT",
			BoardModel:             "T_BEAM",
			FirmwareVersion:        "2.7.18.fb3bf780",
			LoRaRegion:             "RU",
			ModemPreset:            "LONG_FAST",
			HasDefaultChannel:      true,
			HasOptedReportLocation: true,
			NeighborNodesCount:     &neighbors,
			Latitude:               64.5,
			Longitude:              40.6,
			AltitudeM:              &alt,
			PositionPrecision:      &precision,
		},
	}

	ok := svc.handleMapReport(context.Background(), evt, now)
	if !ok {
		t.Fatalf("expected map report to be processed")
	}
	if store.lastNode == nil {
		t.Fatalf("expected node upsert")
	}
	if store.lastNode.FirmwareVersion != "2.7.18.fb3bf780" {
		t.Fatalf("unexpected firmware version: %q", store.lastNode.FirmwareVersion)
	}
	if store.lastNode.LoRaRegion != "RU" {
		t.Fatalf("unexpected region: %q", store.lastNode.LoRaRegion)
	}
	if store.lastNode.ModemPreset != "LONG_FAST" {
		t.Fatalf("unexpected modem preset: %q", store.lastNode.ModemPreset)
	}
	if store.lastNode.HasDefaultChannel == nil || !*store.lastNode.HasDefaultChannel {
		t.Fatalf("unexpected has_default_channel: %v", store.lastNode.HasDefaultChannel)
	}
	if store.lastNode.HasOptedReportLocation == nil || !*store.lastNode.HasOptedReportLocation {
		t.Fatalf("unexpected has_opted_report_location: %v", store.lastNode.HasOptedReportLocation)
	}
	if store.lastNode.NeighborNodesCount == nil || *store.lastNode.NeighborNodesCount != 9 {
		t.Fatalf("unexpected neighbor count: %v", store.lastNode.NeighborNodesCount)
	}
	if store.lastPosition == nil {
		t.Fatalf("expected position upsert")
	}
	if store.lastPosition.SourceKind != domain.PositionSourceMapReport {
		t.Fatalf("unexpected source kind: %q", store.lastPosition.SourceKind)
	}
	if store.lastPosition.PositionPrecision == nil || *store.lastPosition.PositionPrecision != 12 {
		t.Fatalf("unexpected position precision: %v", store.lastPosition.PositionPrecision)
	}
}
