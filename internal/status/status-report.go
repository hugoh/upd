package status

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

type StatusReportByPeriod struct {
	Period       ReadableDuration `json:"period"`
	Availability ReadablePercent  `json:"availability"`
}

type StatusReport struct {
	Up         bool                   `json:"isUp"`
	Stats      []StatusReportByPeriod `json:"reports"`
	CheckCount int64                  `json:"totalChecksRun"`
	LastUpdate ReadableDuration       `json:"timeSinceLastUpdate"`
	Uptime     ReadableDuration       `json:"updUptime"`
	Version    string                 `json:"updVersion"`
	Generated  time.Time              `json:"generatedAt"`
}

type StatHandler struct {
	StatServer *StatServer
}

var ErrCompilingTemplate = errors.New("error compiling HTML template")

func NewStatHandler(server *StatServer) *StatHandler {
	return &StatHandler{
		StatServer: server,
	}
}

//go:embed static/stats.min.html
var statPage string

func StatPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "max-age=604800") // 7 days
	_, err := fmt.Fprint(w, statPage)
	if err != nil {
		http.Error(w, "Failed to return stats page", http.StatusInternalServerError)
	}
}

func (h *StatHandler) GenStatReport() *StatusReport {
	return h.StatServer.Status.GenStatReport(h.StatServer.Config.Reports)
}

func (h *StatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Logger.WithField("requestor", r.RemoteAddr).Info("[Stats] requested")

	stats := h.GenStatReport()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		logger.Logger.WithError(err).Error("[Stats] error output JSON stats")
	}
}
