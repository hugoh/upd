// Package status provides network connectivity status tracking and statistics.
//
// Status Tracking:
//
// The Status struct tracks whether the network connection is up or down,
// providing thread-safe access to the current state and state change history.
//
// Example - Tracking connection status:
//
//	status := status.NewStatus()
//	status.SetRetention(24 * time.Hour)
//
//	// After running checks:
//	changed := status.Update(true)  // Connection came up
//	if changed {
//		fmt.Println("Connection status changed!")
//	}
//
//	// Get current status:
//	up := status.Up
//
// Thread Safety:
//
// The Status struct is fully thread-safe and can be accessed concurrently
// from multiple goroutines. All methods use internal mutex locking to
// ensure safe concurrent access.
//
// Retention and History:
//
// When retention is set, the Status tracks state changes over time,
// enabling statistics like:
//   - Uptime percentage over various time periods
//   - Downtime duration
//   - State change timestamps
//
// Example - Getting statistics:
//
//	status := status.NewStatus()
//	status.SetRetention(7 * 24 * time.Hour)  // Keep 7 days of history
//
//	status.Update(true)
//	time.Sleep(1 * time.Hour)
//	status.Update(false)
//	time.Sleep(30 * time.Minute)
//	status.Update(true)
//
//	reports := status.GetReports(true, time.Now(),
//		[]time.Duration{
//			1 * time.Hour,
//			24 * time.Hour,
//			7 * 24 * time.Hour,
//		})
//
//	for _, report := range reports {
//		fmt.Printf("Period: %s, Uptime: %s, Downtime: %s\n",
//			report.Period, report.Availability, report.Downtime)
//	}
//
// State Change Tracking:
//
// The Status internally uses StateChangeTracker to record all state
// changes, pruning old records based on the retention period. This
// allows accurate calculation of uptime/downtime statistics over
// various time windows.
//
// Example - State change history:
//
//	status := status.NewStatus()
//	status.SetRetention(1 * time.Hour)
//
//	status.Update(true)   // Initial state: up
//	time.Sleep(10 * time.Minute)
//	status.Update(false)  // Connection went down
//	time.Sleep(5 * time.Minute)
//	status.Update(true)   // Connection came back
//
//	// Get statistics for last hour
//	reports := status.GetReports(true, time.Now(),
//		[]time.Duration{1 * time.Hour})
//
// // The reports will show accurate uptime based on the state transitions
package status

import (
	"sync"
	"time"

	"github.com/hugoh/upd/internal/version"
)

// Status tracks the current network connectivity state and history.
type Status struct {
	Up                 bool
	initialized        bool
	mutex              sync.Mutex
	stateChangeTracker *StateChangeTracker
	rollingTracker     *RollingProbeTracker
	downActionStatus   DownActionStatus
	loopStatus         LoopStatus
	lastSuccessAt      time.Time
	nextCheckAt        time.Time
}

// NewStatus creates a new Status instance.
func NewStatus() *Status {
	return &Status{}
}

// SetRetention configures the retention period for state change history.
func (s *Status) SetRetention(retention time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if retention <= 0 {
		s.stateChangeTracker = nil

		return
	}

	if s.stateChangeTracker == nil {
		s.stateChangeTracker = &StateChangeTracker{
			retention: retention,
			started:   time.Now(),
		}
	} else {
		s.stateChangeTracker.retention = retention
		s.stateChangeTracker.Prune(time.Now())
	}
}

// SetRollingTracker attaches a probe stats tracker for per-period reporting.
func (s *Status) SetRollingTracker(t *RollingProbeTracker) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.rollingTracker = t
}

// SetDownActionStatus stores a snapshot of the down action loop state.
func (s *Status) SetDownActionStatus(das DownActionStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.downActionStatus = das
}

// SetLoopStatus stores a snapshot of the monitoring loop state.
func (s *Status) SetLoopStatus(ls LoopStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.loopStatus = ls
}

// SetLastSuccessAt stores the timestamp of the last successful check.
func (s *Status) SetLastSuccessAt(t time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lastSuccessAt = t
}

// SetNextCheckAt stores the timestamp of the next scheduled check.
func (s *Status) SetNextCheckAt(t time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.nextCheckAt = t
}

// Update updates the connection status and records the state change if necessary.
//
// Returns true if the status changed (i.e., went from up to down or vice versa),
// false if the status remained the same.
//
// This method is thread-safe and can be called from multiple goroutines.
//
// Example:
//
//	status := status.NewStatus()
//	changed := status.Update(true)  // Set status to up
//	if changed {
//		fmt.Println("Status changed to up")
//	}
//
//	changed = status.Update(false)  // Set status to down
//	if changed {
//		fmt.Println("Status changed to down")
//	}
func (s *Status) Update(isUp bool) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.recordResult(isUp)

	if !s.hasChanged(isUp) {
		return false
	}

	s.set(isUp)

	return true
}

// GenStatReport generates a statistics report for the specified time periods.
func (s *Status) GenStatReport(periods []time.Duration) *Report {
	generated := time.Now()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	das := s.downActionStatus
	loopSt := s.loopStatus

	rpt := &Report{
		Generated:  generated,
		Up:         s.Up,
		Version:    version.Version(),
		Loop:       &loopSt,
		DownAction: nil,
	}

	if das.Iteration > 0 || das.SleepTime > 0 {
		rpt.DownAction = &das
	}

	if !s.lastSuccessAt.IsZero() {
		rpt.Loop.LastSuccess = ReadableDuration(generated.Sub(s.lastSuccessAt))
	}

	if !s.nextCheckAt.IsZero() {
		remaining := max(s.nextCheckAt.Sub(generated), 0)

		rpt.Loop.NextCheck = ReadableDuration(remaining)
	}

	if s.stateChangeTracker == nil {
		return rpt
	}

	rpt.Uptime = ReadableDuration(generated.Sub(s.stateChangeTracker.started))
	rpt.Stats = s.stateChangeTracker.GenReports(s.Up, generated, periods)
	rpt.Loop.TimeSinceUpdate = ReadableDuration(generated.Sub(s.stateChangeTracker.lastUpdated))
	rpt.Loop.TotalChecksRun = s.stateChangeTracker.updateCount

	if s.rollingTracker != nil {
		for idx, period := range periods {
			ps := s.rollingTracker.Stats(period, generated)
			rpt.Stats[idx].TotalProbes = ps.Total

			rpt.Stats[idx].FailedProbes = ps.Failed
			if ps.Total > 0 {
				rpt.Stats[idx].FailureRate = ReadablePercent(float64(ps.Failed) / float64(ps.Total))
			}
		}
	}

	return rpt
}

func (s *Status) set(up bool) {
	if !s.initialized {
		s.initialized = true
	}

	s.Up = up
}

func (s *Status) hasChanged(newStatus bool) bool {
	return !s.initialized || newStatus != s.Up
}

func (s *Status) recordResult(up bool) {
	if s.stateChangeTracker == nil {
		return
	}

	s.stateChangeTracker.RecordChange(time.Now(), up)
}
