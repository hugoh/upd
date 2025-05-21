package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const TestVersion = "test"

type TestSuiteStats struct {
	suite.Suite
	Now     time.Time
	Tracker *StateChangeTracker
}

func GetTracker() *StateChangeTracker {
	return &StateChangeTracker{
		Retention: 24 * time.Hour, // 1 day retention
	}
}

func (suite *TestSuiteStats) SetupTest() {
	tracker := GetTracker()
	now := time.Now()
	tracker.RecordChange(now.Add(-1*time.Hour), true)     // 1 hour ago, up
	tracker.RecordChange(now.Add(-30*time.Minute), false) // 30 minutes ago, down
	tracker.RecordChange(now.Add(-15*time.Minute), true)  // 15 minutes ago, up
	suite.Now = now
	suite.Tracker = tracker
}

func TestSuiteStatsRun(t *testing.T) {
	suite.Run(t, new(TestSuiteStats))
}

func TestEmpty(t *testing.T) {
	tracker := GetTracker()
	assert.Equal(t, 0, tracker.RecordsCount()) // Corrected: RecordsCount
}

func (suite *TestSuiteStats) TestCount() {
	t := suite.T()
	tracker := suite.Tracker
	assert.Equal(t, 3, tracker.RecordsCount()) // Corrected: RecordsCount
	tracker.RecordChange(suite.Now.Add(-25*time.Hour), true) // 25 hours
	// Pruning happens here. suite.Now is the 'currentTime'.
	// suite.Now - 24h is the retention limit.
	// The record at suite.Now.Add(-25*time.Hour) is older than this limit and should be pruned.
	// However, the original records from SetupTest are:
	// -1h, -30m, -15m. All are NEWER than suite.Now - 24h.
	// So, after adding a -25h record, it gets pruned, and the 3 original records remain.
	assert.Equal(t, 3, tracker.RecordsCount()) // Corrected: RecordsCount
}

func (suite *TestSuiteStats) TestPrune() {
	t := suite.T()
	baseTime := suite.Now // Use a consistent time from the suite, set during SetupTest

	retentionPeriod := 10 * time.Minute
	tracker := &StateChangeTracker{
		Retention: retentionPeriod,
		Started:   baseTime.Add(-1 * time.Hour), // Ensure Started is well before any records
	}

	// Timestamps for records
	// Old records (should be pruned relative to finalRecordTime)
	oldTime1 := baseTime.Add(-2 * retentionPeriod)              // e.g., suite.Now - 20 minutes
	oldTime2 := baseTime.Add(-retentionPeriod - 2*time.Minute) // e.g., suite.Now - 12 minutes

	// New records (should be kept relative to finalRecordTime)
	newTime1 := baseTime.Add(-retentionPeriod / 2) // e.g., suite.Now - 5 minutes
	newTime2 := baseTime.Add(-retentionPeriod / 3) // e.g., suite.Now - 3m20s

	// Add records. Pruning happens with each RecordChange.
	// The 'currentTime' for pruning is the timestamp of the record being added.

	tracker.RecordChange(oldTime1, true)  // currentTime = oldTime1. Cutoff = oldTime1 - 10m. No records before this.
	// List: [oldTime1(true)]

	tracker.RecordChange(oldTime2, false) // currentTime = oldTime2. Cutoff = oldTime2 - 10m.
	// oldTime1 is NOT before (oldTime2 - 10m) because -20m is not before (-12m - 10m = -22m).
	// List: [oldTime1(true), oldTime2(false)]

	tracker.RecordChange(newTime1, true)  // currentTime = newTime1. Cutoff = newTime1 - 10m.
	// oldTime1 (-20m) IS before (newTime1 - 10m = -5m - 10m = -15m). oldTime1 is pruned.
	// oldTime2 (-12m) is NOT before -15m.
	// List: [oldTime2(false), newTime1(true)]

	tracker.RecordChange(newTime2, false) // currentTime = newTime2. Cutoff = newTime2 - 10m.
	// oldTime2 (-12m) IS before (newTime2 - 10m = -3m20s - 10m = -13m20s). oldTime2 is pruned.
	// newTime1 (-5m) is NOT before -13m20s.
	// List: [newTime1(true), newTime2(false)]

	// Final record to set the "current time" for pruning accurately for the test assertion point.
	finalRecordTime := baseTime // suite.Now
	tracker.RecordChange(finalRecordTime, true) // currentTime = finalRecordTime. Cutoff = finalRecordTime - 10m.
	// newTime1 (-5m from finalRecordTime) is NOT before (finalRecordTime - 10m). Kept.
	// newTime2 (-3m20s from finalRecordTime) is NOT before (finalRecordTime - 10m). Kept.
	// List: [newTime1(true), newTime2(false), finalRecordTime(true)]

	// Expected number of records: newTime1, newTime2, and finalRecordTime record.
	assert.Equal(t, 3, tracker.RecordsCount(), "Should only have 3 records after pruning")

	// Check the content of the remaining records
	var remainingTimestamps []time.Time
	current := tracker.Head
	for current != nil {
		remainingTimestamps = append(remainingTimestamps, current.Timestamp)
		current = current.Next
	}

	// Assert that old records' original timestamps are not present
	assert.NotContains(t, remainingTimestamps, oldTime1, "Timestamp of old record 1 should be pruned")
	assert.NotContains(t, remainingTimestamps, oldTime2, "Timestamp of old record 2 should be pruned")

	// Assert that new records' original timestamps are present
	assert.Contains(t, remainingTimestamps, newTime1, "Timestamp of new record 1 should be present")
	assert.Contains(t, remainingTimestamps, newTime2, "Timestamp of new record 2 should be present")
	assert.Contains(t, remainingTimestamps, finalRecordTime, "Timestamp of final record should be present")

	// Assert that all remaining records are within the retention period relative to finalRecordTime
	pruningCutoff := finalRecordTime.Add(-retentionPeriod)
	for _, ts := range remainingTimestamps {
		assert.False(t, ts.Before(pruningCutoff), "Timestamp %v should not be before cutoff %v (finalRecordTime: %v)", ts, pruningCutoff, finalRecordTime)
		// Also check they are not in the future relative to finalRecordTime, which would be illogical.
		assert.False(t, ts.After(finalRecordTime), "Timestamp %v should not be after the final record time %v", ts, finalRecordTime)
	}
}

