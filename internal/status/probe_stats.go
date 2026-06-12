package status

import (
	"sync"
	"time"
)

const bucketInterval = time.Minute

type probeBucket struct {
	timestamp time.Time
	total     uint32
	failed    uint32
}

// RollingProbeTracker tracks probe success/failure rates within a rolling time
// window using time-bucketed counters (1-minute buckets) in a ring buffer.
// Thread-safe.
type RollingProbeTracker struct {
	mu         sync.Mutex
	maxBuckets int
	buckets    []probeBucket
	head       int
	tail       int
	count      int
}

// NewRollingProbeTracker creates a tracker that retains data for the given
// duration using 1-minute buckets.
func NewRollingProbeTracker(retention time.Duration) *RollingProbeTracker {
	maxBuckets := max(int(retention/bucketInterval)+1, 1)

	return &RollingProbeTracker{
		maxBuckets: maxBuckets,
		buckets:    make([]probeBucket, maxBuckets),
	}
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
	bucketTime := now.Truncate(bucketInterval)

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
		for nextTime := lastTime.Add(bucketInterval); !nextTime.After(bucketTime); nextTime = nextTime.Add(bucketInterval) {
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
