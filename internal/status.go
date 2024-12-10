package internal

import (
	"sync"
	"time"
)

type Status struct {
	Up                 bool
	Initialized        bool
	Version            string
	StateChangeTracker *StateChangeTracker
	mutex              sync.Mutex
}

func NewStatus(version string, statsRetention time.Duration) *Status {
	var stateChangeTracker *StateChangeTracker
	if statsRetention > 0 {
		stateChangeTracker = &StateChangeTracker{
			Retention: statsRetention,
			Started:   time.Now(),
		}
	}
	return &Status{
		Version:            version,
		StateChangeTracker: stateChangeTracker,
	}
}

func (s *Status) set(up bool) {
	if !s.Initialized {
		s.Initialized = true
	}
	s.Up = up
}

func (s *Status) hasChanged(newStatus bool) bool {
	return !s.Initialized || newStatus != s.Up
}

func (s *Status) recordResult(up bool) {
	if s.StateChangeTracker == nil {
		return
	}
	s.StateChangeTracker.RecordChange(time.Now(), up)
}

// Returns true if it changed
func (s *Status) Update(up bool) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.recordResult(up)
	if !s.hasChanged(up) {
		return false
	}
	s.set(up)
	return true
}

func (s *Status) GenStatReport(periods []time.Duration) *StatusReport {
	generated := time.Now()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return &StatusReport{
		Generated:  generated,
		Uptime:     ReadableDuration(generated.Sub(s.StateChangeTracker.Started)),
		Up:         s.Up,
		Version:    s.Version,
		Stats:      s.StateChangeTracker.GenReports(s.Up, generated, periods),
		CheckCount: s.StateChangeTracker.UpdateCount,
		LastUpdate: ReadableDuration(generated.Sub(s.StateChangeTracker.LastUpdated)),
	}
}
