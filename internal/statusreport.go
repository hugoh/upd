package internal

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
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
	logger.Trace("[Stats] generating stats")
	generated := time.Now()
	var reports []StatusReportByPeriod
	reportCount := len(h.StatServer.Config.Reports)
	logger.WithField("reportCount", reportCount).Trace("[Stats] reports to generate")
	if reportCount > 0 {
		reports = make([]StatusReportByPeriod, reportCount)
		for i := range reportCount {
			period := h.StatServer.Config.Reports[i]
			logger.WithField("period", period).Trace("[Stats] generating report for period")
			availability, err := h.StatServer.Status.StateChangeTracker.
				CalculateUptime(h.StatServer.Status.Up, period, generated)
			if err != nil {
				logger.WithError(err).WithField("period", period).Debug("[Stats] invalid range for stat report")
			}
			reports[i] = StatusReportByPeriod{
				Period:       ReadableDuration(period),
				Availability: ReadablePercent(availability),
			}
			logger.WithField("report", reports[i]).Trace("[Stats] generated report for period")
		}
	}
	logger.WithField("reports", reports).Trace("[Stats] computed reports")
	return &StatusReport{
		Generated:  generated,
		Uptime:     ReadableDuration(generated.Sub(h.StatServer.Status.StateChangeTracker.Started)),
		Up:         h.StatServer.Status.Up,
		Version:    h.StatServer.Status.Version,
		Stats:      reports,
		CheckCount: h.StatServer.Status.StateChangeTracker.UpdateCount,
		LastUpdate: ReadableDuration(generated.Sub(h.StatServer.Status.StateChangeTracker.LastUpdated)),
	}
}

func (h *StatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.WithField("requestor", r.RemoteAddr).Info("[Stats] requested")

	stats := h.GenStatReport()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		logger.WithError(err).Error("[Stats] error output JSON stats")
	}
}
