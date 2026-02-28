package ingest

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/meshtastic"
)

func TestTracerouteTrackerRequestReplyCompletesOnce(t *testing.T) {
	tracker := newTracerouteTracker(nil, tracerouteTrackerOptions{
		timeout:        30 * time.Second,
		maxEntries:     16,
		finalRetention: 30 * time.Second,
	})
	start := time.Unix(1772296589, 0).UTC()

	request := tracker.OnRequest(tracerouteObservation{
		packetID: 101,
		channel:  "LongFast",
		now:      start,
		payload: &meshtastic.TraceroutePayload{
			Role:       "request",
			RequestID:  321,
			FromNodeID: "!a55e5e56",
			ToNodeID:   "!9028d008",
		},
	})
	if !request.suppressPacketLog || request.lifecycle != nil {
		t.Fatalf("request should suppress raw row and wait for terminal lifecycle: %#v", request)
	}

	reply := tracker.OnReply(tracerouteObservation{
		packetID: 202,
		channel:  "LongFast",
		now:      start.Add(2 * time.Second),
		payload: &meshtastic.TraceroutePayload{
			Role:                "reply",
			Status:              "completed",
			RequestID:           321,
			ReplyID:             101,
			FromNodeID:          "!9028d008",
			ToNodeID:            "!a55e5e56",
			ForwardPath:         []string{"!a55e5e56", "!01020304", "!9028d008"},
			ReturnPath:          []string{"!9028d008", "!0a0b0c0d", "!a55e5e56"},
			SnrTowards:          []int32{22},
			SnrBack:             []int32{12},
			InferredForwardPath: true,
		},
	})
	if !reply.suppressPacketLog || reply.lifecycle == nil {
		t.Fatalf("expected one terminal lifecycle emission, got %#v", reply)
	}
	if reply.lifecycle.status != "completed" {
		t.Fatalf("unexpected reply status: %q", reply.lifecycle.status)
	}
	if len(reply.lifecycle.forwardPath) != 3 || len(reply.lifecycle.returnPath) != 3 {
		t.Fatalf("expected merged paths, got %#v", reply.lifecycle)
	}
	if !reply.lifecycle.inferredForwardPath {
		t.Fatalf("expected inferred forward marker to survive")
	}
	if len(reply.lifecycle.steps) != 2 {
		t.Fatalf("expected request+reply steps, got %#v", reply.lifecycle.steps)
	}

	dup := tracker.OnReply(tracerouteObservation{
		packetID: 303,
		channel:  "LongFast",
		now:      start.Add(3 * time.Second),
		payload: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "completed",
			RequestID:   321,
			ReplyID:     101,
			ForwardPath: []string{"!a55e5e56", "!01020304", "!9028d008"},
		},
	})
	if !dup.suppressPacketLog || dup.lifecycle != nil {
		t.Fatalf("duplicate reply must suppress raw row and not emit again: %#v", dup)
	}
}

