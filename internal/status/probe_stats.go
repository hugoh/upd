package status

import (
	"cmp"
	"fmt"
	"sync"
	"time"
)

// ProbeStats holds the result of a probe stats query.
type ProbeStats struct {
	Total  int
	Failed int
}

// probeBucket counts probe results within one bucket interval. Bucket start
// times are not stored: they are derived from the ring's lastTime and the
// bucket's distance from the newest bucket, keeping rings at 8 bytes per
// bucket.
type probeBucket struct {
	total  uint32
	failed uint32
}

const (
	// DefaultBucketsMin is the default minimum number of buckets each report
	// period is split into.
	DefaultBucketsMin = 100
	// DefaultBucketMaxSpan is the default maximum time span a single bucket
	// aggregates.
	DefaultBucketMaxSpan = 30 * time.Minute
	// MinBucketInterval is the smallest bucket interval.
	MinBucketInterval = time.Second
	// MaxBucketsPerPeriod bounds the ring size for a single report period
	// (~800 KiB at 8 bytes per bucket).
	MaxBucketsPerPeriod = 100_000
)

// BucketConfig tunes probe-stat bucket granularity. Zero values mean
// defaults.
type BucketConfig struct {
	// Min is the minimum number of buckets a report period is split into.
	Min int
	// MaxSpan is the maximum time span a single bucket aggregates.
	MaxSpan time.Duration
}

// Interval returns the bucket interval for the given report period:
// period/Min, capped at MaxSpan and floored at MinBucketInterval.
func (c BucketConfig) Interval(period time.Duration) time.Duration {
	minBuckets := cmp.Or(c.Min, DefaultBucketsMin)
	maxSpan := cmp.Or(c.MaxSpan, DefaultBucketMaxSpan)

	interval := min(period/time.Duration(minBuckets), maxSpan)

	return max(interval, MinBucketInterval)
}

// BucketCount returns the ring size needed to cover the given report period.
func (c BucketConfig) BucketCount(period time.Duration) int {
	return max(int(period/c.Interval(period))+1, 1)
}

// probeRing is a fixed-size ring of time-bucketed counters covering one
// report period.
type probeRing struct {
	period   time.Duration
	interval time.Duration
	buckets  []probeBucket
	head     int
	count    int
	lastTime time.Time // start time of the newest bucket
}

// RollingProbeTracker tracks probe success/failure rates per report period.
// Each period gets its own ring of bucketed counters, so bucket granularity
// is proportional to the period it serves instead of one global resolution
// paying for the longest retention. Thread-safe.
type RollingProbeTracker struct {
	mu    sync.Mutex
	rings []*probeRing
}

// NewRollingProbeTracker creates a tracker with one ring per report period.
func NewRollingProbeTracker(periods []time.Duration, cfg BucketConfig) *RollingProbeTracker {
	rings := make([]*probeRing, 0, len(periods))

	for _, period := range periods {
		rings = append(rings, &probeRing{
			period:   period,
			interval: cfg.Interval(period),
			buckets:  make([]probeBucket, cfg.BucketCount(period)),
		})
	}

	return &RollingProbeTracker{rings: rings}
}

// Record records a probe result.
func (t *RollingProbeTracker) Record(failed bool) {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, ring := range t.rings {
		ring.recordAt(now)

		if failed {
			ring.buckets[ring.newestIdx()].failed++
		}
	}
}

// Stats returns probe results within the given report period before now.
// Panics if period is not one of the configured report periods — use StatsAll
// for bulk reads that avoid this constraint.
func (t *RollingProbeTracker) Stats(period time.Duration, now time.Time) ProbeStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, ring := range t.rings {
		if ring.period == period {
			return ring.statsSince(now.Add(-period))
		}
	}

	panic(fmt.Sprintf("RollingProbeTracker.Stats: no ring for period %v", period))
}

// StatsAll returns probe results for every configured ring in construction
// order, under a single lock acquisition. Use this for report generation to
// ensure all periods reflect the same ring state.
func (t *RollingProbeTracker) StatsAll(now time.Time) []ProbeStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]ProbeStats, len(t.rings))

	for i, ring := range t.rings {
		result[i] = ring.statsSince(now.Add(-ring.period))
	}

	return result
}

// newestIdx returns the ring index of the newest bucket. Must be called with
// the tracker lock held and count > 0.
func (r *probeRing) newestIdx() int {
	return (r.head + r.count - 1) % len(r.buckets)
}

// recordAt counts a probe at the given time. Must be called with the tracker
// lock held.
func (r *probeRing) recordAt(now time.Time) {
	bucketTime := now.Truncate(r.interval)

	switch {
	case r.count == 0:
		r.head = 0
		r.count = 1
		r.lastTime = bucketTime
		r.buckets[0] = probeBucket{}
	case bucketTime.After(r.lastTime):
		r.advanceTo(bucketTime)
	default:
		// Same bucket as the last record, or earlier (clock drift, which
		// should never happen): count into the newest bucket.
	}

	r.buckets[r.newestIdx()].total++
}

// advanceTo appends empty buckets up to bucketTime, evicting the oldest ones
// once the ring is full. Must be called with the tracker lock held,
// count > 0, and bucketTime after lastTime.
func (r *probeRing) advanceTo(bucketTime time.Time) {
	maxBuckets := len(r.buckets)
	steps := int(bucketTime.Sub(r.lastTime) / r.interval)
	r.lastTime = bucketTime

	if steps >= maxBuckets {
		// The gap spans the whole ring: drop everything.
		r.head = 0
		r.count = 1
		r.buckets[0] = probeBucket{}

		return
	}

	if r.count == maxBuckets {
		// Ring is full: batch-zero the evicted slots and rotate head in one
		// step instead of looping. The evicted range [head, head+steps) wraps
		// around, so handle the two contiguous halves separately.
		newHead := (r.head + steps) % maxBuckets
		if newHead > r.head {
			clear(r.buckets[r.head:newHead])
		} else {
			clear(r.buckets[r.head:])
			if newHead > 0 {
				clear(r.buckets[:newHead])
			}
		}
		r.head = newHead

		return
	}

	for range steps {
		r.buckets[(r.head+r.count)%maxBuckets] = probeBucket{}
		r.count++
	}
}

// statsSince sums the buckets that start at or after cutoff. Must be called
// with the tracker lock held.
func (r *probeRing) statsSince(cutoff time.Time) ProbeStats {
	if r.count == 0 {
		return ProbeStats{}
	}

	oldest := r.lastTime.Add(-time.Duration(r.count-1) * r.interval)

	skip := 0
	if diff := cutoff.Sub(oldest); diff > 0 {
		// Ceil: a bucket is included only if it starts at or after the cutoff.
		skip = min(int((diff+r.interval-1)/r.interval), r.count)
	}

	var result ProbeStats

	for i := skip; i < r.count; i++ {
		bucket := r.buckets[(r.head+i)%len(r.buckets)]
		result.Total += int(bucket.total)
		result.Failed += int(bucket.failed)
	}

	return result
}
