package stats

import (
	"context"
	"log/slog"
	"time"
)

// HardwareSnapshotOptions configures a HardwareSnapshotJob.
type HardwareSnapshotOptions struct {
	Store      HardwareWriter
	Logger     *slog.Logger
	Now        func() time.Time // optional; defaults to time.Now UTC
	OnSnapshot OnSnapshotFunc   // optional; fired after a successful write
	// MaxAge is the staleness window applied to nodes.last_seen_any_event_at.
	// Nodes that haven't been seen on any event in this duration are excluded
	// from node_hardware_history on each weekly snapshot (write-time gate; the
	// history read path does not re-filter at query time). Unlike firmware,
	// hardware arrives with NodeInfo and covers nearly every known node, so the
	// broadest liveness column is the right gate. See web.stats.hardware.max_age.
	MaxAge time.Duration
}

// HardwareWriter is the subset of repo.WriteStore the job needs. Defined
// here so the job has no hard dependency on internal/repo (or any future
// storage backend).
type HardwareWriter interface {
	RecordHardwareHistoryWeek(ctx context.Context, weekStart time.Time, observedAt time.Time, maxAge time.Duration) (int64, error)
	LastHardwareHistoryWeek(ctx context.Context) (time.Time, error)
}

// defaultHardwareMaxAge is the canonical fallback for the staleness window
// applied to nodes.last_seen_any_event_at when no explicit value is supplied.
// Matches web.stats.hardware.max_age's config default (see
// internal/config/defaults.go). The HTTP layer applies the same fallback in
// hardwareSnapshot; both sites must agree or the snapshot endpoint and the
// weekly writer will silently disagree about which nodes are "active".
const defaultHardwareMaxAge = 14 * 24 * time.Hour

type hardwareSnapshotRunStatus uint8

const (
	hardwareSnapshotDone hardwareSnapshotRunStatus = iota
	hardwareSnapshotRetry
)

// HardwareSnapshotJob runs the weekly INSERT OR IGNORE snapshot for the
// node_hardware_history table. It catches up on the current week on startup
// (if the most recent row in the table is older than the current Monday) and
// then sleeps until the next Monday 00:00 UTC.
//
// The job does NOT backfill older missed weeks: if the service was down for
// several weeks, the table stays sparse on the week axis and the API's
// buildModelsByWeek helper pads the missing columns with zeros.
type HardwareSnapshotJob struct {
	store      HardwareWriter
	logger     *slog.Logger
	now        func() time.Time
	onSnapshot OnSnapshotFunc
	maxAge     time.Duration
	retryDelay time.Duration
}

// NewHardwareSnapshotJob constructs a job. store is required.
// A non-positive MaxAge is replaced with defaultHardwareMaxAge so a
// config-driven 0 (e.g. an explicit `max_age: 0` in YAML) does not collapse
// the SQL cutoff to "now" and silently stop recording history rows. The HTTP
// layer applies the same fallback in hardwareSnapshot; this constructor is the
// canonical default.
func NewHardwareSnapshotJob(opts HardwareSnapshotOptions) *HardwareSnapshotJob {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = defaultHardwareMaxAge
	}

	return &HardwareSnapshotJob{
		store:      opts.Store,
		logger:     logger,
		now:        now,
		onSnapshot: opts.OnSnapshot,
		maxAge:     maxAge,
		retryDelay: snapshotRetryDelay,
	}
}

// Start launches the job's run loop. It performs the catch-up check
// immediately and then sleeps until the next Monday 00:00 UTC, repeating
// forever (until ctx is cancelled).
func (j *HardwareSnapshotJob) Start(ctx context.Context) {
	for {
		status, err := j.runOnceResult(ctx)
		if err != nil {
			j.logger.Warn("hardware snapshot failed", "err", err)
		}

		wait := j.nextWait(status, err)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			j.logger.Info("hardware snapshot job stopped")

			return
		case <-timer.C:
		}
	}
}

// runOnce performs one snapshot for the current week if it has not yet
// been written.
func (j *HardwareSnapshotJob) runOnce(ctx context.Context) error {
	_, err := j.runOnceResult(ctx)

	return err
}

func (j *HardwareSnapshotJob) runOnceResult(ctx context.Context) (hardwareSnapshotRunStatus, error) {
	now := j.now()
	currentWeek := WeekStartOf(now)

	last, err := j.store.LastHardwareHistoryWeek(ctx)
	if err != nil {
		return hardwareSnapshotDone, err
	}

	// If we already have a row for the current week (or a future one, which
	// can only happen if the clock moved backwards), no work to do.
	if !last.IsZero() && !last.Before(currentWeek) {
		j.logger.Debug("hardware snapshot already current",
			"current_week", currentWeek.Format("2006-01-02"),
			"last_week", last.Format("2006-01-02"),
		)

		return hardwareSnapshotDone, nil
	}

	observed := now
	inserted, err := j.store.RecordHardwareHistoryWeek(ctx, currentWeek, observed, j.maxAge)
	if err != nil {
		return hardwareSnapshotDone, err
	}

	// Only announce the write and invalidate caches when a row was actually
	// inserted. With an empty or all-stale fleet the INSERT OR IGNORE matches
	// zero rows: the "written" log line would be misleading and the OnSnapshot
	// callback (which clears the hardware response caches) would be a no-op
	// since the data on disk didn't change. Demoted to Debug so the absence of
	// rows is still observable for ops triage. Return a retry status so a
	// startup before fresh NodeInfo arrives does not skip the entire current
	// week.
	if inserted == 0 {
		j.logger.Debug("hardware snapshot skipped: no active nodes",
			"week_start", currentWeek.Format("2006-01-02"),
			"max_age", j.maxAge,
		)

		return hardwareSnapshotRetry, nil
	}

	j.logger.Info("hardware snapshot written",
		"week_start", currentWeek.Format("2006-01-02"),
		"inserted_rows", inserted,
	)

	if j.onSnapshot != nil {
		j.onSnapshot()
	}

	return hardwareSnapshotDone, nil
}

func (j *HardwareSnapshotJob) nextWait(status hardwareSnapshotRunStatus, err error) time.Duration {
	if status == hardwareSnapshotRetry || err != nil {
		if j.retryDelay > 0 {
			return j.retryDelay
		}

		return snapshotRetryDelay
	}

	now := j.now()
	wait := nextMondayUTC(now).Sub(now)
	if wait < 0 {
		return time.Hour
	}

	return wait
}