func TestTracerouteTrackerRoutingFailureAndNoneHandling(t *testing.T) {
	tracker := newTracerouteTracker(nil, tracerouteTrackerOptions{
		timeout:        30 * time.Second,
		maxEntries:     16,
		finalRetention: 30 * time.Second,
	})
	start := time.Unix(1772296589, 0).UTC()

	_ = tracker.OnRequest(tracerouteObservation{
		packetID: 101,
		channel:  "LongFast",
		now:      start,
		payload: &meshtastic.TraceroutePayload{
			Role:       "request",
			RequestID:  321,
			FromNodeID: "!a55e5e56",
			ToNodeID:   "!9028d008",
		},
	})

	none := tracker.OnRouting(tracerouteRoutingObservation{
		packetID: 201,
		channel:  "LongFast",
		now:      start.Add(time.Second),
		payload: &meshtastic.RoutingPayload{
			RequestID:   321,
			ErrorReason: "NONE",
		},
	})
	if !none.suppressPacketLog || none.lifecycle != nil {
		t.Fatalf("routing NONE should be absorbed into tracker state only: %#v", none)
	}

	failure := tracker.OnRouting(tracerouteRoutingObservation{
		packetID: 202,
		channel:  "LongFast",
		now:      start.Add(2 * time.Second),
		payload: &meshtastic.RoutingPayload{
			RequestID:   321,
			FromNodeID:  "!a55e5e56",
			ToNodeID:    "!9028d008",
			ErrorReason: "NO_ROUTE",
		},
	})
	if !failure.suppressPacketLog || failure.lifecycle == nil {
		t.Fatalf("expected routing failure lifecycle emission, got %#v", failure)
	}
	if failure.lifecycle.status != "failed" || failure.lifecycle.errorReason != "NO_ROUTE" {
		t.Fatalf("unexpected failure lifecycle: %#v", failure.lifecycle)
	}
	if len(failure.lifecycle.steps) != 3 {
		t.Fatalf("expected request+routing+routing_error steps, got %#v", failure.lifecycle.steps)
	}

	dup := tracker.OnRouting(tracerouteRoutingObservation{
		packetID: 203,
		channel:  "LongFast",
		now:      start.Add(3 * time.Second),
		payload: &meshtastic.RoutingPayload{
			RequestID:   321,
			ErrorReason: "NO_ROUTE",
		},
	})
	if !dup.suppressPacketLog || dup.lifecycle != nil {
		t.Fatalf("duplicate routing failure must not emit again: %#v", dup)
	}
}

func TestTracerouteTrackerTimeoutAndRetention(t *testing.T) {
	tracker := newTracerouteTracker(nil, tracerouteTrackerOptions{
		timeout:        10 * time.Second,
		maxEntries:     16,
		finalRetention: 5 * time.Second,
	})
	start := time.Unix(1772296589, 0).UTC()

	_ = tracker.OnRequest(tracerouteObservation{
		packetID: 101,
		channel:  "LongFast",
		now:      start,
		payload: &meshtastic.TraceroutePayload{
			Role:       "request",
			RequestID:  321,
			FromNodeID: "!a55e5e56",
			ToNodeID:   "!9028d008",
		},
	})

	timeoutRows := tracker.Sweep(start.Add(11 * time.Second))
	if len(timeoutRows) != 1 {
		t.Fatalf("expected one timeout lifecycle, got %d", len(timeoutRows))
	}
	if timeoutRows[0].status != "timed_out" {
		t.Fatalf("unexpected timeout status: %#v", timeoutRows[0])
	}
	if len(timeoutRows[0].steps) != 2 || timeoutRows[0].steps[1].Type != "timeout" {
		t.Fatalf("expected request+timeout steps, got %#v", timeoutRows[0].steps)
	}
	if len(tracker.Sweep(start.Add(12*time.Second))) != 0 {
		t.Fatalf("timeout sweep must not emit duplicate rows")
	}
	if tracker.ActiveLen() != 1 {
		t.Fatalf("timed out entry should remain for dedup retention")
	}

	tracker.Sweep(start.Add(17 * time.Second))
	if tracker.ActiveLen() != 0 {
		t.Fatalf("expected timed out entry to be evicted after retention")
	}
}

func TestTracerouteTrackerUnmatchedReplyRemainsRawEvidence(t *testing.T) {
	tracker := newTracerouteTracker(nil, tracerouteTrackerOptions{
		timeout:        30 * time.Second,
		maxEntries:     16,
		finalRetention: 30 * time.Second,
	})
	start := time.Unix(1772296589, 0).UTC()

	reply := tracker.OnReply(tracerouteObservation{
		packetID: 202,
		channel:  "LongFast",
		now:      start,
		payload: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "partial",
			RequestID:   321,
			FromNodeID:  "!9028d008",
			ToNodeID:    "!a55e5e56",
			ForwardPath: []string{"!a55e5e56", "!9028d008"},
		},
	})
	if reply.suppressPacketLog || reply.lifecycle != nil {
		t.Fatalf("unmatched reply should remain raw evidence only: %#v", reply)
	}
}

