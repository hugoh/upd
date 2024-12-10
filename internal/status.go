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

func (s *Status) Set(up bool) {
	s.mutex.Lock()
	if !s.Initialized {
		s.Initialized = true
	}
	s.Up = up
	s.mutex.Unlock()
}

func (s *Status) HasChanged(newStatus bool) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return !s.Initialized || newStatus != s.Up
}

func (s *Status) RecordResult(up bool) {
	if s.StateChangeTracker == nil {
		return
	}
	s.mutex.Lock()
	s.StateChangeTracker.RecordChange(time.Now(), up)
	s.mutex.Unlock()
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
