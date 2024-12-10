package internal

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
	assert.Equal(t, 0, tracker.RecordsCound())
}

func (suite *TestSuiteStats) TestCount() {
	t := suite.T()
	tracker := suite.Tracker
	assert.Equal(t, 3, tracker.RecordsCound())
	tracker.RecordChange(suite.Now.Add(-25*time.Hour), true) // 25 hours
	assert.Equal(t, 3, tracker.RecordsCound())
}

func (suite *TestSuiteStats) TestPrune() {
	t := suite.T()
	tracker := suite.Tracker
	assert.Equal(t, 3, tracker.RecordsCound())
}

func (suite *TestSuiteStats) TestCalc() {
	t := suite.T()
	empty := GetTracker()
	assert.Equal(t, 1.0,
		empty.uptimeCalculation(true,
			1*time.Minute,
			suite.Now))
	assert.Equal(t, 0.0,
		empty.uptimeCalculation(false,
			1*time.Minute,
			suite.Now))
	tracker := suite.Tracker
	assert.Equal(t, 1.0,
		tracker.uptimeCalculation(true,
			1*time.Minute,
			suite.Now))
	assert.Equal(t, 1.0,
		tracker.uptimeCalculation(false,
			1*time.Minute,
			suite.Now))
	assert.Equal(t, 1.0,
		tracker.uptimeCalculation(true,
			14*time.Minute,
			suite.Now))
	assert.Equal(t, 1.0,
		tracker.uptimeCalculation(false,
			14*time.Minute,
			suite.Now))
	assert.Equal(t, 15.0/16.0,
		tracker.uptimeCalculation(true,
			16*time.Minute,
			suite.Now))
	assert.Equal(t, 15.0/16.0,
		tracker.uptimeCalculation(false,
			16*time.Minute,
			suite.Now))
	assert.Equal(t, 0.5,
		tracker.uptimeCalculation(true,
			30*time.Minute,
			suite.Now))
	assert.Equal(t, 0.5,
		tracker.uptimeCalculation(false,
			30*time.Minute,
			suite.Now))
	assert.Equal(t, 0.75/24,
		tracker.uptimeCalculation(true,
			24*time.Hour,
			suite.Now))
	assert.Equal(t, 0.75/24,
		tracker.uptimeCalculation(false,
			24*time.Hour,
			suite.Now))
	empty.RecordChange(suite.Now.Add(-2*time.Hour), false)
	assert.Equal(t, 0.0,
		empty.uptimeCalculation(true,
			1*time.Hour,
			suite.Now))
	assert.Equal(t, 0.0,
		empty.uptimeCalculation(false,
			1*time.Hour,
			suite.Now))
	assert.Equal(t, 0.0,
		empty.uptimeCalculation(true,
			2*time.Hour,
			suite.Now))
	assert.Equal(t, 0.0,
		empty.uptimeCalculation(false,
			2*time.Hour,
			suite.Now))
	assert.Equal(t, 22.0/24,
		empty.uptimeCalculation(true,
			24*time.Hour,
			suite.Now))
	assert.Equal(t, 22.0/24,
		empty.uptimeCalculation(false,
			24*time.Hour,
			suite.Now))
	empty.Started = suite.Now.Add(-1 * time.Minute)
	v, err := empty.CalculateUptime(false, 1*time.Hour, suite.Now)
	assert.Equal(t, -1.0, v)
	assert.Error(t, err)
}

func (suite *TestSuiteStats) TestCalcError() {
	t := suite.T()
	empty := GetTracker()
	var err error
	_, err = empty.CalculateUptime(true, 72*time.Hour, suite.Now)
	assert.Error(t, err)
	_, err = empty.CalculateUptime(true, 24*time.Hour, suite.Now)
	assert.NoError(t, err)
}
