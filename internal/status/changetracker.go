package status

import (
	"errors"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

const (
	// InvalidRangeMsg is the error message returned when the requested range exceeds retention.
	InvalidRangeMsg = "range greater than the retention period"
)

// StateChange represents a single state transition in the tracker.
type StateChange struct {
	timestamp time.Time
	up        bool
	prev      *StateChange
	next      *StateChange
}

// StateChangeTracker manages a doubly-linked list of state changes for uptime calculations.
type StateChangeTracker struct {
	head        *StateChange
	tail        *StateChange
	retention   time.Duration
	updateCount int64
	lastUpdated time.Time
	started     time.Time
}

// RecordChange adds a new state change to the tracker and prunes old entries.
func (tracker *StateChangeTracker) RecordChange(timestamp time.Time, state bool) {
	tracker.updateCount++
	tracker.lastUpdated = timestamp

	// Ignore duplicate consecutive states
	if tracker.tail != nil && tracker.tail.up == state {
		return
	}

	newChange := &StateChange{
		timestamp: timestamp,
		up:        state,
		prev:      tracker.tail,
	}

	if tracker.tail != nil {
		tracker.tail.next = newChange
	}

	tracker.tail = newChange

	if tracker.head == nil {
		tracker.head = newChange
	}

	tracker.Prune(timestamp)
}

// Prune removes state changes older than the retention period.
func (tracker *StateChangeTracker) Prune(currentTime time.Time) {
	retentionLimit := currentTime.Add(-tracker.retention)

	// Remove nodes at the head of the list that are outside retention
	for tracker.head != nil && tracker.head.timestamp.Before(retentionLimit) {
		tracker.head = tracker.head.next
		if tracker.head != nil {
			tracker.head.prev = nil
		}
	}

	// If the list becomes empty, reset the Tail
	if tracker.head == nil {
		tracker.tail = nil
	}
}

// ErrInvalidRange is returned when the requested duration exceeds retention.
var ErrInvalidRange = errors.New(InvalidRangeMsg)

// CalculateUptime computes availability percentage and downtime for a given period.
func (tracker *StateChangeTracker) CalculateUptime(currentState bool,
	last time.Duration, end time.Time,
) (float64, time.Duration, error) {
	if last > tracker.retention {
		return -1, 0, ErrInvalidRange
	}

	if end.Sub(tracker.started) < last {
		return -1, 0, ErrInvalidRange
	}

	availability, downtime := tracker.uptimeCalculation(currentState, last, end)

	return availability, downtime, nil
}

// RecordsCount returns the number of state changes in the tracker.
// This method is primarily used for testing and debugging.
// The count includes only records that are within the retention period.
func (tracker *StateChangeTracker) RecordsCount() int {
	recordsNumber := 0

	cur := tracker.head
	for cur != nil {
		recordsNumber++
		cur = cur.next
	}

	return recordsNumber
}

// GenReports generates uptime reports for multiple time periods.
func (tracker *StateChangeTracker) GenReports(currentState bool, end time.Time,
	periods []time.Duration,
) []ReportByPeriod {
	reportCount := len(periods)
	if reportCount == 0 {
		return nil
	}

	reports := make([]ReportByPeriod, reportCount)

	for idx := range periods {
		period := periods[idx]

		availability, downtime, err := tracker.CalculateUptime(currentState, period, end)
		if err != nil {
			logger.L.Debug("[Stats] invalid range for stat report", "error", err, "period", period)
		}

		reports[idx] = ReportByPeriod{
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
	if tracker.tail == nil {
		// No records other than the current status
		if currentState {
			return 1.0, 0
		}

		return 0.0, last
	}

	uptime := time.Duration(0)
	start := end.Add(-last)

	current := tracker.tail
	endOfPeriod := end

	var (
		lastTimestampSeen time.Time
		lastStateRecorded bool
	)

	for current != nil {
		lastStateRecorded = current.up

		lastTimestampSeen = current.timestamp
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
		current = current.prev
	}

	if lastTimestampSeen.After(start) {
		oldState := !lastStateRecorded
		if oldState {
			uptime += lastTimestampSeen.Sub(start)
		}
	}

	return (float64(uptime) / float64(last)), last - uptime
}
