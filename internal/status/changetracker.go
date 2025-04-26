package status

import (
	"errors"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

type StateChange struct {
	Timestamp time.Time
	Up        bool
	Prev      *StateChange
	Next      *StateChange
}

type StateChangeTracker struct {
	Head        *StateChange
	Tail        *StateChange
	Retention   time.Duration
	UpdateCount int64
	LastUpdated time.Time
	Started     time.Time
}

func (tracker *StateChangeTracker) RecordChange(timestamp time.Time, state bool) {
	tracker.UpdateCount++
	tracker.LastUpdated = timestamp

	// Ignore duplicate consecutive states
	if tracker.Tail != nil && tracker.Tail.Up == state {
		return
	}

	newChange := &StateChange{
		Timestamp: timestamp,
		Up:        state,
		Prev:      tracker.Tail,
	}

	if tracker.Tail != nil {
		tracker.Tail.Next = newChange
	}
	tracker.Tail = newChange

	if tracker.Head == nil {
		tracker.Head = newChange
	}

	tracker.Prune(timestamp)
}

func (tracker *StateChangeTracker) Prune(currentTime time.Time) {
	retentionLimit := currentTime.Add(-tracker.Retention)

	// Remove nodes at the head of the list that are outside retention
	for tracker.Head != nil && tracker.Head.Timestamp.Before(retentionLimit) {
		tracker.Head = tracker.Head.Next
		if tracker.Head != nil {
			tracker.Head.Prev = nil
		}
	}

	// If the list becomes empty, reset the Tail
	if tracker.Head == nil {
		tracker.Tail = nil
	}
}

var ErrInvalidRange = errors.New("range greater than the retention period")

func (tracker *StateChangeTracker) CalculateUptime(currentState bool,
	last time.Duration, end time.Time,
) (float64, time.Duration, error) {
	if last > tracker.Retention {
		return -1, 0, ErrInvalidRange
	}
	if end.Sub(tracker.Started) < last {
		return -1, 0, ErrInvalidRange
	}
	availability, downtime := tracker.uptimeCalculation(currentState, last, end)
	return availability, downtime, nil
}

func (tracker *StateChangeTracker) RecordsCound() int {
	i := 0
	cur := tracker.Head
	for cur != nil {
		i++
		cur = cur.Next
	}
	return i
}

func (tracker *StateChangeTracker) GenReports(currentState bool, end time.Time,
	periods []time.Duration,
) []ReportByPeriod {
	reportCount := len(periods)
	if reportCount == 0 {
		return nil
	}
	reports := make([]ReportByPeriod, reportCount)
	for i := range reportCount {
		period := periods[i]
		availability, downtime, err := tracker.CalculateUptime(currentState, period, end)
		if err != nil {
			logger.L.WithError(err).WithField("period", period).Debug("[Stats] invalid range for stat report")
		}
		reports[i] = ReportByPeriod{
			Period:       ReadableDuration(period),
			Availability: ReadablePercent(availability),
			Downtime:     ReadableDuration(downtime),
		}
	}
	return reports
}

func (tracker *StateChangeTracker) uptimeCalculation(currentState bool,
	last time.Duration, end time.Time,
) (float64, time.Duration) {
	if tracker.Tail == nil {
		// No records other than the current status
		if currentState {
			return 1.0, 0
		}
		return 0.0, last
	}

	uptime := time.Duration(0)
	start := end.Add(-last)

	current := tracker.Tail
	endOfPeriod := end
	var lastTimestampSeen time.Time
	var lastStateRecorded bool

	for current != nil {
		lastStateRecorded = current.Up
		lastTimestampSeen = current.Timestamp
		if lastTimestampSeen.Before(start) {
			lastTimestampSeen = start
		}
		// Add duration if state was 'up'
		if lastStateRecorded {
			uptime += endOfPeriod.Sub(lastTimestampSeen)
		}

		if time.Time.Equal(lastTimestampSeen, start) {
			break
		}

		endOfPeriod = lastTimestampSeen
		current = current.Prev
	}

	if lastTimestampSeen.After(start) {
		oldState := !lastStateRecorded
		if oldState {
			uptime += lastTimestampSeen.Sub(start)
		}
	}

	return (float64(uptime) / float64(last)), last - uptime
}
