package status

import (
	"errors"
	"fmt"
	"time"
)

// ErrPeriodExceedsRetention is returned when the requested period is longer
// than the configured retention window.
var ErrPeriodExceedsRetention = errors.New("period exceeds retention")

// UptimeResult holds the result of an uptime calculation.
type UptimeResult struct {
	Availability float64
	Downtime     time.Duration
	Coverage     time.Duration
}

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
	updateCount uint32
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

// CalculateUptime computes availability, downtime, and coverage for a given
// period. Coverage may be less than last when the tracker started recently.
// Returns an error if last exceeds the configured retention (data was pruned
// and cannot be recovered).
func (tracker *StateChangeTracker) CalculateUptime(currentState bool,
	last time.Duration, end time.Time,
) (UptimeResult, error) {
	if last > tracker.retention {
		return UptimeResult{}, fmt.Errorf(
			"%w: period %v exceeds retention %v",
			ErrPeriodExceedsRetention,
			last,
			tracker.retention,
		)
	}

	coverage := min(end.Sub(tracker.started), last)
	result := tracker.uptimeCalculation(currentState, coverage, end)
	result.Coverage = coverage

	return result, nil
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

		result, err := tracker.CalculateUptime(currentState, period, end)
		if err != nil {
			reports[idx] = ReportByPeriod{
				Period:       ReadableDuration(period),
				Availability: ReadablePercent(-1),
			}

			continue
		}

		rpt := ReportByPeriod{
			Period:       ReadableDuration(period),
			Availability: ReadablePercent(result.Availability),
			Downtime:     ReadableDuration(result.Downtime),
		}

		if result.Coverage < period {
			c := ReadableDuration(result.Coverage)
			rpt.Coverage = &c
		}

		reports[idx] = rpt
	}

	return reports
}

func (tracker *StateChangeTracker) uptimeCalculation(currentState bool,
	last time.Duration, end time.Time,
) UptimeResult {
	if tracker.tail == nil {
		// No records other than the current status
		if currentState {
			return UptimeResult{Availability: 1.0}
		}

		return UptimeResult{Downtime: last}
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

		if lastTimestampSeen.Equal(start) {
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

	return UptimeResult{
		Availability: float64(uptime) / float64(last),
		Downtime:     last - uptime,
	}
}
