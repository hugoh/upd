package status

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func startOfThisMinute() time.Time {
	return time.Now().Truncate(time.Minute)
}

func TestBucketInterval_Defaults(t *testing.T) {
	assert.Equal(t, time.Minute, BucketInterval(nil))
	assert.Equal(t, time.Minute, BucketInterval([]time.Duration{}))
}

func TestBucketInterval_FromPeriods(t *testing.T) {
	assert.Equal(
		t,
		15*time.Second,
		BucketInterval([]time.Duration{time.Minute, 15 * time.Second, 5 * time.Minute}),
	)
	assert.Equal(t, time.Second, BucketInterval([]time.Duration{100 * time.Millisecond}))
}

func TestRollingProbeTracker_Record(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(time.Hour, time.Minute)

	for range 3 {
		tracker.Record(false)
	}

	total, failed := tracker.Stats(time.Hour, now.Add(time.Minute))
	assert.Equal(t, 3, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_RecordFailed(t *testing.T) {
	tracker := NewRollingProbeTracker(time.Hour, time.Minute)

	tracker.Record(false)
	tracker.Record(true)
	tracker.Record(false)

	total, failed := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 3, total)
	assert.Equal(t, 1, failed)
}

func TestRollingProbeTracker_Empty(t *testing.T) {
	tracker := NewRollingProbeTracker(time.Hour, time.Minute)

	total, failed := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_OutsideWindow(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(time.Hour, time.Minute)

	tracker.Record(false)

	total, failed := tracker.Stats(time.Nanosecond, now.Add(time.Hour))
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_BucketAdvancement(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(time.Hour, time.Minute)

	tracker.mu.Lock()
	tracker.recordAt(now)                      // bucket 1
	tracker.recordAt(now.Add(2 * time.Minute)) // bucket 3 (bucket 2 is empty gap)
	tracker.mu.Unlock()

	total, failed := tracker.Stats(time.Hour, now.Add(3*time.Minute))
	assert.Equal(t, 2, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_WrapAround(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(5*time.Minute, time.Minute)
	// maxBuckets = 5/1 + 1 = 6

	tracker.mu.Lock()

	for range 20 {
		tracker.recordAt(now)
		now = now.Add(time.Minute)
	}

	tracker.mu.Unlock()

	// After 20 minutes of 1-min buckets with maxBuckets=6, only 6 remain
	total, _ := tracker.Stats(time.Hour, now)
	assert.Equal(t, 6, total)
}

func TestRollingProbeTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewRollingProbeTracker(time.Hour, time.Minute)

	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			for range 100 {
				tracker.Record(true)
			}
		})
	}

	wg.Wait()

	total, failed := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 1000, total)
	assert.Equal(t, 1000, failed)
}

func TestRollingProbeTracker_AlignedWindow(t *testing.T) {
	bi := 5 * time.Second
	tracker := NewRollingProbeTracker(time.Hour, bi)

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tracker.mu.Lock()
	// Bucket [12:00:00, 12:00:05)
	tracker.recordAt(base.Add(4 * time.Second))
	// Bucket [12:00:05, 12:00:10)
	tracker.recordAt(base.Add(9 * time.Second))
	tracker.mu.Unlock()

	// Stats for last 15s from 12:00:20 → cutoff = 12:00:05 (bucket boundary)
	total, failed := tracker.Stats(15*time.Second, base.Add(20*time.Second))
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, failed)

	// Stats for last 20s from 12:00:20 → cutoff = 12:00:00 (bucket boundary)
	total, failed = tracker.Stats(20*time.Second, base.Add(20*time.Second))
	assert.Equal(t, 2, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_NonAlignedWindow(t *testing.T) {
	bi := 5 * time.Second
	tracker := NewRollingProbeTracker(time.Hour, bi)

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tracker.mu.Lock()
	// Bucket [12:00:00, 12:00:05)
	tracker.recordAt(base.Add(4 * time.Second))
	// Bucket [12:00:05, 12:00:10)
	tracker.recordAt(base.Add(6 * time.Second))
	tracker.mu.Unlock()

	// Stats for last 17s from 12:00:20 → cutoff = 12:00:03 (middle of first bucket)
	// Bucket [12:00:00, 12:00:05): timestamp=12:00:00 < 12:00:03 → excluded
	// Bucket [12:00:05, 12:00:10): timestamp=12:00:05 >= 12:00:03 → included
	// total = 1 (the probe at 12:00:06)
	total, failed := tracker.Stats(17*time.Second, base.Add(20*time.Second))
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_SubMinuteBucket(t *testing.T) {
	bi := 5 * time.Second
	tracker := NewRollingProbeTracker(time.Hour, bi)

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tracker.mu.Lock()
	// Bucket [12:00:00, 12:00:05)
	tracker.recordAt(base.Add(1 * time.Second))
	tracker.recordAt(base.Add(3 * time.Second))
	// Bucket [12:00:05, 12:00:10)
	tracker.recordAt(base.Add(7 * time.Second))
	tracker.mu.Unlock()

	// Stats for last 5s from 12:00:09 → cutoff = 12:00:04
	// Bucket [12:00:00, 12:00:05): timestamp=12:00:00 < 12:00:04 → excluded
	// Bucket [12:00:05, 12:00:10): timestamp=12:00:05 >= 12:00:04 → included
	total, failed := tracker.Stats(5*time.Second, base.Add(9*time.Second))
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, failed)

	// Stats for last 10s from 12:00:09 → cutoff = 11:59:59
	// Bucket [12:00:00, 12:00:05): timestamp=12:00:00 >= 11:59:59 → included
	// Bucket [12:00:05, 12:00:10): timestamp=12:00:05 >= 11:59:59 → included
	total, failed = tracker.Stats(10*time.Second, base.Add(9*time.Second))
	assert.Equal(t, 3, total)
	assert.Equal(t, 0, failed)
}

func TestNewRollingProbeTracker_ZeroRetention(t *testing.T) {
	tracker := NewRollingProbeTracker(0, time.Minute)
	assert.NotNil(t, tracker)

	tracker.Record(false)
	total, failed := tracker.Stats(time.Minute, time.Now())
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, failed)
}
