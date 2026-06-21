// Package stats owns periodic stat-collection jobs. The first job is the
// weekly firmware snapshot; future jobs (e.g. hardware snapshot for issue
// #109) belong here too and follow the same shape.
package stats

import (
	"context"
	"log/slog"
	"time"
)

// WeekStartOf returns the Monday 00:00 UTC of the week containing t.
// Mirrors the same Monday-start week semantics used by the SQL store.
func WeekStartOf(t time.Time) time.Time {
	t = t.UTC()
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// offset is the number of days to subtract to reach Monday.
	offset := (int(t.Weekday()) + 6) % 7

	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.UTC)
}

// OnSnapshotFunc is the optional callback fired after a successful snapshot
// write. The HTTP layer uses it to invalidate the firmware caches so the
// freshly-written week becomes visible on the next API call.
type OnSnapshotFunc func()

// FirmwareSnapshotOptions configures a FirmwareSnapshotJob.
type FirmwareSnapshotOptions struct {
	Store      FirmwareWriter
	Logger     *slog.Logger
	Now        func() time.Time // optional; defaults to time.Now UTC
	OnSnapshot OnSnapshotFunc   // optional; fired after a successful write
	// MaxAge is the staleness window applied to nodes.last_map_report_at.
	// Nodes that haven't sent a MapReport in this duration are excluded
	// from node_firmware_history on each weekly snapshot (write-time
	// gate; the history read path does not re-filter at query time).
	// See web.stats.software.map_report_max_age.
	MaxAge time.Duration
}

// FirmwareWriter is the subset of repo.WriteStore the job needs. Defined
// here so the job has no hard dependency on internal/repo (or any future
// storage backend).
type FirmwareWriter interface {
	RecordFirmwareHistoryWeek(ctx context.Context, weekStart time.Time, observedAt time.Time, maxAge time.Duration) (int64, error)
	LastFirmwareHistoryWeek(ctx context.Context) (time.Time, error)
}

// FirmwareSnapshotJob runs the weekly INSERT OR IGNORE snapshot for the
// node_firmware_history table. It catches up on the current week on
// startup (if the most recent row in the table is older than the current
// Monday) and then sleeps until the next Monday 00:00 UTC.
//
// The job does NOT backfill older missed weeks: if the service was down
// for several weeks, the table stays sparse on the week axis and the
// API's buildVersionsByWeek helper pads the missing columns with zeros.
type FirmwareSnapshotJob struct {
	store      FirmwareWriter
	logger     *slog.Logger
	now        func() time.Time
	onSnapshot OnSnapshotFunc
	maxAge     time.Duration
}

// NewFirmwareSnapshotJob constructs a job. store is required.
func NewFirmwareSnapshotJob(opts FirmwareSnapshotOptions) *FirmwareSnapshotJob {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &FirmwareSnapshotJob{
		store:      opts.Store,
		logger:     logger,
		now:        now,
		onSnapshot: opts.OnSnapshot,
		maxAge:     opts.MaxAge,
	}
}

// Start launches the job's run loop. It performs the catch-up check
// immediately and then sleeps until the next Monday 00:00 UTC, repeating
// forever (until ctx is cancelled).
func (j *FirmwareSnapshotJob) Start(ctx context.Context) {
	if err := j.runOnce(ctx); err != nil {
		j.logger.Warn("firmware snapshot catch-up failed", "err", err)
	}

	for {
		now := j.now()
		next := nextMondayUTC(now)
		wait := next.Sub(now)
		if wait < 0 {
			wait = time.Hour
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			j.logger.Info("firmware snapshot job stopped")

			return
		case <-timer.C:
			if err := j.runOnce(ctx); err != nil {
				j.logger.Warn("firmware snapshot failed", "err", err)
			}
		}
	}
}

// runOnce performs one snapshot for the current week if it has not yet
// been written.
func (j *FirmwareSnapshotJob) runOnce(ctx context.Context) error {
	now := j.now()
	currentWeek := WeekStartOf(now)

	last, err := j.store.LastFirmwareHistoryWeek(ctx)
	if err != nil {
		return err
	}

	// If we already have a row for the current week (or a future one,
	// which can only happen if the clock moved backwards), no work to do.
	if !last.IsZero() && !last.Before(currentWeek) {
		j.logger.Debug("firmware snapshot already current",
			"current_week", currentWeek.Format("2006-01-02"),
			"last_week", last.Format("2006-01-02"),
		)

		return nil
	}

	observed := now
	inserted, err := j.store.RecordFirmwareHistoryWeek(ctx, currentWeek, observed, j.maxAge)
	if err != nil {
		return err
	}

	j.logger.Info("firmware snapshot written",
		"week_start", currentWeek.Format("2006-01-02"),
		"inserted_rows", inserted,
	)

	if j.onSnapshot != nil {
		j.onSnapshot()
	}

	return nil
}

// nextMondayUTC returns the Monday 00:00 UTC strictly after t.
func nextMondayUTC(t time.Time) time.Time {
	t = t.UTC()
	currentWeek := WeekStartOf(t)
	// Add 7 days to land on the next Monday.
	return currentWeek.AddDate(0, 0, 7)
}
