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

func TestRollingProbeTracker_Record(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(time.Hour)

	for range 3 {
		tracker.Record(false)
	}

	total, failed := tracker.Stats(time.Hour, now.Add(time.Minute))
	assert.Equal(t, 3, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_RecordFailed(t *testing.T) {
	tracker := NewRollingProbeTracker(time.Hour)

	tracker.Record(false)
	tracker.Record(true)
	tracker.Record(false)

	total, failed := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 3, total)
	assert.Equal(t, 1, failed)
}

func TestRollingProbeTracker_Empty(t *testing.T) {
	tracker := NewRollingProbeTracker(time.Hour)

	total, failed := tracker.Stats(time.Hour, time.Now())
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_OutsideWindow(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(time.Hour)

	tracker.Record(false)

	total, failed := tracker.Stats(time.Nanosecond, now.Add(time.Hour))
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, failed)
}

func TestRollingProbeTracker_BucketAdvancement(t *testing.T) {
	now := startOfThisMinute()
	tracker := NewRollingProbeTracker(time.Hour)

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
	tracker := NewRollingProbeTracker(5 * time.Minute)
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
	tracker := NewRollingProbeTracker(time.Hour)

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

func TestNewRollingProbeTracker_ZeroRetention(t *testing.T) {
	tracker := NewRollingProbeTracker(0)
	assert.NotNil(t, tracker)

	tracker.Record(false)
	total, failed := tracker.Stats(time.Minute, time.Now())
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, failed)
}
