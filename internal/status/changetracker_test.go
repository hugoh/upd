package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		retention: 24 * time.Hour, // 1 day retention
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
	assert.Equal(t, now, tracker.lastUpdated)
	assert.Equal(t, int64(1), tracker.updateCount)
}

func TestRecordChange_HeadAndTailSetCorrectly(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.RecordChange(now, true)
	assert.NotNil(t, tracker.head)
	assert.NotNil(t, tracker.tail)
	assert.Equal(t, tracker.head, tracker.tail)
	assert.Equal(t, now, tracker.head.timestamp)
	assert.True(t, tracker.head.up)
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
	assert.False(t, tracker.tail.up)
	assert.True(t, tracker.head.up)
}

func TestPrune_RemovesOldRecords(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.retention = 10 * time.Minute
	tracker.RecordChange(now.Add(-20*time.Minute), true)
	tracker.RecordChange(now.Add(-5*time.Minute), false)
	tracker.Prune(now)
	assert.Equal(t, 1, tracker.RecordsCount())
	assert.Equal(t, now.Add(-5*time.Minute), tracker.head.timestamp)
}

func TestPrune_EmptiesListIfAllOld(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.retention = 1 * time.Minute
	tracker.RecordChange(now.Add(-10*time.Minute), true)
	tracker.Prune(now)
	assert.Nil(t, tracker.head)
	assert.Nil(t, tracker.tail)
	assert.Equal(t, 0, tracker.RecordsCount())
}

func TestPrune_DoesNotRemoveRecentRecords(t *testing.T) {
	tracker := GetTracker()
	now := time.Now()
	tracker.retention = 1 * time.Hour
	tracker.RecordChange(now.Add(-30*time.Minute), true)
	tracker.RecordChange(now.Add(-10*time.Minute), false)
	tracker.Prune(now)
	assert.Equal(t, 2, tracker.RecordsCount())
}

func (suite *TestSuiteStats) TestCount() {
	tracker := suite.Tracker
	suite.Equal(3, tracker.RecordsCount())
	tracker.RecordChange(suite.Now.Add(-25*time.Hour), true) // 25 hours
	suite.Equal(3, tracker.RecordsCount())
}

func (suite *TestSuiteStats) TestPrune() {
	tracker := suite.Tracker
	suite.Equal(3, tracker.RecordsCount())
}

func (suite *TestSuiteStats) TestCalc_EmptyTracker() {
	t := suite.T()
	empty := GetTracker()

	actual, downtime := empty.uptimeCalculation(true, 1*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, actual, 0.01)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = empty.uptimeCalculation(false, 1*time.Minute, suite.Now)
	assert.InDelta(t, 0.0, actual, 0.0001)
	assert.Equal(t, 1*time.Minute, downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinOneMinute() {
	t := suite.T()
	tracker := suite.Tracker

	actual, downtime := tracker.uptimeCalculation(true, 1*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, actual, 0.01)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = tracker.uptimeCalculation(false, 1*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, actual, 0.01)
	assert.Equal(t, time.Duration(0), downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinFourteenMinutes() {
	t := suite.T()
	tracker := suite.Tracker

	actual, downtime := tracker.uptimeCalculation(true, 14*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, actual, 0.01)
	assert.Equal(t, time.Duration(0), downtime)

	actual, downtime = tracker.uptimeCalculation(false, 14*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, actual, 0.01)
	assert.Equal(t, time.Duration(0), downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinSixteenMinutes() {
	t := suite.T()
	tracker := suite.Tracker

	actual, downtime := tracker.uptimeCalculation(true, 16*time.Minute, suite.Now)
	assert.InEpsilon(t, 15.0/16.0, actual, 0.01)
	assert.Equal(t, 1*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 16*time.Minute, suite.Now)
	assert.InEpsilon(t, 15.0/16.0, actual, 0.01)
	assert.Equal(t, 1*time.Minute, downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinThirtyMinutes() {
	t := suite.T()
	tracker := suite.Tracker

	actual, downtime := tracker.uptimeCalculation(true, 30*time.Minute, suite.Now)
	assert.InEpsilon(t, 0.5, actual, 0.01)
	assert.Equal(t, 15*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 30*time.Minute, suite.Now)
	assert.InEpsilon(t, 0.5, actual, 0.01)
	assert.Equal(t, 15*time.Minute, downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinTwentyFourHours() {
	t := suite.T()
	tracker := suite.Tracker

	actual, downtime := tracker.uptimeCalculation(true, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 0.75/24, actual, 0.01)
	assert.Equal(t, 23*time.Hour+15*time.Minute, downtime)

	actual, downtime = tracker.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 0.75/24, actual, 0.01)
	assert.Equal(t, 23*time.Hour+15*time.Minute, downtime)
}

func (suite *TestSuiteStats) TestCalc_EmptyWithRecordChange() {
	t := suite.T()
	empty := GetTracker()
	empty.RecordChange(suite.Now.Add(-2*time.Hour), false)

	actual, downtime := empty.uptimeCalculation(true, 1*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, actual, 0.0001)
	assert.Equal(t, 1*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(false, 1*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, actual, 0.0001)
	assert.Equal(t, 1*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(true, 2*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, actual, 0.0001)
	assert.Equal(t, 2*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(false, 2*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, actual, 0.0001)
	assert.Equal(t, 2*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(true, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 22.0/24, actual, 0.01)
	assert.Equal(t, 2*time.Hour, downtime)

	actual, downtime = empty.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 22.0/24, actual, 0.01)
	assert.Equal(t, 2*time.Hour, downtime)

	empty.started = suite.Now.Add(-1 * time.Minute)
	v, w, err := empty.CalculateUptime(false, 1*time.Hour, suite.Now)
	assert.InDelta(t, -1.0, v, 0.0001)
	assert.Equal(t, time.Duration(0), w)
	require.Error(t, err)
}

func (suite *TestSuiteStats) TestCalcError() {
	empty := GetTracker()

	var err error

	_, _, err = empty.CalculateUptime(true, 72*time.Hour, suite.Now)
	suite.Require().Error(err)
	_, _, err = empty.CalculateUptime(true, 24*time.Hour, suite.Now)
	suite.NoError(err)
}
