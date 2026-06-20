package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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
	assert.Equal(t, uint32(1), tracker.updateCount)
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

	result := empty.uptimeCalculation(true, 1*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, result.Availability, 0.01)
	assert.Equal(t, time.Duration(0), result.Downtime)

	result = empty.uptimeCalculation(false, 1*time.Minute, suite.Now)
	assert.InDelta(t, 0.0, result.Availability, 0.0001)
	assert.Equal(t, 1*time.Minute, result.Downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinOneMinute() {
	t := suite.T()
	tracker := suite.Tracker

	result := tracker.uptimeCalculation(true, 1*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, result.Availability, 0.01)
	assert.Equal(t, time.Duration(0), result.Downtime)

	result = tracker.uptimeCalculation(false, 1*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, result.Availability, 0.01)
	assert.Equal(t, time.Duration(0), result.Downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinFourteenMinutes() {
	t := suite.T()
	tracker := suite.Tracker

	result := tracker.uptimeCalculation(true, 14*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, result.Availability, 0.01)
	assert.Equal(t, time.Duration(0), result.Downtime)

	result = tracker.uptimeCalculation(false, 14*time.Minute, suite.Now)
	assert.InEpsilon(t, 1.0, result.Availability, 0.01)
	assert.Equal(t, time.Duration(0), result.Downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinSixteenMinutes() {
	t := suite.T()
	tracker := suite.Tracker

	result := tracker.uptimeCalculation(true, 16*time.Minute, suite.Now)
	assert.InEpsilon(t, 15.0/16.0, result.Availability, 0.01)
	assert.Equal(t, 1*time.Minute, result.Downtime)

	result = tracker.uptimeCalculation(false, 16*time.Minute, suite.Now)
	assert.InEpsilon(t, 15.0/16.0, result.Availability, 0.01)
	assert.Equal(t, 1*time.Minute, result.Downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinThirtyMinutes() {
	t := suite.T()
	tracker := suite.Tracker

	result := tracker.uptimeCalculation(true, 30*time.Minute, suite.Now)
	assert.InEpsilon(t, 0.5, result.Availability, 0.01)
	assert.Equal(t, 15*time.Minute, result.Downtime)

	result = tracker.uptimeCalculation(false, 30*time.Minute, suite.Now)
	assert.InEpsilon(t, 0.5, result.Availability, 0.01)
	assert.Equal(t, 15*time.Minute, result.Downtime)
}

func (suite *TestSuiteStats) TestCalc_TrackerWithinTwentyFourHours() {
	t := suite.T()
	tracker := suite.Tracker

	result := tracker.uptimeCalculation(true, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 0.75/24, result.Availability, 0.01)
	assert.Equal(t, 23*time.Hour+15*time.Minute, result.Downtime)

	result = tracker.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 0.75/24, result.Availability, 0.01)
	assert.Equal(t, 23*time.Hour+15*time.Minute, result.Downtime)
}

func (suite *TestSuiteStats) TestCalc_EmptyWithRecordChange() {
	t := suite.T()
	empty := GetTracker()
	empty.RecordChange(suite.Now.Add(-2*time.Hour), false)

	result := empty.uptimeCalculation(true, 1*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, result.Availability, 0.0001)
	assert.Equal(t, 1*time.Hour, result.Downtime)

	result = empty.uptimeCalculation(false, 1*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, result.Availability, 0.0001)
	assert.Equal(t, 1*time.Hour, result.Downtime)

	result = empty.uptimeCalculation(true, 2*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, result.Availability, 0.0001)
	assert.Equal(t, 2*time.Hour, result.Downtime)

	result = empty.uptimeCalculation(false, 2*time.Hour, suite.Now)
	assert.InDelta(t, 0.0, result.Availability, 0.0001)
	assert.Equal(t, 2*time.Hour, result.Downtime)

	result = empty.uptimeCalculation(true, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 22.0/24, result.Availability, 0.01)
	assert.Equal(t, 2*time.Hour, result.Downtime)

	result = empty.uptimeCalculation(false, 24*time.Hour, suite.Now)
	assert.InEpsilon(t, 22.0/24, result.Availability, 0.01)
	assert.Equal(t, 2*time.Hour, result.Downtime)

	empty.started = suite.Now.Add(-1 * time.Minute)
	result, err := empty.CalculateUptime(false, 1*time.Hour, suite.Now)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, result.Availability, 0.0001)
	assert.Equal(t, 1*time.Minute, result.Downtime)
	assert.Equal(t, 1*time.Minute, result.Coverage)
}

func (suite *TestSuiteStats) TestCalcError() {
	empty := GetTracker()

	_, err := empty.CalculateUptime(true, 72*time.Hour, suite.Now)
	suite.Require().Error(err)

	result, err := empty.CalculateUptime(true, 24*time.Hour, suite.Now)
	suite.Require().NoError(err)
	suite.InDelta(1.0, result.Availability, 0.0001)
	suite.Equal(time.Duration(0), result.Downtime)
}

func TestCalculateUptime_ClockSkew_ReturnsSafeZero(t *testing.T) {
	// Simulate end < started (backward clock step). Should not panic or produce
	// NaN/negative values; should return a zero UptimeResult with no error.
	tracker := &StateChangeTracker{
		retention: time.Hour,
		started:   time.Now(),
	}
	tracker.RecordChange(time.Now().Add(-time.Minute), false)

	// end is one second in the past relative to started.
	result, err := tracker.CalculateUptime(false, time.Minute, time.Now().Add(-time.Second))
	require.NoError(t, err)
	assert.Equal(t, float64(0), result.Availability)
	assert.Equal(t, time.Duration(0), result.Downtime)
	assert.Equal(t, time.Duration(0), result.Coverage)
}

func TestGenReports_ErrorPath_NotComputedDowntime(t *testing.T) {
	// A tracker with 1h retention asked for a 24h period returns "Not computed"
	// for both Availability and Downtime rather than a misleading "0s".
	tracker := &StateChangeTracker{
		retention: time.Hour,
		started:   time.Now().Add(-time.Hour),
	}

	reports := tracker.GenReports(true, time.Now(), []time.Duration{24 * time.Hour})
	require.Len(t, reports, 1)
	assert.Equal(t, ReadablePercent(-1), reports[0].Availability)
	assert.Equal(t, NotComputedDuration, reports[0].Downtime)
}
