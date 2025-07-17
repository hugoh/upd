package status

import (
	"sync"
	"time"

	"github.com/hugoh/upd/pkg"
)

type Status struct {
	Up                 bool
	initialized        bool
	mutex              sync.Mutex
	stateChangeTracker *StateChangeTracker
}

func NewStatus() *Status {
	var stateChangeTracker *StateChangeTracker
	return &Status{
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
			retention: retention,
			started:   time.Now(),
		}
	} else {
		s.stateChangeTracker.retention = retention
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
		Uptime:     ReadableDuration(generated.Sub(s.stateChangeTracker.started)),
		Up:         s.Up,
		Version:    pkg.Version(),
		Stats:      s.stateChangeTracker.GenReports(s.Up, generated, periods),
		CheckCount: s.stateChangeTracker.updateCount,
		LastUpdate: ReadableDuration(generated.Sub(s.stateChangeTracker.lastUpdated)),
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