func (suite *TestSuiteStats) TestCalc() {
	t := suite.T()
	empty := GetTracker()

	// Updated test cases for the new return values.
	actual, downtime := empty.uptimeCalculation(true, 1*time.Minute, suite.Now)
	assert.Equal(t, 1.0, actual)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = empty.uptimeCalculation(false, 1*time.Minute, suite.Now)
	assert.Equal(t, 0.0, actual)
	assert.Equal(t, 1*time.Minute, downtime)

	tracker := suite.Tracker

	actual, downtime = tracker.uptimeCalculation(true, 1*time.Minute, suite.Now)
	assert.Equal(t, 1.0, actual)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = tracker.uptimeCalculation(false, 1*time.Minute, suite.Now)
	assert.Equal(t, 1.0, actual)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = tracker.uptimeCalculation(true, 14*time.Minute, suite.Now)
	assert.Equal(t, 1.0, actual)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = tracker.uptimeCalculation(false, 14*time.Minute, suite.Now)
	assert.Equal(t, 1.0, actual)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = tracker.uptimeCalculation(true, 16*time.Minute, suite.Now)
	assert.Equal(t, 15.0/16.0, actual)
	assert.Equal(t, 1*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 16*time.Minute, suite.Now)
	assert.Equal(t, 15.0/16.0, actual)
	assert.Equal(t, 1*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(true, 30*time.Minute, suite.Now)
	assert.Equal(t, 0.5, actual)
	assert.Equal(t, 15*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 30*time.Minute, suite.Now)
	assert.Equal(t, 0.5, actual)
	assert.Equal(t, 15*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(true, 24*time.Hour, suite.Now)
	assert.Equal(t, 0.75/24, actual) // This seems like an existing potentially odd test calculation, keeping as is.
	assert.Equal(t, 23*time.Hour+15*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.Equal(t, 0.75/24, actual) // This seems like an existing potentially odd test calculation, keeping as is.
	assert.Equal(t, 23*time.Hour+15*time.Minute, downtime)

	empty.RecordChange(suite.Now.Add(-2*time.Hour), false)

	actual, downtime = empty.uptimeCalculation(true, 1*time.Hour, suite.Now)
	assert.Equal(t, 0.0, actual)
	assert.Equal(t, 1*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(false, 1*time.Hour, suite.Now)
	assert.Equal(t, 0.0, actual)
	assert.Equal(t, 1*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(true, 2*time.Hour, suite.Now)
	assert.Equal(t, 0.0, actual)
	assert.Equal(t, 2*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(false, 2*time.Hour, suite.Now)
	assert.Equal(t, 0.0, actual)
	assert.Equal(t, 2*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(true, 24*time.Hour, suite.Now)
	assert.Equal(t, 22.0/24, actual)
	assert.Equal(t, 2*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.Equal(t, 22.0/24, actual)
	assert.Equal(t, 2*time.Hour, downtime)

	empty.Started = suite.Now.Add(-1 * time.Minute)
	v, w, err := empty.CalculateUptime(false, 1*time.Hour, suite.Now)
	assert.Equal(t, -1.0, v)
	assert.Equal(t, time.Duration(0), w)
	assert.Error(t, err)
}

func (suite *TestSuiteStats) TestCalcError() {
	t := suite.T()
	empty := GetTracker()
	var err error
	_, _, err = empty.CalculateUptime(true, 72*time.Hour, suite.Now)
	assert.Error(t, err)
	_, _, err = empty.CalculateUptime(true, 24*time.Hour, suite.Now)
	assert.NoError(t, err)
}
