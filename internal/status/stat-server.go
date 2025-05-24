package status

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

type StatServerConfig struct {
	port      string `validate:"omitempty,validTCPPort"`
	reports   []time.Duration
	retention time.Duration
}

type StatServer struct {
	config *StatServerConfig
	server *http.Server
	status *Status
}

func (c StatServerConfig) GetRetention() time.Duration {
	return c.retention
}

func StartStatServer(status *Status, config *StatServerConfig) *StatServer {
	if config.port == "" {
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
	const StatRoute = "/stats.json"
	const ReqTimeout = 3 * time.Second
	const IdleTimeout = 3 * time.Second
	mux := http.NewServeMux()
	statHandler := NewStatHandler(s)
	mux.Handle(StatRoute, statHandler)
	s.server = &http.Server{
		Addr:         s.config.port,
		Handler:      mux,
		ReadTimeout:  ReqTimeout,
		WriteTimeout: ReqTimeout,
		IdleTimeout:  IdleTimeout,
	}
	logger.L.WithField("statserver", fmt.Sprintf("http://localhost%s%s", s.server.Addr, StatRoute)).
		Info("[Stats] server started")
	if err := s.server.ListenAndServe(); err != nil {
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
