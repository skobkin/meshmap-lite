package stats

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeHardwareStore records every RecordHardwareHistoryWeek /
// LastHardwareHistoryWeek call so the tests can assert on the schedule without
// coupling to the real SQLite store. Mirrors fakeStore from the firmware
// snapshot tests; kept separate so the two fakes don't entangle their call
// histories.
type fakeHardwareStore struct {
	mu sync.Mutex

	// lastWeek is the value returned by LastHardwareHistoryWeek. Zero
	// time means "no rows yet".
	lastWeek time.Time

	// recorded captures the (weekStart, observedAt, maxAge) arguments of each
	// RecordHardwareHistoryWeek call.
	recorded []hardwareRecordedCall

	// recordError and lastError are returned from the corresponding
	// methods when set; nil otherwise.
	recordError error
	lastError   error

	// insertZero forces RecordHardwareHistoryWeek to return 0 rows.
	// insertedRows, when set, returns a sequence of row counts. Both
	// simulate the empty-fleet / all-stale-fleet case where the SQL
	// store's INSERT OR IGNORE matches zero rows.
	insertZero   bool
	insertedRows []int64
}

type hardwareRecordedCall struct {
	WeekStart  time.Time
	ObservedAt time.Time
	MaxAge     time.Duration
}

func (f *fakeHardwareStore) RecordHardwareHistoryWeek(_ context.Context, weekStart, observedAt time.Time, maxAge time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordError != nil {
		return 0, f.recordError
	}
	f.recorded = append(f.recorded, hardwareRecordedCall{WeekStart: weekStart, ObservedAt: observedAt, MaxAge: maxAge})

	inserted := int64(1)
	if len(f.insertedRows) > 0 {
		inserted = f.insertedRows[0]
		f.insertedRows = f.insertedRows[1:]
	} else if f.insertZero {
		inserted = 0
	}
	if inserted > 0 {
		f.lastWeek = weekStart
	}

	return inserted, nil
}

func (f *fakeHardwareStore) LastHardwareHistoryWeek(_ context.Context) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.lastWeek, f.lastError
}

// newHardwareTestJob builds a HardwareSnapshotJob pinned to the given clock
// and a no-op callback.
func newHardwareTestJob(t *testing.T, store HardwareWriter, now func() time.Time) (*HardwareSnapshotJob, *fakeHardwareStore, *bool) {
	t.Helper()
	fs, ok := store.(*fakeHardwareStore)
	if !ok {
		t.Fatalf("expected *fakeHardwareStore, got %T", store)
	}
	called := false

	return NewHardwareSnapshotJob(HardwareSnapshotOptions{
		Store: store,
		Now:   now,
		OnSnapshot: func() {
			called = true
		},
	}), fs, &called
}

// TestHardwareRunOnce_CatchesUpOnFirstRun covers the cold-start scenario:
// empty history, current week not yet recorded. The job must write the current
// week's snapshot and fire the OnSnapshot callback.
func TestHardwareRunOnce_CatchesUpOnFirstRun(t *testing.T) {
	ctx := context.Background()
	store := &fakeHardwareStore{}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	job, fs, cb := newHardwareTestJob(t, store, func() time.Time { return now })

	if err := job.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}

	if len(fs.recorded) != 1 {
		t.Fatalf("expected one record call, got %d", len(fs.recorded))
	}
	expectedWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) // Monday
	if !fs.recorded[0].WeekStart.Equal(expectedWeek) {
		t.Errorf("expected week_start %s, got %s", expectedWeek, fs.recorded[0].WeekStart)
	}
	if !fs.recorded[0].ObservedAt.Equal(now) {
		t.Errorf("expected observed_at %s, got %s", now, fs.recorded[0].ObservedAt)
	}
	if !*cb {
		t.Errorf("expected OnSnapshot callback to fire")
	}
}

// TestHardwareRunOnce_NoopIfCurrentWeekAlreadyRecorded covers the warm-start
// scenario: history already has a row for the current week, runOnce must be a
// no-op (no record call, no callback).
func TestHardwareRunOnce_NoopIfCurrentWeekAlreadyRecorded(t *testing.T) {
	ctx := context.Background()
	store := &fakeHardwareStore{
		lastWeek: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), // Monday
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday of same week
	job, fs, cb := newHardwareTestJob(t, store, func() time.Time { return now })

	if err := job.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}

	if len(fs.recorded) != 0 {
		t.Errorf("expected no record call, got %d", len(fs.recorded))
	}
	if *cb {
		t.Errorf("expected OnSnapshot NOT to fire")
	}
}

