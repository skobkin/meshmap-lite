package ingest

import (
	"log/slog"
	"strconv"
	"sync"
	"time"

	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/meshtastic"
)

type tracerouteTrackerOptions struct {
	timeout        time.Duration
	maxEntries     int
	finalRetention time.Duration
}

type tracerouteObservation struct {
	packetID   uint32
	channel    string
	now        time.Time
	reportedAt *time.Time
	payload    *meshtastic.TraceroutePayload
}

type tracerouteRoutingObservation struct {
	packetID   uint32
	channel    string
	now        time.Time
	reportedAt *time.Time
	payload    *meshtastic.RoutingPayload
}

type tracerouteStep struct {
	Type       string     `json:"type"`
	ObservedAt time.Time  `json:"observed_at"`
	ReportedAt *time.Time `json:"reported_at,omitempty"`
	PacketID   uint32     `json:"packet_id,omitempty"`
}

type tracerouteLifecycleRecord struct {
	requestID           uint32
	nodeID              string
	channel             string
	status              string
	fromNodeID          string
	toNodeID            string
	forwardPath         []string
	returnPath          []string
	forwardSNR          []int32
	returnSNR           []int32
	inferredForwardPath bool
	inferredReturnPath  bool
	inferredDirect      bool
	errorReason         string
	startedAt           time.Time
	updatedAt           time.Time
	completedAt         *time.Time
	sourcePackets       map[string]uint32
	steps               []tracerouteStep
}

type tracerouteTrackerResult struct {
	suppressPacketLog bool
	lifecycle         *tracerouteLifecycleRecord
}

type tracerouteTracker struct {
	mu      sync.Mutex
	log     *slog.Logger
	timeout time.Duration
	max     int
	retain  time.Duration
	entries map[string]*tracerouteTrackerEntry
}

type tracerouteTrackerEntry struct {
	key                 string
	requestID           uint32
	requestPacketID     uint32
	replyPacketID       uint32
	routingPacketID     uint32
	fromNodeID          string
	toNodeID            string
	channel             string
	startedAt           time.Time
	updatedAt           time.Time
	status              string
	forwardPath         []string
	returnPath          []string
	forwardSNR          []int32
	returnSNR           []int32
	inferredForwardPath bool
	inferredReturnPath  bool
	inferredDirect      bool
	errorReason         string
	finalEmitted        bool
	completedAt         *time.Time
	steps               []tracerouteStep
}

func newTracerouteTracker(log *slog.Logger, opts tracerouteTrackerOptions) *tracerouteTracker {
	timeout := opts.timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxEntries := opts.maxEntries
	if maxEntries < 1 {
		maxEntries = 1000
	}
	finalRetention := opts.finalRetention
	if finalRetention <= 0 {
		finalRetention = timeout
	}

	return &tracerouteTracker{
		log:     log,
		timeout: timeout,
		max:     maxEntries,
		retain:  finalRetention,
		entries: make(map[string]*tracerouteTrackerEntry, maxEntries),
	}
}

