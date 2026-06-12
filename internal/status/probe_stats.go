package status

import (
	"sync"
	"time"
)

type probeBucket struct {
	timestamp time.Time
	total     uint32
	failed    uint32
}

// RollingProbeTracker tracks probe success/failure rates within a rolling time
// window using time-bucketed counters in a ring buffer. Thread-safe.
type RollingProbeTracker struct {
	mu             sync.Mutex
	bucketInterval time.Duration
	maxBuckets     int
	buckets        []probeBucket
	head           int
	tail           int
	count          int
}

// NewRollingProbeTracker creates a tracker that retains data for the given
// duration using buckets of the given interval.
func NewRollingProbeTracker(retention, bucketInterval time.Duration) *RollingProbeTracker {
	maxBuckets := max(int(retention/bucketInterval)+1, 1)

	return &RollingProbeTracker{
		bucketInterval: bucketInterval,
		maxBuckets:     maxBuckets,
		buckets:        make([]probeBucket, maxBuckets),
	}
}

// BucketInterval returns the probe bucket interval to use for the given
// report periods. Defaults to 1 minute when no periods are given.
func BucketInterval(periods []time.Duration) time.Duration {
	if len(periods) == 0 {
		return time.Minute
	}

	minPeriod := periods[0]
	for _, p := range periods[1:] {
		if p < minPeriod {
			minPeriod = p
		}
	}

	return max(minPeriod, time.Second)
}

// Record records a probe result.
func (t *RollingProbeTracker) Record(failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.recordAt(time.Now())

	if failed {
		idx := (t.tail - 1 + t.maxBuckets) % t.maxBuckets
		t.buckets[idx].failed++
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

	var total, failed int

	for i := range t.count {
		idx := (t.head + i) % t.maxBuckets
		b := t.buckets[idx]

		if !b.timestamp.Before(cutoff) {
			total += int(b.total)
			failed += int(b.failed)
		}
	}

	return total, failed
}

// recordAt records a probe result at the given time. Must be called with the
// lock held.
func (t *RollingProbeTracker) recordAt(now time.Time) {
	bucketTime := now.Truncate(t.bucketInterval)

	if t.count == 0 {
		t.buckets[t.tail] = probeBucket{timestamp: bucketTime, total: 1}
		t.tail = (t.tail + 1) % t.maxBuckets
		t.count = 1

		return
	}

	lastIdx := (t.tail - 1 + t.maxBuckets) % t.maxBuckets
	lastTime := t.buckets[lastIdx].timestamp

	if bucketTime.Equal(lastTime) {
		t.buckets[lastIdx].total++

		return
	}

	if bucketTime.After(lastTime) {
		bi := t.bucketInterval

		for nextTime := lastTime.Add(bi); !nextTime.After(bucketTime); nextTime = nextTime.Add(bi) {
			if t.count == t.maxBuckets {
				t.head = (t.head + 1) % t.maxBuckets
				t.count--
			}

			t.buckets[t.tail] = probeBucket{timestamp: nextTime}
			t.tail = (t.tail + 1) % t.maxBuckets
			t.count++
		}

		lastIdx = (t.tail - 1 + t.maxBuckets) % t.maxBuckets
		t.buckets[lastIdx].total++

		return
	}

	// bucketTime before lastTime should never happen (clock drift guard)
	t.buckets[lastIdx].total++
}
