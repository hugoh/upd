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
	assert.Equal(t, 0, tracker.RecordsCount())
}

func TestRecordChange_UpdatesCurrentTimeFields(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.RecordChange(now, true)
	assert.Equal(t, now, tracker.LastUpdated)
	assert.Equal(t, int64(1), tracker.UpdateCount)
}

func TestRecordChange_HeadAndTailSetCorrectly(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.RecordChange(now, true)
	assert.NotNil(t, tracker.Head)
	assert.NotNil(t, tracker.Tail)
	assert.Equal(t, tracker.Head, tracker.Tail)
	assert.Equal(t, now, tracker.Head.Timestamp)
	assert.True(t, tracker.Head.Up)
}

func TestRecordChange_IgnoresDuplicateConsecutiveStates(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.RecordChange(now, true)
	tracker.RecordChange(now.Add(1*time.Minute), true)
	assert.Equal(t, 1, tracker.RecordsCount())
}

func TestRecordChange_AddsNewStateChange(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.RecordChange(now, true)
	tracker.RecordChange(now.Add(1*time.Minute), false)
	assert.Equal(t, 2, tracker.RecordsCount())
	assert.False(t, tracker.Tail.Up)
	assert.True(t, tracker.Head.Up)
}

func TestPrune_RemovesOldRecords(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.Retention = 10 * time.Minute
	tracker.RecordChange(now.Add(-20*time.Minute), true)
	tracker.RecordChange(now.Add(-5*time.Minute), false)
	tracker.Prune(now)
	assert.Equal(t, 1, tracker.RecordsCount())
	assert.Equal(t, now.Add(-5*time.Minute), tracker.Head.Timestamp)
}

func TestPrune_EmptiesListIfAllOld(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.Retention = 1 * time.Minute
	tracker.RecordChange(now.Add(-10*time.Minute), true)
	tracker.Prune(now)
	assert.Nil(t, tracker.Head)
	assert.Nil(t, tracker.Tail)
	assert.Equal(t, 0, tracker.RecordsCount())
}

func TestPrune_DoesNotRemoveRecentRecords(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.Retention = 1 * time.Hour
	tracker.RecordChange(now.Add(-30*time.Minute), true)
	tracker.RecordChange(now.Add(-10*time.Minute), false)
	tracker.Prune(now)
	assert.Equal(t, 2, tracker.RecordsCount())
}

func (suite *TestSuiteStats) TestCount() {
	t := suite.T()
	tracker := suite.Tracker
	assert.Equal(t, 3, tracker.RecordsCount())
	tracker.RecordChange(suite.Now.Add(-25*time.Hour), true) // 25 hours
	assert.Equal(t, 3, tracker.RecordsCount())
}

func (suite *TestSuiteStats) TestPrune() {
	t := suite.T()
	tracker := suite.Tracker
	assert.Equal(t, 3, tracker.RecordsCount())
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
	assert.Equal(t, 0.75/24, actual)
	assert.Equal(t, 23*time.Hour+15*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.Equal(t, 0.75/24, actual)
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
