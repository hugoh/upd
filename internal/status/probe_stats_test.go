package status

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startOfThisMinute() time.Time {
	return time.Now().Truncate(time.Minute)
}

// newSingleRingTracker creates a tracker with one ring for the given period
// and bucket interval.
func newSingleRingTracker(period, interval time.Duration) *RollingProbeTracker {
	return NewRollingProbeTracker(
		[]time.Duration{period},
		BucketConfig{Min: int(period / interval)},
	)
}

func TestBucketConfig_Interval_Defaults(t *testing.T) {
	var cfg BucketConfig

	assert.Equal(t, time.Second, cfg.Interval(time.Minute), "floored at 1s")
	assert.Equal(t, 9*time.Second, cfg.Interval(15*time.Minute), "period/100")
	assert.Equal(t, 36*time.Second, cfg.Interval(time.Hour), "period/100")
	assert.Equal(t, 864*time.Second, cfg.Interval(24*time.Hour), "period/100")
	assert.Equal(t, 30*time.Minute, cfg.Interval(168*time.Hour), "capped at maxSpan")
}

func TestBucketConfig_Interval_Custom(t *testing.T) {
	cfg := BucketConfig{Min: 10, MaxSpan: time.Hour}

	assert.Equal(t, 6*time.Second, cfg.Interval(time.Minute))
	assert.Equal(t, time.Hour, cfg.Interval(168*time.Hour))
}

func TestBucketConfig_BucketCount(t *testing.T) {
	var cfg BucketConfig

	assert.Equal(t, 61, cfg.BucketCount(time.Minute), "1s buckets")
	assert.Equal(t, 101, cfg.BucketCount(time.Hour), "period/100 buckets")
	assert.Equal(t, 337, cfg.BucketCount(168*time.Hour), "30m buckets")
	assert.Equal(t, 1, cfg.BucketCount(0), "degenerate period")
}

