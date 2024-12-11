package status

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

type StatServerConfig struct {
	Port      string `validate:"omitempty,validTCPPort"`
	Retention time.Duration
	Reports   []time.Duration
}

type StatServer struct {
	Status *Status
	Config *StatServerConfig
}

func StartStatServer(status *Status, config *StatServerConfig) {
	if config.Port == "" {
		logger.L.Debug("no stat server specified")
		return
	}
	server := StatServer{
		Status: status,
		Config: config,
	}
	go server.Start()
}

func (s *StatServer) Start() {
	const StatRoute = "/stats"
	const ReqTimeout = 3 * time.Second
	const IdleTimeout = 3 * time.Second
	mux := http.NewServeMux()
	statHandler := NewStatHandler(s)
	mux.Handle(StatRoute+".json", statHandler)
	mux.HandleFunc(StatRoute, StatPage)
	server := &http.Server{
		Addr:         s.Config.Port,
		Handler:      mux,
		ReadTimeout:  ReqTimeout,
		WriteTimeout: ReqTimeout,
		IdleTimeout:  IdleTimeout,
	}
	logger.L.WithField("statserver", fmt.Sprintf("http://localhost%s%s", server.Addr, StatRoute)).
		Info("[Stats] server started")
	if err := server.ListenAndServe(); err != nil {
		logger.L.WithError(err).Error("[Stats] error starting stats server")
	}
}
