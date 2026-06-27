package stats

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeStore records every RecordFirmwareHistoryWeek / LastFirmwareHistoryWeek
// call so the tests can assert on the schedule without coupling to the
// real SQLite store.
type fakeStore struct {
	mu sync.Mutex

	// lastWeek is the value returned by LastFirmwareHistoryWeek. Zero
	// time means "no rows yet".
	lastWeek time.Time

	// recorded captures the (weekStart, observedAt) arguments of each
	// RecordFirmwareHistoryWeek call.
	recorded []recordedCall

	// recordError and lastError are returned from the corresponding
	// methods when set; nil otherwise.
	recordError error
	lastError   error

	// insertZero forces RecordFirmwareHistoryWeek to return 0 rows.
	// insertedRows, when set, returns a sequence of row counts. Both
	// simulate the empty-fleet / all-stale-fleet case where the SQL
	// store's INSERT OR IGNORE matches zero rows.
	insertZero   bool
	insertedRows []int64
}

type recordedCall struct {
	WeekStart  time.Time
	ObservedAt time.Time
	MaxAge     time.Duration
}

func (f *fakeStore) RecordFirmwareHistoryWeek(_ context.Context, weekStart, observedAt time.Time, maxAge time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordError != nil {
		return 0, f.recordError
	}
	f.recorded = append(f.recorded, recordedCall{WeekStart: weekStart, ObservedAt: observedAt, MaxAge: maxAge})

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

func (f *fakeStore) LastFirmwareHistoryWeek(_ context.Context) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.lastWeek, f.lastError
}

// newTestJob builds a FirmwareSnapshotJob pinned to the given clock and
// a no-op callback.
func newTestJob(t *testing.T, store FirmwareWriter, now func() time.Time) (*FirmwareSnapshotJob, *fakeStore, *bool) {
	t.Helper()
	fs, ok := store.(*fakeStore)
	if !ok {
		t.Fatalf("expected *fakeStore, got %T", store)
	}
	called := false

	return NewFirmwareSnapshotJob(FirmwareSnapshotOptions{
		Store: store,
		Now:   now,
		OnSnapshot: func() {
			called = true
		},
	}), fs, &called
}