// TestHardwareRunOnce_FirstWriterWinsOnMidWeekModelChange mirrors the firmware
// invariant: if a node changes its hardware_model_id mid-week, a second run for
// the same week must NOT overwrite the first writer's row. With the fake store
// this is modelled by leaving recorded untouched (the real SQL store enforces
// the same invariant via INSERT OR IGNORE).
func TestHardwareRunOnce_FirstWriterWinsOnMidWeekModelChange(t *testing.T) {
	ctx := context.Background()
	currentWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	store := &fakeHardwareStore{lastWeek: currentWeek}

	// Simulate "earlier this week, a row was written" by pre-populating
	// recorded with the Monday snapshot.
	earlierObserved := currentWeek.Add(2 * time.Hour)
	store.recorded = append(store.recorded, hardwareRecordedCall{
		WeekStart:  currentWeek,
		ObservedAt: earlierObserved,
	})

	// Now a new NodeInfo packet arrives mid-week — but the job must NOT
	// issue a new record call (the SQL store would also drop it via INSERT
	// OR IGNORE).
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job, fs, cb := newHardwareTestJob(t, store, func() time.Time { return now })

	if err := job.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}

	if len(fs.recorded) != 1 {
		t.Fatalf("expected the Monday record to remain (no overwrite), got %d records", len(fs.recorded))
	}
	if !fs.recorded[0].ObservedAt.Equal(earlierObserved) {
		t.Errorf("expected Monday's observed_at %s to win, got %s",
			earlierObserved, fs.recorded[0].ObservedAt)
	}
	if *cb {
		t.Errorf("expected OnSnapshot NOT to fire (no new write)")
	}
}

// TestHardwareRunOnce_DoesNotBackfillMissedWeeks pins the design decision: only
// the current week is recorded when the job runs, regardless of how many weeks
// the service was down.
func TestHardwareRunOnce_DoesNotBackfillMissedWeeks(t *testing.T) {
	ctx := context.Background()
	// Last recorded week was 4 weeks ago.
	store := &fakeHardwareStore{
		lastWeek: time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // current week starts 2026-06-15
	job, fs, _ := newHardwareTestJob(t, store, func() time.Time { return now })

	if err := job.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}

	if len(fs.recorded) != 1 {
		t.Fatalf("expected exactly one record call (current week only), got %d", len(fs.recorded))
	}
	if !fs.recorded[0].WeekStart.Equal(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("expected current week 2026-06-15, got %s", fs.recorded[0].WeekStart)
	}
}

// TestHardwareRunOnce_CallbackOnlyFiresOnSuccessfulWrite verifies the callback
// is not fired when RecordHardwareHistoryWeek returns an error — the cache
// invalidation is a no-op when the write failed.
func TestHardwareRunOnce_CallbackOnlyFiresOnSuccessfulWrite(t *testing.T) {
	ctx := context.Background()
	store := &fakeHardwareStore{recordError: errInjected}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job, _, cb := newHardwareTestJob(t, store, func() time.Time { return now })

	err := job.runOnce(ctx)
	if err == nil {
		t.Fatalf("expected runOnce to propagate the store error")
	}
	if *cb {
		t.Errorf("expected OnSnapshot NOT to fire when store returns error")
	}
}

// TestHardwareRunOnce_NoCallbackWhenZeroRowsInserted pins the all-stale-fleet
// branch: when RecordHardwareHistoryWeek succeeds but matches zero rows (empty
// fleet or every node filtered out by the last_seen_any_event_at staleness
// window), the OnSnapshot callback must NOT fire. The cache invalidation is a
// no-op when the underlying data didn't change.
func TestHardwareRunOnce_NoCallbackWhenZeroRowsInserted(t *testing.T) {
	ctx := context.Background()
	store := &fakeHardwareStore{insertZero: true}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job, _, cb := newHardwareTestJob(t, store, func() time.Time { return now })

	if err := job.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}
	if *cb {
		t.Errorf("expected OnSnapshot NOT to fire when zero rows were inserted")
	}
	if len(store.recorded) != 1 {
		t.Errorf("expected the record call to still happen (the writer itself is cheap), got %d", len(store.recorded))
	}
	if !store.lastWeek.IsZero() {
		t.Errorf("expected zero-row insert not to mark the current week as written, got %s", store.lastWeek)
	}
}