func TestRollingProbeTracker_Record(t *testing.T) {
	now := startOfThisMinute()
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	for range 3 {
		tracker.Record(false)
	}

	ps := tracker.Stats(time.Hour, now.Add(time.Minute))
	assert.Equal(t, 3, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_RecordFailed(t *testing.T) {
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	tracker.Record(false)
	tracker.Record(true)
	tracker.Record(false)

	ps := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 3, ps.Total)
	assert.Equal(t, 1, ps.Failed)
}

func TestRollingProbeTracker_Empty(t *testing.T) {
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	ps := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 0, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_UnknownPeriod(t *testing.T) {
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	tracker.Record(false)

	ps := tracker.Stats(2*time.Hour, time.Now())
	assert.Equal(t, 0, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_NoPeriods(t *testing.T) {
	tracker := NewRollingProbeTracker(nil, BucketConfig{})

	tracker.Record(false)

	ps := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 0, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_OutsideWindow(t *testing.T) {
	now := startOfThisMinute()
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	tracker.mu.Lock()
	tracker.rings[0].recordAt(now)
	tracker.mu.Unlock()

	ps := tracker.Stats(time.Hour, now.Add(2*time.Hour))
	assert.Equal(t, 0, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_BucketAdvancement(t *testing.T) {
	now := startOfThisMinute()
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	tracker.mu.Lock()
	tracker.rings[0].recordAt(now)                      // bucket 1
	tracker.rings[0].recordAt(now.Add(2 * time.Minute)) // bucket 3 (bucket 2 is empty gap)
	tracker.mu.Unlock()

	ps := tracker.Stats(time.Hour, now.Add(3*time.Minute))
	assert.Equal(t, 2, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_WrapAround(t *testing.T) {
	now := startOfThisMinute()
	tracker := newSingleRingTracker(5*time.Minute, time.Minute)
	// buckets = 5/1 + 1 = 6

	tracker.mu.Lock()

	for range 20 {
		tracker.rings[0].recordAt(now)
		now = now.Add(time.Minute)
	}

	// After 20 minutes of 1-min buckets with 6 buckets, only 6 remain even
	// for a cutoff far in the past.
	ps := tracker.rings[0].statsSince(now.Add(-time.Hour))
	tracker.mu.Unlock()

	assert.Equal(t, 6, ps.Total)

	// The 5m report window itself covers the 5 newest buckets.
	ps = tracker.Stats(5*time.Minute, now)
	assert.Equal(t, 5, ps.Total)
}

func TestRollingProbeTracker_LongGapResets(t *testing.T) {
	now := startOfThisMinute()
	tracker := newSingleRingTracker(5*time.Minute, time.Minute)

	tracker.mu.Lock()
	tracker.rings[0].recordAt(now)
	tracker.rings[0].recordAt(now.Add(time.Minute))
	// Gap longer than the whole ring.
	tracker.rings[0].recordAt(now.Add(24 * time.Hour))
	tracker.mu.Unlock()

	ps := tracker.Stats(5*time.Minute, now.Add(24*time.Hour))
	assert.Equal(t, 1, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_PerPeriodGranularity(t *testing.T) {
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tracker := NewRollingProbeTracker(
		[]time.Duration{time.Minute, time.Hour},
		BucketConfig{},
	)

	tracker.mu.Lock()

	for _, ring := range tracker.rings {
		ring.recordAt(base)
		ring.recordAt(base.Add(2 * time.Minute))
	}

	tracker.mu.Unlock()

	// The 1m window only sees the most recent probe.
	ps := tracker.Stats(time.Minute, base.Add(2*time.Minute))
	assert.Equal(t, 1, ps.Total)

	// The 1h window sees both.
	ps = tracker.Stats(time.Hour, base.Add(2*time.Minute))
	assert.Equal(t, 2, ps.Total)
}

func TestRollingProbeTracker_ConcurrentAccess(t *testing.T) {
	tracker := newSingleRingTracker(time.Hour, time.Minute)

	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			for range 100 {
				tracker.Record(true)
			}
		})
	}

	wg.Wait()

	ps := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 1000, ps.Total)
	assert.Equal(t, 1000, ps.Failed)
}

func TestRollingProbeTracker_AlignedWindow(t *testing.T) {
	tracker := newSingleRingTracker(time.Hour, 5*time.Second)

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tracker.mu.Lock()
	// Bucket [12:00:00, 12:00:05)
	tracker.rings[0].recordAt(base.Add(4 * time.Second))
	// Bucket [12:00:05, 12:00:10)
	tracker.rings[0].recordAt(base.Add(9 * time.Second))
	tracker.mu.Unlock()

	// 1h window from 12:55:05 → cutoff = 12:00:05 (later bucket boundary)
	ps := tracker.rings[0].statsSince(base.Add(5 * time.Second))
	assert.Equal(t, 1, ps.Total)
	assert.Equal(t, 0, ps.Failed)

	// Cutoff = 12:00:00 (first bucket boundary): both included
	ps = tracker.rings[0].statsSince(base)
	assert.Equal(t, 2, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_NonAlignedWindow(t *testing.T) {
	tracker := newSingleRingTracker(time.Hour, 5*time.Second)

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tracker.mu.Lock()
	// Bucket [12:00:00, 12:00:05)
	tracker.rings[0].recordAt(base.Add(4 * time.Second))
	// Bucket [12:00:05, 12:00:10)
	tracker.rings[0].recordAt(base.Add(6 * time.Second))
	tracker.mu.Unlock()

	// Cutoff = 12:00:03 (middle of first bucket):
	// Bucket [12:00:00, 12:00:05): starts before cutoff → excluded
	// Bucket [12:00:05, 12:00:10): starts at or after cutoff → included
	ps := tracker.rings[0].statsSince(base.Add(3 * time.Second))
	assert.Equal(t, 1, ps.Total)
	assert.Equal(t, 0, ps.Failed)

	// Cutoff = 11:59:59: both buckets included
	ps = tracker.rings[0].statsSince(base.Add(-time.Second))
	assert.Equal(t, 2, ps.Total)
	assert.Equal(t, 0, ps.Failed)
}

func TestRollingProbeTracker_StatsAll(t *testing.T) {
	now := time.Now()
	tracker := NewRollingProbeTracker(
		[]time.Duration{time.Minute, time.Hour},
		BucketConfig{},
	)

	tracker.Record(false)
	tracker.Record(true)

	all := tracker.StatsAll(now.Add(time.Second))
	require.Len(t, all, 2)
	assert.Equal(t, 2, all[0].Total)
	assert.Equal(t, 1, all[0].Failed)
	assert.Equal(t, 2, all[1].Total)
	assert.Equal(t, 1, all[1].Failed)
}

func TestRollingProbeTracker_StatsAll_Empty(t *testing.T) {
	tracker := NewRollingProbeTracker([]time.Duration{time.Hour}, BucketConfig{})
	all := tracker.StatsAll(time.Now())
	require.Len(t, all, 1)
	assert.Equal(t, 0, all[0].Total)
}