func TestTracerouteTrackerEvictsOldestNonFinalWhenFull(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := newTracerouteTracker(logger, tracerouteTrackerOptions{
		timeout:        time.Hour,
		maxEntries:     2,
		finalRetention: time.Minute,
	})
	start := time.Unix(1772296589, 0).UTC()

	_ = tracker.OnRequest(tracerouteObservation{
		packetID: 100,
		channel:  "LongFast",
		now:      start,
		payload:  &meshtastic.TraceroutePayload{Role: "request", RequestID: 100},
	})
	_ = tracker.OnRequest(tracerouteObservation{
		packetID: 200,
		channel:  "LongFast",
		now:      start.Add(time.Second),
		payload:  &meshtastic.TraceroutePayload{Role: "request", RequestID: 200},
	})
	_ = tracker.OnRequest(tracerouteObservation{
		packetID: 300,
		channel:  "LongFast",
		now:      start.Add(2 * time.Second),
		payload:  &meshtastic.TraceroutePayload{Role: "request", RequestID: 300},
	})

	if tracker.ActiveLen() != 2 {
		t.Fatalf("expected bounded tracker size, got %d", tracker.ActiveLen())
	}
	reply := tracker.OnReply(tracerouteObservation{
		packetID: 301,
		channel:  "LongFast",
		now:      start.Add(3 * time.Second),
		payload: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "completed",
			RequestID:   300,
			ForwardPath: []string{"!a", "!b"},
		},
	})
	if !reply.suppressPacketLog || reply.lifecycle == nil || reply.lifecycle.status != "completed" {
		t.Fatalf("expected newest request to remain after eviction, got %#v", reply)
	}
}

func TestServiceTracerouteLogDecisionSuppressesMatchedPacketRows(t *testing.T) {
	svc := &Service{
		cfg: Config{
			Ingest: config.IngestConfig{
				Traceroute: config.TracerouteIngestConfig{
					Timeout:        30 * time.Second,
					MaxEntries:     16,
					FinalRetention: 30 * time.Second,
				},
			},
		},
		tracker: newTracerouteTracker(nil, tracerouteTrackerOptions{
			timeout:        30 * time.Second,
			maxEntries:     16,
			finalRetention: 30 * time.Second,
		}),
	}
	start := time.Unix(1772296589, 0).UTC()

	request := svc.tracerouteLogDecision(meshtastic.ParsedEvent{
		Kind: meshtastic.ParsedTraceroute,
		Traceroute: &meshtastic.TraceroutePayload{
			Role:       "request",
			RequestID:  321,
			FromNodeID: "!a55e5e56",
			ToNodeID:   "!9028d008",
		},
		PacketID: 101,
	}, "LongFast", start)
	if !request.suppressPacketLog || len(request.lifecycleEvents) != 0 {
		t.Fatalf("request should not persist raw or intermediate lifecycle rows: %#v", request)
	}

	reply := svc.tracerouteLogDecision(meshtastic.ParsedEvent{
		Kind: meshtastic.ParsedTraceroute,
		Traceroute: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "completed",
			RequestID:   321,
			ReplyID:     101,
			ForwardPath: []string{"!a55e5e56", "!01020304", "!9028d008"},
		},
		PacketID: 202,
	}, "LongFast", start.Add(time.Second))
	if !reply.suppressPacketLog || len(reply.lifecycleEvents) != 1 {
		t.Fatalf("matched reply should emit one lifecycle row and suppress raw row: %#v", reply)
	}
	if reply.lifecycleEvents[0].Details["status"] != "completed" {
		t.Fatalf("unexpected terminal lifecycle row: %#v", reply.lifecycleEvents[0].Details)
	}
	if _, ok := reply.lifecycleEvents[0].Details["steps"]; !ok {
		t.Fatalf("expected step history in final lifecycle row: %#v", reply.lifecycleEvents[0].Details)
	}

	orphan := svc.tracerouteLogDecision(meshtastic.ParsedEvent{
		Kind: meshtastic.ParsedTraceroute,
		Traceroute: &meshtastic.TraceroutePayload{
			Role:        "reply",
			Status:      "partial",
			RequestID:   999,
			ForwardPath: []string{"!x", "!y"},
		},
		PacketID: 303,
	}, "LongFast", start.Add(2*time.Second))
	if orphan.suppressPacketLog || len(orphan.lifecycleEvents) != 0 {
		t.Fatalf("orphan reply should remain raw packet evidence: %#v", orphan)
	}
}
