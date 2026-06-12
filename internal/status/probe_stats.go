package status

import (
	"slices"
	"sync"
	"time"
)

// probeBucket counts probe results within one bucket interval. Bucket start
// times are not stored: they are derived from RollingProbeTracker.lastTime
// and the bucket's distance from the newest bucket, keeping the ring buffer
// at 8 bytes per bucket.
type probeBucket struct {
	total  uint32
	failed uint32
}

// MaxBucketCount caps the ring buffer size regardless of configuration
// (~800 KiB at 8 bytes per bucket). When a configuration would exceed it,
// the bucket interval is enlarged instead.
const MaxBucketCount = 100_000

// RollingProbeTracker tracks probe success/failure rates within a rolling time
// window using time-bucketed counters in a ring buffer. Thread-safe.
type RollingProbeTracker struct {
	mu             sync.Mutex
	bucketInterval time.Duration
	maxBuckets     int
	buckets        []probeBucket
	head           int
	count          int
	lastTime       time.Time // start time of the newest bucket
}

// NewRollingProbeTracker creates a tracker that retains data for the given
// duration using buckets of the given interval, enlarged if needed to keep
// the bucket count under MaxBucketCount.
func NewRollingProbeTracker(retention, bucketInterval time.Duration) *RollingProbeTracker {
	if minInterval := retention / (MaxBucketCount - 1); bucketInterval < minInterval {
		bucketInterval = minInterval
	}

	maxBuckets := min(max(int(retention/bucketInterval)+1, 1), MaxBucketCount)

	return &RollingProbeTracker{
		bucketInterval: bucketInterval,
		maxBuckets:     maxBuckets,
		buckets:        make([]probeBucket, maxBuckets),
	}
}

// BucketIntervalDivisor controls bucket granularity relative to the shortest
// report period; the rolling-window error is bounded to ~1/Divisor of it.
const BucketIntervalDivisor = 10

// BucketInterval returns the probe bucket interval to use for the given
// report periods. Defaults to 1 minute when no periods are given.
func BucketInterval(periods []time.Duration) time.Duration {
	if len(periods) == 0 {
		return time.Minute
	}

	return max(slices.Min(periods)/BucketIntervalDivisor, time.Second)
}

// Record records a probe result.
func (t *RollingProbeTracker) Record(failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.recordAt(time.Now())

	if failed {
		t.buckets[t.newestIdx()].failed++
	}
}

// Stats returns (total, failed) probe results within the given duration
// before now.
func (t *RollingProbeTracker) Stats(since time.Duration, now time.Time) (int, int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.count == 0 {
		return 0, 0
	}

	cutoff := now.Add(-since)
	oldest := t.lastTime.Add(-time.Duration(t.count-1) * t.bucketInterval)

	skip := 0
	if diff := cutoff.Sub(oldest); diff > 0 {
		// Ceil: a bucket is included only if it starts at or after the cutoff.
		skip = min(int((diff+t.bucketInterval-1)/t.bucketInterval), t.count)
	}

	var total, failed int

	for i := skip; i < t.count; i++ {
		bucket := t.buckets[(t.head+i)%t.maxBuckets]
		total += int(bucket.total)
		failed += int(bucket.failed)
	}

	return total, failed
}

// newestIdx returns the ring index of the newest bucket. Must be called with
// the lock held and count > 0.
func (t *RollingProbeTracker) newestIdx() int {
	return (t.head + t.count - 1) % t.maxBuckets
}

// recordAt counts a probe at the given time. Must be called with the lock
// held.
func (t *RollingProbeTracker) recordAt(now time.Time) {
	bucketTime := now.Truncate(t.bucketInterval)

	switch {
	case t.count == 0:
		t.head = 0
		t.count = 1
		t.lastTime = bucketTime
		t.buckets[0] = probeBucket{}
	case bucketTime.After(t.lastTime):
		t.advanceTo(bucketTime)
	default:
		// Same bucket as the last record, or earlier (clock drift, which
		// should never happen): count into the newest bucket.
	}

	t.buckets[t.newestIdx()].total++
}

// advanceTo appends empty buckets up to bucketTime, evicting the oldest ones
// once the ring is full. Must be called with the lock held, count > 0, and
// bucketTime after lastTime.
func (t *RollingProbeTracker) advanceTo(bucketTime time.Time) {
	steps := int(bucketTime.Sub(t.lastTime) / t.bucketInterval)
	t.lastTime = bucketTime

	if steps >= t.maxBuckets {
		// The gap spans the whole window: drop everything.
		t.head = 0
		t.count = 1
		t.buckets[0] = probeBucket{}

		return
	}

	for range steps {
		if t.count == t.maxBuckets {
			t.buckets[t.head] = probeBucket{}
			t.head = (t.head + 1) % t.maxBuckets
		} else {
			t.buckets[(t.head+t.count)%t.maxBuckets] = probeBucket{}
			t.count++
		}
	}
}
