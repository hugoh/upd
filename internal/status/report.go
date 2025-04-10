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

type ReportByPeriod struct {
	Period       ReadableDuration `json:"period"`
	Availability ReadablePercent  `json:"availability"`
	Downtime     ReadableDuration `json:"downTime"`
}

type Report struct {
	Up         bool             `json:"isUp"`
	Stats      []ReportByPeriod `json:"reports"`
	CheckCount int64            `json:"totalChecksRun"`
	LastUpdate ReadableDuration `json:"timeSinceLastUpdate"`
	Uptime     ReadableDuration `json:"updUptime"`
	Version    string           `json:"updVersion"`
	Generated  time.Time        `json:"generatedAt"`
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

func (h *StatHandler) GenStatReport() *Report {
	return h.StatServer.Status.GenStatReport(h.StatServer.Config.Reports)
}

func (h *StatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.L.WithField("requestor", r.RemoteAddr).Info("[Stats] requested")

	stats := h.GenStatReport()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		logger.L.WithError(err).Error("[Stats] error output JSON stats")
	}
}
