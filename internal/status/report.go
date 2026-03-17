package status

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/hugoh/upd/internal/logger"
)

const (
	// JSONIndentSpaces is the indentation used for JSON output.
	JSONIndentSpaces = "    "
	// FailedToGenerateJSONMsg is the error message for JSON generation failures.
	FailedToGenerateJSONMsg = "Failed to generate JSON"
)

// ReportByPeriod contains uptime statistics for a specific time period.
type ReportByPeriod struct {
	Period       ReadableDuration `json:"period"`
	Availability ReadablePercent  `json:"availability"`
	Downtime     ReadableDuration `json:"downTime"`
}

// Report contains the full status report with statistics.
type Report struct {
	Up         bool             `json:"isUp"`
	Stats      []ReportByPeriod `json:"reports"`
	CheckCount int64            `json:"totalChecksRun"`
	LastUpdate ReadableDuration `json:"timeSinceLastUpdate"`
	Uptime     ReadableDuration `json:"updUptime"`
	Version    string           `json:"updVersion"`
	Generated  time.Time        `json:"generatedAt"`
}

// StatHandler handles HTTP requests for statistics.
type StatHandler struct {
	statServer *StatServer
}

// ErrCompilingTemplate is returned when template compilation fails.
var ErrCompilingTemplate = errors.New("error compiling HTML template")

// NewStatHandler creates a new statistics handler for the given server.
func NewStatHandler(server *StatServer) *StatHandler {
	return &StatHandler{
		statServer: server,
	}
}

// GenStatReport generates a statistics report from the server's status.
func (h *StatHandler) GenStatReport() *Report {
	return h.statServer.status.GenStatReport(h.statServer.config.Reports)
}

func (h *StatHandler) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
	logger.L.WithField("requester", r.RemoteAddr).Info("[Stats] requested")

	stats := h.GenStatReport()

	writer.Header().Set("Content-Type", "application/json")

	jsonData, err := json.MarshalIndent(stats, "", JSONIndentSpaces)
	if err != nil {
		logger.L.WithError(err).Error("[Stats] error marshalling JSON stats")
		http.Error(writer, FailedToGenerateJSONMsg, http.StatusInternalServerError)

		return
	}

	_, err = writer.Write(jsonData)
	if err != nil {
		logger.L.WithError(err).Error("[Stats] error returning JSON stats")
	}
}
