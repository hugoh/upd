package status

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

const (
	// DefaultStatServerReadTimeout is the default read timeout for the stats server
	DefaultStatServerReadTimeout = 3 * time.Second
	// DefaultStatServerWriteTimeout is the default write timeout for the stats server
	DefaultStatServerWriteTimeout = 3 * time.Second
	// DefaultStatServerIdleTimeout is the default idle timeout for the stats server
	DefaultStatServerIdleTimeout = 3 * time.Second
	// StatRoute is the HTTP route for the statistics endpoint
	StatRoute = "/stats.json"
)

type StatServerConfig struct {
	Port      string `validate:"omitempty,validTCPPort"`
	Reports   []time.Duration
	Retention time.Duration
	// Timeouts for the HTTP server
	ReadTimeout  time.Duration `koanf:"readTimeout"  validate:"omitempty,gte=0"`
	WriteTimeout time.Duration `koanf:"writeTimeout" validate:"omitempty,gte=0"`
	IdleTimeout  time.Duration `koanf:"idleTimeout"  validate:"omitempty,gte=0"`
}

type StatServer struct {
	config *StatServerConfig
	server *http.Server
	status *Status
}

func StartStatServer(status *Status, config *StatServerConfig) *StatServer {
	if config.Port == "" {
		logger.L.Debug("no stat server specified")
		return nil
	}
	server := StatServer{
		status: status,
		config: config,
	}
	go server.Start()
	return &server
}

func (s *StatServer) Start() {
	// Use configured timeouts or fall back to defaults
	readTimeout := s.config.ReadTimeout
	if readTimeout == 0 {
		readTimeout = DefaultStatServerReadTimeout
	}

	writeTimeout := s.config.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = DefaultStatServerWriteTimeout
	}

	idleTimeout := s.config.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = DefaultStatServerIdleTimeout
	}

	mux := http.NewServeMux()
	statHandler := NewStatHandler(s)
	mux.Handle(StatRoute, statHandler)
	s.server = &http.Server{
		Addr:         s.config.Port,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	logger.L.WithField("statserver", fmt.Sprintf("http://localhost%s%s", s.server.Addr, StatRoute)).
		Info("[Stats] server started")
	err := s.server.ListenAndServe()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		logger.L.WithError(err).Error("[Stats] error starting stats server")
	}
}

func (s *StatServer) StopStatServer(ctx context.Context) {
	if s.server == nil {
		return
	}
	logger.L.Info("[Stats] shutting down stats server")
	err := s.server.Shutdown(ctx)
	if err != nil {
		logger.L.WithError(err).Error("[Stats] error shutting down stats server")
	}
}
