package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-playground/validator"
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

func isValidTCPPort(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^:(6553[0-5]|655[0-2][0-9]|65[0-4][0-9]{2}|6[0-4][0-9]{3}|[1-5][0-9]{4}|[0-9]{1,4})$`)
	return re.MatchString(fl.Field().String())
}

func StartStatServer(status *Status, config *StatServerConfig) {
	if config.Port == "" {
		logger.Debug("no stat server specified")
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
	statHandler, err := NewStatHandler(s)
	if err != nil {
		logger.WithError(err).Error("[Stats] error starting stats server")
		return
	}
	mux.Handle(StatRoute+".json", statHandler)
	mux.HandleFunc(StatRoute, htmlHandler(statHandler))
	server := &http.Server{
		Addr:         s.Config.Port,
		Handler:      mux,
		ReadTimeout:  ReqTimeout,
		WriteTimeout: ReqTimeout,
		IdleTimeout:  IdleTimeout,
	}
	logger.WithField("statserver", fmt.Sprintf("http://localhost%s%s", server.Addr, StatRoute)).
		Info("[Stats] server started")
	if err := server.ListenAndServe(); err != nil {
		logger.WithError(err).Error("[Stats] error starting stats server")
	}
}
