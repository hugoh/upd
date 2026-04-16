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
	// DefaultStatServerReadTimeout is the default read timeout for the stats server.
	DefaultStatServerReadTimeout = 3 * time.Second
	// DefaultStatServerWriteTimeout is the default write timeout for the stats server.
	DefaultStatServerWriteTimeout = 3 * time.Second
	// DefaultStatServerIdleTimeout is the default idle timeout for the stats server.
	DefaultStatServerIdleTimeout = 3 * time.Second
	// StatRoute is the HTTP route for the statistics endpoint.
	StatRoute = "/stats.json"
)

// StatServerConfig holds configuration for the statistics HTTP server.
//
//nolint:tagalign // golines formatter reorders tags differently than tagalign expects
type StatServerConfig struct {
	Port         string `validate:"omitempty,validTCPPort"`
	Reports      []time.Duration
	Retention    time.Duration
	ReadTimeout  time.Duration `validate:"omitempty,gte=0"        koanf:"readTimeout"`
	WriteTimeout time.Duration `validate:"omitempty,gte=0"        koanf:"writeTimeout"`
	IdleTimeout  time.Duration `validate:"omitempty,gte=0"        koanf:"idleTimeout"`
}

// StatServer provides an HTTP endpoint for status statistics.
type StatServer struct {
	config *StatServerConfig
	server *http.Server
	status *Status
}

// StartStatServer starts a new statistics server in a goroutine.
func StartStatServer(status *Status, config *StatServerConfig) *StatServer {
	if config.Port == "" {
		logger.L.Debug("no stat server specified")

		return nil
	}

	readTimeout := config.ReadTimeout
	if readTimeout == 0 {
		readTimeout = DefaultStatServerReadTimeout
	}

	writeTimeout := config.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = DefaultStatServerWriteTimeout
	}

	idleTimeout := config.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = DefaultStatServerIdleTimeout
	}

	server := &StatServer{
		status: status,
		config: config,
		server: &http.Server{
			Addr:         config.Port,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
	}

	mux := http.NewServeMux()
	statHandler := NewStatHandler(server)
	mux.Handle(StatRoute, statHandler)
	server.server.Handler = mux

	go server.listenAndServe()

	return server
}

// StopStatServer gracefully shuts down the statistics server.
func (s *StatServer) StopStatServer(ctx context.Context) {
	if s.server == nil {
		return
	}

	logger.L.Info("[Stats] shutting down stats server")

	err := s.server.Shutdown(ctx)
	if err != nil {
		logger.L.Error("[Stats] error shutting down stats server", "error", err)
	}
}

func (s *StatServer) listenAndServe() {
	logger.L.Info(
		"[Stats] server started",
		"statserver",
		fmt.Sprintf("http://localhost%s%s", s.server.Addr, StatRoute),
	)

	err := s.server.ListenAndServe()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}

		logger.L.Error("[Stats] error starting stats server", "error", err)
	}
}
