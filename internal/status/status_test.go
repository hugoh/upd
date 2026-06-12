package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStatus(t *testing.T) {
	a := NewStatus()
	a.SetRetention(0)
	assert.NotNil(t, a)
	assert.Nil(t, a.stateChangeTracker)
	a = NewStatus()
	a.SetRetention(1 * time.Hour)
	assert.NotNil(t, a)
	assert.NotNil(t, a.stateChangeTracker)
}

func Test_Status(t *testing.T) {
	status := NewStatus()
	status.SetRetention(0)
	assert.False(t, status.initialized)
	status.set(true)
	assert.True(t, status.initialized)
	assert.True(t, status.Up)
	status.set(false)
	assert.True(t, status.initialized)
	assert.False(t, status.Up)
	status = NewStatus()
	status.SetRetention(0)
	assert.False(t, status.initialized)
	status.set(false)
	assert.True(t, status.initialized)
	assert.False(t, status.Up)
	status.set(true)
	assert.True(t, status.initialized)
	assert.True(t, status.Up)
}

func Test_Update(t *testing.T) {
	var (
		s *Status
		c bool
	)

	s = NewStatus()
	s.SetRetention(0)
	c = s.Update(true)
	assert.True(t, c)
	c = s.Update(true)
	assert.False(t, c)
	c = s.Update(false)
	assert.True(t, c)

	s = NewStatus()
	s.SetRetention(0)
	c = s.Update(false)
	assert.True(t, c)
	c = s.Update(true)
	assert.True(t, c)
}

func TestSetDownActionStatus(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.SetDownActionStatus(DownActionStatus{
		Iteration:     7,
		SleepTime:     ReadableDuration(30 * time.Second),
		BackoffCapped: true,
	})

	rpt := s.GenStatReport([]time.Duration{time.Minute})
	require.NotNil(t, rpt.DownAction)
	assert.Equal(t, uint32(7), rpt.DownAction.Iteration)
	assert.Equal(t, ReadableDuration(30*time.Second), rpt.DownAction.SleepTime)
	assert.True(t, rpt.DownAction.BackoffCapped)
}

func TestSetDownActionStatus_OmittedWhenZero(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.SetDownActionStatus(DownActionStatus{})

	rpt := s.GenStatReport([]time.Duration{time.Minute})
	assert.Nil(t, rpt.DownAction)
}

func TestSetLoopStatus(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.SetLoopStatus(LoopStatus{
		Interval:        ReadableDuration(30 * time.Second),
		TimeSinceUpdate: ReadableDuration(10 * time.Second),
		TotalChecksRun:  42,
	})

	rpt := s.GenStatReport([]time.Duration{time.Minute})
	require.NotNil(t, rpt.Loop)
	assert.Equal(t, ReadableDuration(30*time.Second), rpt.Loop.Interval)
}

func TestSetLastSuccessAt(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	s.SetLastSuccessAt(time.Now())

	rpt := s.GenStatReport(nil)
	require.NotNil(t, rpt.Loop)
	assert.Less(t, time.Duration(rpt.Loop.LastSuccess), 2*time.Second)
}

func TestSetLastSuccessAt_NotSet(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	rpt := s.GenStatReport(nil)
	require.NotNil(t, rpt.Loop)
	assert.Equal(t, time.Duration(0), time.Duration(rpt.Loop.LastSuccess))
}

func TestSetNextCheckAt(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	s.SetNextCheckAt(time.Now().Add(30 * time.Second))

	rpt := s.GenStatReport(nil)
	require.NotNil(t, rpt.Loop)
	assert.InDelta(
		t,
		float64(30*time.Second),
		float64(time.Duration(rpt.Loop.NextCheck)),
		float64(time.Second),
	)
}

func TestSetNextCheckAt_NotSet(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	rpt := s.GenStatReport(nil)
	require.NotNil(t, rpt.Loop)
	assert.Equal(t, time.Duration(0), time.Duration(rpt.Loop.NextCheck))
}

func TestSetNextCheckAt_Overdue(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	s.SetNextCheckAt(time.Now().Add(-5 * time.Second))

	rpt := s.GenStatReport(nil)
	require.NotNil(t, rpt.Loop)
	assert.Equal(t, time.Duration(0), time.Duration(rpt.Loop.NextCheck))
}

func TestSetLastSuccessAt_ConcurrentAccess(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	done := make(chan bool, 2)

	go func() {
		s.SetLastSuccessAt(time.Now())

		done <- true
	}()

	go func() {
		rpt := s.GenStatReport(nil)
		assert.NotNil(t, rpt)

		done <- true
	}()

	<-done
	<-done
}

func TestSetNextCheckAt_ConcurrentAccess(t *testing.T) {
	s := NewStatus()
	s.SetRetention(time.Hour)
	s.Update(true)

	done := make(chan bool, 2)

	go func() {
		s.SetNextCheckAt(time.Now().Add(time.Minute))

		done <- true
	}()

	go func() {
		rpt := s.GenStatReport(nil)
		assert.NotNil(t, rpt)

		done <- true
	}()

	<-done
	<-done
}
