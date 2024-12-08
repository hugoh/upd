package internal

import (
	"errors"
	"time"
)

type Status struct {
	Up                 bool
	Initialized        bool
	StateChangeTracker *StateChangeTracker
}

type StateChange struct {
	Timestamp time.Time
	Up        bool
	Prev      *StateChange
	Next      *StateChange
}

type StateChangeTracker struct {
	Head      *StateChange
	Tail      *StateChange
	Retention time.Duration
	Started   time.Time
}

func NewStatus(statsRetention time.Duration) *Status {
	var stateChangeTracker *StateChangeTracker
	if statsRetention > 0 {
		stateChangeTracker = &StateChangeTracker{
			Retention: statsRetention,
			Started:   time.Now(),
		}
	}
	return &Status{
		StateChangeTracker: stateChangeTracker,
	}
}

func (s *Status) Set(up bool) {
	if !s.Initialized {
		s.Initialized = true
	}
	s.Up = up
}

func (s *Status) HasChanged(newStatus bool) bool {
	return !s.Initialized || newStatus != s.Up
}

func (s *Status) RecordResult(up bool) {
	if s.StateChangeTracker == nil {
		return
	}
	s.StateChangeTracker.RecordChange(time.Now(), up)
}

func (tracker *StateChangeTracker) RecordChange(timestamp time.Time, state bool) {
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

func (tracker *StateChangeTracker) uptimeCalculation(currentState bool,
	last time.Duration, end time.Time,
) float64 {
	if tracker.Tail == nil {
		// No records other than the current status
		if currentState {
			return 1.0
		}
		return 0.0
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

		if lastTimestampSeen == start {
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

	return (float64(uptime) / float64(last))
}

var ErrInvalidRange = errors.New("range greater than the retention period")

func (tracker *StateChangeTracker) CalculateUptime(currentState bool,
	last time.Duration, end time.Time,
) (float64, error) {
	if last > tracker.Retention {
		return -1, ErrInvalidRange
	}
	if end.Sub(tracker.Started) < last {
		return -1, ErrInvalidRange
	}
	return tracker.uptimeCalculation(currentState, last, end), nil
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