// TestHardwareRunOnce_PassesMaxAgeToWriter pins that the staleness window
// configured at job-construction time is what runOnce hands to the underlying
// RecordHardwareHistoryWeek. This is the write-side gate (on
// last_seen_any_event_at) for the history area chart — the value is locked in
// at job start so a config-reload half-way through a snapshot can't split a
// week across two policies.
func TestHardwareRunOnce_PassesMaxAgeToWriter(t *testing.T) {
	ctx := context.Background()
	store := &fakeHardwareStore{}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job := NewHardwareSnapshotJob(HardwareSnapshotOptions{
		Store:  store,
		Now:    func() time.Time { return now },
		MaxAge: 7 * 24 * time.Hour,
	})

	if err := job.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}

	if len(store.recorded) != 1 {
		t.Fatalf("expected one record call, got %d", len(store.recorded))
	}
	if store.recorded[0].MaxAge != 7*24*time.Hour {
		t.Errorf("expected MaxAge=7d, got %s", store.recorded[0].MaxAge)
	}
}

// TestNewHardwareSnapshotJob_AppliesDefaultMaxAgeWhenUnset pins the canonical
// fallback for the last_seen_any_event_at staleness window. A non-positive value
// in the options (e.g. an operator writing `max_age: 0` in YAML) must be
// normalized to 14d at job-construction time, otherwise the SQL cutoff in
// RecordHardwareHistoryWeek collapses to "now" and the weekly writer silently
// stops recording rows. Mirrors the firmware PR #111 regression guard.
func TestNewHardwareSnapshotJob_AppliesDefaultMaxAgeWhenUnset(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		maxAge time.Duration
		want   time.Duration
	}{
		{name: "zero", maxAge: 0, want: 14 * 24 * time.Hour},
		{name: "negative", maxAge: -5 * time.Hour, want: 14 * 24 * time.Hour},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeHardwareStore{}
			job := NewHardwareSnapshotJob(HardwareSnapshotOptions{
				Store:  store,
				Now:    func() time.Time { return now },
				MaxAge: tc.maxAge,
			})

			if err := job.runOnce(ctx); err != nil {
				t.Fatalf("runOnce: %v", err)
			}

			if len(store.recorded) != 1 {
				t.Fatalf("expected one record call, got %d", len(store.recorded))
			}
			if store.recorded[0].MaxAge != tc.want {
				t.Errorf("expected MaxAge=%s, got %s", tc.want, store.recorded[0].MaxAge)
			}
		})
	}
}

// TestHardwareStart_CatchesUpAndRespectsContextCancel verifies Start performs
// the catch-up immediately and exits cleanly when the context is cancelled. We
// use a clock pinned to a far-future Monday so the inner loop's timer would
// otherwise sleep for a week.
func TestHardwareStart_CatchesUpAndRespectsContextCancel(t *testing.T) {
	store := &fakeHardwareStore{}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	job, fs, _ := newHardwareTestJob(t, store, func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		job.Start(ctx)
		close(done)
	}()

	// Wait briefly for catch-up.
	waitFor(t, func() bool {
		fs.mu.Lock()
		defer fs.mu.Unlock()

		return len(fs.recorded) == 1
	}, 100*time.Millisecond, "catch-up record call")

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Start did not exit after context cancel")
	}
}

// TestHardwareStart_RetriesEmptyCurrentWeekSnapshot covers the cold-start case
// where nodes.last_seen_any_event_at is still NULL/zero for every row. The first
// current-week INSERT matches zero rows, but NodeInfo packets can arrive later
// in the same week; Start must retry before the next Monday so that week is
// still captured.
func TestHardwareStart_RetriesEmptyCurrentWeekSnapshot(t *testing.T) {
	store := &fakeHardwareStore{insertedRows: []int64{0, 1}}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	currentWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	wrote := make(chan struct{})
	var wroteOnce sync.Once
	job := NewHardwareSnapshotJob(HardwareSnapshotOptions{
		Store: store,
		Now:   func() time.Time { return now },
		OnSnapshot: func() {
			wroteOnce.Do(func() {
				close(wrote)
			})
		},
	})
	job.retryDelay = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		job.Start(ctx)
		close(done)
	}()

	waitFor(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()

		return len(store.recorded) >= 2 && store.lastWeek.Equal(currentWeek)
	}, 200*time.Millisecond, "retry record call")

	select {
	case <-wrote:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected OnSnapshot callback after retry inserts rows")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Start did not exit after context cancel")
	}
}
