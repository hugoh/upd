package status

import (
	"encoding/json"
	"errors"
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
	statServer *StatServer
}

var ErrCompilingTemplate = errors.New("error compiling HTML template")

func NewStatHandler(server *StatServer) *StatHandler {
	return &StatHandler{
		statServer: server,
	}
}

func (h *StatHandler) GenStatReport() *Report {
	return h.statServer.status.GenStatReport(h.statServer.config.reports)
}

func (h *StatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.L.WithField("requestor", r.RemoteAddr).Info("[Stats] requested")

	stats := h.GenStatReport()

	w.Header().Set("Content-Type", "application/json")

	jsonData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		logger.L.WithError(err).Error("[Stats] error marshalling JSON stats")
		http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(jsonData)
	if err != nil {
		logger.L.WithError(err).Error("[Stats] error returning JSON stats")
	}
}
