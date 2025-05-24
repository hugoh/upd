package status

import (
	"sync"
	"time"
)

type Status struct {
	Up                 bool
	initialized        bool
	mutex              sync.Mutex
	stateChangeTracker *StateChangeTracker
	version            string
}

func NewStatus(version string) *Status {
	var stateChangeTracker *StateChangeTracker
	return &Status{
		version:            version,
		stateChangeTracker: stateChangeTracker,
	}
}

func (s *Status) SetRetention(retention time.Duration) {
	if retention <= 0 {
		s.stateChangeTracker = nil
		return
	}
	if s.stateChangeTracker == nil {
		s.stateChangeTracker = &StateChangeTracker{
			Retention: retention,
			Started:   time.Now(),
		}
	} else {
		s.stateChangeTracker.Retention = retention
		s.stateChangeTracker.Prune(time.Now())
	}
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

func (s *Status) GenStatReport(periods []time.Duration) *Report {
	generated := time.Now()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return &Report{
		Generated:  generated,
		Uptime:     ReadableDuration(generated.Sub(s.stateChangeTracker.Started)),
		Up:         s.Up,
		Version:    s.version,
		Stats:      s.stateChangeTracker.GenReports(s.Up, generated, periods),
		CheckCount: s.stateChangeTracker.UpdateCount,
		LastUpdate: ReadableDuration(generated.Sub(s.stateChangeTracker.LastUpdated)),
	}
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