func (t *tracerouteTracker) OnRequest(obs tracerouteObservation) tracerouteTrackerResult {
	if obs.payload == nil {
		return tracerouteTrackerResult{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.evictForInsert(obs.now)

	entry := &tracerouteTrackerEntry{
		key:             tracerouteTrackerKey(obs.payload.RequestID, obs.packetID),
		requestID:       effectiveTracerouteRequestID(obs.payload.RequestID, obs.packetID),
		requestPacketID: obs.packetID,
		fromNodeID:      obs.payload.FromNodeID,
		toNodeID:        obs.payload.ToNodeID,
		channel:         obs.channel,
		startedAt:       obs.now,
		updatedAt:       obs.now,
		status:          "requested",
	}
	entry.steps = append(entry.steps, tracerouteStep{
		Type:       "request",
		ObservedAt: obs.now,
		ReportedAt: cloneTimePtr(obs.reportedAt),
		PacketID:   obs.packetID,
	})
	t.entries[entry.key] = entry

	return tracerouteTrackerResult{suppressPacketLog: true}
}

func (t *tracerouteTracker) OnReply(obs tracerouteObservation) tracerouteTrackerResult {
	if obs.payload == nil {
		return tracerouteTrackerResult{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entry := t.entries[tracerouteTrackerKey(obs.payload.RequestID, 0)]
	if entry == nil && obs.payload.ReplyID > 0 {
		entry = t.entries[tracerouteTrackerKey(0, obs.payload.ReplyID)]
	}
	if entry == nil {
		return tracerouteTrackerResult{}
	}
	if entry.finalEmitted {
		return tracerouteTrackerResult{suppressPacketLog: true}
	}

	entry.replyPacketID = obs.packetID
	entry.updatedAt = obs.now
	if entry.channel == "" {
		entry.channel = obs.channel
	}
	if entry.fromNodeID == "" {
		entry.fromNodeID = obs.payload.FromNodeID
	}
	if entry.toNodeID == "" {
		entry.toNodeID = obs.payload.ToNodeID
	}
	entry.forwardPath, entry.inferredForwardPath = preferTraceroutePath(
		entry.forwardPath,
		entry.inferredForwardPath,
		obs.payload.ForwardPath,
		obs.payload.InferredForwardPath,
	)
	entry.returnPath, entry.inferredReturnPath = preferTraceroutePath(
		entry.returnPath,
		entry.inferredReturnPath,
		obs.payload.ReturnPath,
		obs.payload.InferredReturnPath,
	)
	entry.forwardSNR = preferTracerouteSNR(entry.forwardSNR, obs.payload.SnrTowards)
	entry.returnSNR = preferTracerouteSNR(entry.returnSNR, obs.payload.SnrBack)
	if obs.payload.InferredDirect {
		entry.inferredDirect = true
	}
	entry.status = obs.payload.Status
	if entry.status == "" || entry.status == "requested" {
		entry.status = "partial"
	}
	entry.steps = append(entry.steps, tracerouteStep{
		Type:       "reply",
		ObservedAt: obs.now,
		ReportedAt: cloneTimePtr(obs.reportedAt),
		PacketID:   obs.packetID,
	})
	entry.finalEmitted = true
	completedAt := obs.now
	entry.completedAt = &completedAt

	lifecycle := entry.snapshot()

	return tracerouteTrackerResult{
		suppressPacketLog: true,
		lifecycle:         &lifecycle,
	}
}

func (t *tracerouteTracker) OnRouting(obs tracerouteRoutingObservation) tracerouteTrackerResult {
	if obs.payload == nil || obs.payload.RequestID == 0 {
		return tracerouteTrackerResult{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entry := t.entries[tracerouteTrackerKey(obs.payload.RequestID, 0)]
	if entry == nil {
		return tracerouteTrackerResult{}
	}
	if entry.finalEmitted {
		return tracerouteTrackerResult{suppressPacketLog: true}
	}

	entry.routingPacketID = obs.packetID
	entry.updatedAt = obs.now
	if entry.channel == "" {
		entry.channel = obs.channel
	}
	if entry.fromNodeID == "" {
		entry.fromNodeID = obs.payload.FromNodeID
	}
	if entry.toNodeID == "" {
		entry.toNodeID = obs.payload.ToNodeID
	}

	stepType := "routing"
	if obs.payload.ErrorReason != "" && obs.payload.ErrorReason != "NONE" {
		stepType = "routing_error"
	}
	entry.steps = append(entry.steps, tracerouteStep{
		Type:       stepType,
		ObservedAt: obs.now,
		ReportedAt: cloneTimePtr(obs.reportedAt),
		PacketID:   obs.packetID,
	})

	if obs.payload.ErrorReason == "" || obs.payload.ErrorReason == "NONE" {
		return tracerouteTrackerResult{suppressPacketLog: true}
	}

	entry.errorReason = obs.payload.ErrorReason
	entry.status = "failed"
	entry.finalEmitted = true
	completedAt := obs.now
	entry.completedAt = &completedAt

	lifecycle := entry.snapshot()

	return tracerouteTrackerResult{
		suppressPacketLog: true,
		lifecycle:         &lifecycle,
	}
}

func (t *tracerouteTracker) Sweep(now time.Time) []tracerouteLifecycleRecord {
	t.mu.Lock()
	defer t.mu.Unlock()

	var out []tracerouteLifecycleRecord
	for key, entry := range t.entries {
		if !entry.finalEmitted && now.Sub(entry.startedAt) >= t.timeout {
			entry.status = "timed_out"
			entry.updatedAt = now
			entry.finalEmitted = true
			entry.steps = append(entry.steps, tracerouteStep{
				Type:       "timeout",
				ObservedAt: now,
			})
			completedAt := now
			entry.completedAt = &completedAt
			out = append(out, entry.snapshot())

			continue
		}
		if entry.finalEmitted && now.Sub(entry.updatedAt) >= t.retain {
			delete(t.entries, key)
		}
	}

	return out
}

func (t *tracerouteTracker) ActiveLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.entries)
}

func (t *tracerouteTracker) evictForInsert(now time.Time) {
	for key, entry := range t.entries {
		if entry.finalEmitted && now.Sub(entry.updatedAt) >= t.retain {
			delete(t.entries, key)
		}
	}
	if len(t.entries) < t.max {
		return
	}

	for key, entry := range t.entries {
		if !entry.finalEmitted && now.Sub(entry.startedAt) >= t.timeout {
			delete(t.entries, key)
		}
	}
	if len(t.entries) < t.max {
		return
	}

	oldestKey := ""
	var oldestAt time.Time
	found := false
	for key, entry := range t.entries {
		if entry.finalEmitted {
			continue
		}
		if !found || entry.startedAt.Before(oldestAt) {
			oldestKey = key
			oldestAt = entry.startedAt
			found = true
		}
	}
	if !found {
		for key, entry := range t.entries {
			if !found || entry.updatedAt.Before(oldestAt) {
				oldestKey = key
				oldestAt = entry.updatedAt
				found = true
			}
		}
	}
	if found {
		delete(t.entries, oldestKey)
		if t.log != nil {
			t.log.Warn("evicted traceroute tracker entry",
				"active_entries", len(t.entries),
				"timeout_window", t.timeout,
				"evicted_key", oldestKey,
			)
		}
	}
}

func (e *tracerouteTrackerEntry) snapshot() tracerouteLifecycleRecord {
	var completedAt *time.Time
	if e.completedAt != nil {
		v := *e.completedAt
		completedAt = &v
	}

	sourcePackets := make(map[string]uint32, 3)
	if e.requestPacketID > 0 {
		sourcePackets["request"] = e.requestPacketID
	}
	if e.replyPacketID > 0 {
		sourcePackets["reply"] = e.replyPacketID
	}
	if e.routingPacketID > 0 {
		sourcePackets["routing"] = e.routingPacketID
	}

	return tracerouteLifecycleRecord{
		requestID:           e.requestID,
		nodeID:              firstNonEmpty(e.fromNodeID, e.toNodeID),
		channel:             e.channel,
		status:              e.status,
		fromNodeID:          e.fromNodeID,
		toNodeID:            e.toNodeID,
		forwardPath:         append([]string(nil), e.forwardPath...),
		returnPath:          append([]string(nil), e.returnPath...),
		forwardSNR:          append([]int32(nil), e.forwardSNR...),
		returnSNR:           append([]int32(nil), e.returnSNR...),
		inferredForwardPath: e.inferredForwardPath,
		inferredReturnPath:  e.inferredReturnPath,
		inferredDirect:      e.inferredDirect,
		errorReason:         e.errorReason,
		startedAt:           e.startedAt,
		updatedAt:           e.updatedAt,
		completedAt:         completedAt,
		sourcePackets:       sourcePackets,
		steps:               append([]tracerouteStep(nil), e.steps...),
	}
}

func tracerouteLifecycleLogEvent(in tracerouteLifecycleRecord) domain.LogEvent {
	details := map[string]any{
		"scope":      "lifecycle",
		"status":     in.status,
		"started_at": in.startedAt,
		"updated_at": in.updatedAt,
	}
	if in.requestID > 0 {
		details["request_id"] = in.requestID
	}
	if in.fromNodeID != "" {
		details["from"] = in.fromNodeID
	}
	if in.toNodeID != "" {
		details["to"] = in.toNodeID
	}
	if len(in.forwardPath) > 0 {
		details["forward_path"] = in.forwardPath
	}
	if len(in.returnPath) > 0 {
		details["return_path"] = in.returnPath
	}
	if len(in.forwardSNR) > 0 {
		details["forward_snr"] = in.forwardSNR
	}
	if len(in.returnSNR) > 0 {
		details["return_snr"] = in.returnSNR
	}
	if in.inferredForwardPath {
		details["inferred_forward_path"] = true
	}
	if in.inferredReturnPath {
		details["inferred_return_path"] = true
	}
	if in.inferredDirect {
		details["inferred_direct"] = true
	}
	if in.errorReason != "" {
		details["error_reason"] = in.errorReason
	}
	if in.completedAt != nil {
		details["completed_at"] = *in.completedAt
	}
	if len(in.sourcePackets) > 0 {
		details["source_packets"] = in.sourcePackets
	}
	if len(in.steps) > 0 {
		details["steps"] = in.steps
	}

	return domain.LogEvent{
		ObservedAt: in.updatedAt,
		NodeID:     in.nodeID,
		EventKind:  domain.LogEventKindTracerouteValue,
		Channel:    in.channel,
		Details:    details,
	}
}

func effectiveTracerouteRequestID(requestID, fallbackPacketID uint32) uint32 {
	if requestID > 0 {
		return requestID
	}

	return fallbackPacketID
}

func tracerouteTrackerKey(requestID, fallbackPacketID uint32) string {
	return "req:" + strconv.FormatUint(uint64(effectiveTracerouteRequestID(requestID, fallbackPacketID)), 10)
}

func preferTraceroutePath(existing []string, existingInferred bool, next []string, nextInferred bool) ([]string, bool) {
	if len(next) == 0 {
		return append([]string(nil), existing...), existingInferred
	}
	if len(existing) == 0 {
		return append([]string(nil), next...), nextInferred
	}
	if existingInferred && !nextInferred {
		return append([]string(nil), next...), nextInferred
	}
	if !existingInferred && nextInferred {
		return append([]string(nil), existing...), existingInferred
	}
	if len(next) > len(existing) {
		return append([]string(nil), next...), nextInferred
	}

	return append([]string(nil), existing...), existingInferred
}

func preferTracerouteSNR(existing []int32, next []int32) []int32 {
	if len(next) == 0 {
		return append([]int32(nil), existing...)
	}
	if len(existing) == 0 || len(next) > len(existing) {
		return append([]int32(nil), next...)
	}

	return append([]int32(nil), existing...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	v := *in

	return &v
}
