package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/version"
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
type StatServerConfig struct {
	Port         int
	Reports      []time.Duration
	Retention    time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// StatServer provides an HTTP endpoint for status statistics.
type StatServer struct {
	config *StatServerConfig
	server *http.Server
	status *Status
}

// StartStatServer starts a new statistics server in a goroutine.
func StartStatServer(status *Status, config *StatServerConfig) *StatServer {
	if config.Port == 0 {
		logger.L.Debug("no stat server specified", logger.LogComponent, logger.LogComponentStats)

		return nil
	}

	server := &StatServer{
		status: status,
		config: config,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			ReadTimeout:  defaultTimeout(config.ReadTimeout, DefaultStatServerReadTimeout),
			WriteTimeout: defaultTimeout(config.WriteTimeout, DefaultStatServerWriteTimeout),
			IdleTimeout:  defaultTimeout(config.IdleTimeout, DefaultStatServerIdleTimeout),
		},
	}

	mux := http.NewServeMux()
	mux.Handle(StatRoute, &StatHandler{statServer: server})
	server.server.Handler = serverHeader(mux)

	go server.listenAndServe()

	return server
}

// Shutdown gracefully shuts down the statistics server.
func (s *StatServer) Shutdown(ctx context.Context) {
	if s.server == nil {
		return
	}

	logger.L.Info("shutting down stats server", logger.LogComponent, logger.LogComponentStats)

	if err := s.server.Shutdown(ctx); err != nil {
		logger.L.Error(
			"error shutting down stats server",
			logger.LogComponent,
			logger.LogComponentStats,
			"error",
			err,
		)
	}
}

func (s *StatServer) listenAndServe() {
	logger.L.Info("server started",
		logger.LogComponent, logger.LogComponentStats,
		"statserver", fmt.Sprintf("http://localhost%s%s", s.server.Addr, StatRoute),
	)

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.L.Error(
			"error starting stats server",
			logger.LogComponent,
			logger.LogComponentStats,
			"error",
			err,
		)
	}
}

// StatHandler handles HTTP requests for statistics.
type StatHandler struct {
	statServer *StatServer
}

// GenStatReport generates a statistics report from the server's status.
func (h *StatHandler) GenStatReport() *Report {
	return h.statServer.status.GenStatReport(h.statServer.config.Reports)
}

func (h *StatHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	logger.L.Debug(
		"requested",
		logger.LogComponent,
		logger.LogComponentStats,
		"requester",
		req.RemoteAddr,
	)

	writeJSON(writer, h.GenStatReport())
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetIndent("", JSONIndentSpaces)

	if err := enc.Encode(data); err != nil {
		logger.L.Error(
			"error writing JSON response",
			logger.LogComponent,
			logger.LogComponentStats,
			"error",
			err,
		)
	}
}

func serverHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "upd/"+version.Version())
		next.ServeHTTP(w, r)
	})
}

func defaultTimeout(d, def time.Duration) time.Duration {
	if d == 0 {
		return def
	}

	return d
}
