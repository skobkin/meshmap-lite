package httpapi

import (
	"context"
	"fmt"
	"time"

	"meshmap-lite/internal/domain"
)

type activityPeriodDefinition struct {
	key    string
	title  string
	window time.Duration
	bucket time.Duration
}

type activityPeriodCache struct {
	expiresAt time.Time
	period    activityPeriodPayload
}

const maxActivityBuckets = 1000

func (s *Server) activityPeriods() []activityPeriodDefinition {
	return []activityPeriodDefinition{
		{
			key:    "daily",
			title:  "24 hours",
			window: 24 * time.Hour,
			bucket: cappedBucket(24*time.Hour, s.cfg.Web.Stats.Activity.Daily.Bucket),
		},
		{
			key:    "weekly",
			title:  "7 days",
			window: 168 * time.Hour,
			bucket: cappedBucket(168*time.Hour, s.cfg.Web.Stats.Activity.Weekly.Bucket),
		},
	}
}

func cappedBucket(window, bucket time.Duration) time.Duration {
	if bucket <= 0 {
		return bucket
	}
	if window/bucket > maxActivityBuckets {
		return window / maxActivityBuckets
	}

	return bucket
}

func (s *Server) loadActivityPeriod(ctx context.Context, def activityPeriodDefinition, now time.Time) (activityPeriodPayload, error) {
	if def.window <= 0 || def.bucket <= 0 {
		return activityPeriodPayload{
			Key:     def.key,
			Title:   def.title,
			Window:  durationString(def.window),
			Bucket:  durationString(def.bucket),
			Buckets: []activityBucketPayload{},
		}, nil
	}

	s.activityMu.Lock()
	if cached, ok := s.activityCache[def.key]; ok && now.Before(cached.expiresAt) {
		s.activityMu.Unlock()

		return cached.period, nil
	}
	s.activityMu.Unlock()

	end := alignTime(now.UTC(), def.bucket)
	bucketCount := int(def.window / def.bucket)
	start := end.Add(-time.Duration(bucketCount) * def.bucket)
	buckets, err := s.store.ActivityBuckets(ctx, domain.ActivityQuery{
		Start:  start,
		End:    end,
		Bucket: def.bucket,
	})
	if err != nil {
		return activityPeriodPayload{}, err
	}
	period := activityPeriodPayload{
		Key:     def.key,
		Title:   def.title,
		Window:  durationString(def.window),
		Bucket:  durationString(def.bucket),
		Buckets: activityBucketPayloads(buckets),
	}
	cache := activityPeriodCache{
		expiresAt: end.Add(def.bucket),
		period:    period,
	}

	s.activityMu.Lock()
	s.activityCache[def.key] = cache
	s.activityMu.Unlock()

	return period, nil
}

func activityBucketPayloads(buckets []domain.ActivityBucket) []activityBucketPayload {
	out := make([]activityBucketPayload, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, activityBucketPayload{
			BucketStart:  bucket.BucketStart,
			TextMessages: bucket.TextMessages,
			PKI:          bucket.PKI,
			NodeInfo:     bucket.NodeInfo,
			Telemetry:    bucket.Telemetry,
			NeighborInfo: bucket.NeighborInfo,
			RangeTest:    bucket.RangeTest,
		})
	}

	return out
}

func alignTime(t time.Time, bucket time.Duration) time.Time {
	seconds := int64(bucket / time.Second)
	if seconds <= 0 {
		return t.UTC()
	}

	return time.Unix(t.UTC().Unix()/seconds*seconds, 0).UTC()
}

func durationString(d time.Duration) string {
	if d <= 0 {
		return d.String()
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", d/time.Second)
	}

	return d.String()
}