// TestRunOnce_CatchesUpOnFirstRun covers the cold-start scenario: empty
// history, current week not yet recorded. The job must write the
// current week's snapshot and fire the OnSnapshot callback.
func TestRunOnce_CatchesUpOnFirstRun(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	job, fs, cb := newTestJob(t, store, func() time.Time { return now })

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

// TestRunOnce_NoopIfCurrentWeekAlreadyRecorded covers the warm-start
// scenario: history already has a row for the current week, runOnce
// must be a no-op (no record call, no callback).
func TestRunOnce_NoopIfCurrentWeekAlreadyRecorded(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{
		lastWeek: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), // Monday
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday of same week
	job, fs, cb := newTestJob(t, store, func() time.Time { return now })

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

// TestRunOnce_FirstWriterWinsOnMidWeekVersionChange is the regression
// test for the user's explicit concern: if a node changes its
// firmware_version_id mid-week, a second run for the same week must
// NOT overwrite the first writer's row. With the fake store this is
// modelled by leaving recorded untouched (the real SQL store enforces
// the same invariant via INSERT OR IGNORE).
//
// Practically, the job's runOnce sees the table already has the
// current week recorded and returns early — so a stale node state in
// nodes is irrelevant. This test pins that behaviour explicitly.
func TestRunOnce_FirstWriterWinsOnMidWeekVersionChange(t *testing.T) {
	ctx := context.Background()
	currentWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{lastWeek: currentWeek}

	// Simulate "earlier this week, a row was written" by pre-populating
	// recorded with the Monday snapshot.
	earlierObserved := currentWeek.Add(2 * time.Hour)
	store.recorded = append(store.recorded, recordedCall{
		WeekStart:  currentWeek,
		ObservedAt: earlierObserved,
	})

	// Now a new NodeInfo packet arrives mid-week — but the job must
	// NOT issue a new record call (the SQL store would also drop it
	// via INSERT OR IGNORE).
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job, fs, cb := newTestJob(t, store, func() time.Time { return now })

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

// TestRunOnce_DoesNotBackfillMissedWeeks pins the design decision: only
// the current week is recorded when the job runs, regardless of how
// many weeks the service was down.
func TestRunOnce_DoesNotBackfillMissedWeeks(t *testing.T) {
	ctx := context.Background()
	// Last recorded week was 4 weeks ago.
	store := &fakeStore{
		lastWeek: time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // current week starts 2026-06-15
	job, fs, _ := newTestJob(t, store, func() time.Time { return now })

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

// TestRunOnce_CallbackOnlyFiresOnSuccessfulWrite verifies the callback
// is not fired when RecordFirmwareHistoryWeek returns an error — the
// cache invalidation is a no-op when the write failed.
func TestRunOnce_CallbackOnlyFiresOnSuccessfulWrite(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{recordError: errInjected}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job, _, cb := newTestJob(t, store, func() time.Time { return now })

	err := job.runOnce(ctx)
	if err == nil {
		t.Fatalf("expected runOnce to propagate the store error")
	}
	if *cb {
		t.Errorf("expected OnSnapshot NOT to fire when store returns error")
	}
}

// TestRunOnce_NoCallbackWhenZeroRowsInserted pins the all-stale-fleet
// branch: when RecordFirmwareHistoryWeek succeeds but matches zero
// rows (empty fleet or every node filtered out by the staleness
// window), the OnSnapshot callback must NOT fire and the "snapshot
// written" log line must NOT be emitted at info level. The cache
// invalidation is a no-op when the underlying data didn't change.
func TestRunOnce_NoCallbackWhenZeroRowsInserted(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{insertZero: true}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job, _, cb := newTestJob(t, store, func() time.Time { return now })

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

// TestRunOnce_PassesMaxAgeToWriter pins that the staleness window
// configured at job-construction time is what runOnce hands to the
// underlying RecordFirmwareHistoryWeek. This is the write-side gate for
// the history area chart — the value is locked in at job start so a
// config-reload half-way through a snapshot can't split a week across
// two policies.
func TestRunOnce_PassesMaxAgeToWriter(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	job := NewFirmwareSnapshotJob(FirmwareSnapshotOptions{
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

// TestNewFirmwareSnapshotJob_AppliesDefaultMaxAgeWhenUnset pins the
// canonical fallback for MapReportMaxAge. A non-positive value in the
// options (e.g. an operator writing `map_report_max_age: 0` in YAML)
// must be normalized to 14d at job-construction time, otherwise the
// SQL cutoff in RecordFirmwareHistoryWeek collapses to "now" and the
// weekly writer silently stops recording rows. Regression test for
// the CodeX review of PR #111.
func TestNewFirmwareSnapshotJob_AppliesDefaultMaxAgeWhenUnset(t *testing.T) {
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
			store := &fakeStore{}
			job := NewFirmwareSnapshotJob(FirmwareSnapshotOptions{
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

// TestStart_CatchesUpAndRespectsContextCancel verifies Start performs
// the catch-up immediately and exits cleanly when the context is
// cancelled. We use a clock pinned to a far-future Monday so the inner
// loop's timer would otherwise sleep for a week.
func TestStart_CatchesUpAndRespectsContextCancel(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	job, fs, _ := newTestJob(t, store, func() time.Time { return now })

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

// TestStart_RetriesEmptyCurrentWeekSnapshot covers the cold-start
// migration case where nodes.last_map_report_at is still NULL for
// every row. The first current-week INSERT matches zero rows, but
// MapReports can arrive later in the same week; Start must retry
// before the next Monday so that week is still captured.
func TestStart_RetriesEmptyCurrentWeekSnapshot(t *testing.T) {
	store := &fakeStore{insertedRows: []int64{0, 1}}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC) // Wednesday
	currentWeek := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	wrote := make(chan struct{})
	var wroteOnce sync.Once
	job := NewFirmwareSnapshotJob(FirmwareSnapshotOptions{
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

// waitFor polls cond every 5ms until it returns true or timeout elapses.
func waitFor(t *testing.T, cond func() bool, timeout time.Duration, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// errInjected is a sentinel error returned by the fake store's
// RecordFirmwareHistoryWeek when recordError is set.
var errInjected = sentinelError("injected")

type sentinelError string

func (e sentinelError) Error() string { return string(e) }
