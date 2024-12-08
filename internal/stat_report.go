package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

type StatReportByPeriod struct {
	Period       CustomDuration   `json:"period"`
	Availability PercentAvailable `json:"availability"`
}

type StatReport struct {
	Up        bool                 `json:"currentlyUp"`
	Uptime    CustomDuration       `json:"uptime"`
	Stats     []StatReportByPeriod `json:"stats"`
	Version   string               `json:"version"`
	Generated time.Time            `json:"generated"`
}

type StatHandler struct {
	StatServer *StatServer
	template   *template.Template
}

type PercentAvailable float64

func (p PercentAvailable) MarshalJSON() ([]byte, error) {
	const Hundred = 100
	if p == -1.0 {
		return json.Marshal("Not computed") //nolint:wrapcheck
	}
	return json.Marshal(fmt.Sprintf("%.2f %%", p*Hundred)) //nolint:wrapcheck
}

type CustomDuration time.Duration

func (d CustomDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String()) //nolint:wrapcheck
}

var ErrCompilingTemplate = errors.New("error compiling HTML template")

func NewStatHandler(server *StatServer) (*StatHandler, error) {
	tmpl := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Upd Status</title>
		<link href="https://cdn.jsdelivr.net/npm/prismjs/themes/prism.min.css" rel="stylesheet" />
		<script src="https://cdn.jsdelivr.net/npm/prismjs/prism.min.js"></script>
		<script src="https://cdn.jsdelivr.net/npm/prismjs/components/prism-json.min.js"></script>
		<style>
			body { font-family: Arial, sans-serif; margin: 20px; }
			pre { border: 1px solid #ddd; padding: 10px; border-radius: 5px; background: #f9f9f9; }
		</style>
	</head>
	<body>
		<h1>Upd Status</h1>
        <pre><code class="language-json" id="json"></code></pre>

        <script>
            fetch("{{.}}")
                .then(response => response.json())
                .then(data => {
                    document.getElementById("json").textContent = JSON.stringify(data, null, 2);
                    Prism.highlightAll();
                })
                .catch(error => console.error("Error fetching JSON:", error));
        </script>

	</body>
	</html>`

	t, err := template.New("stats").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCompilingTemplate, err)
	}
	return &StatHandler{
		StatServer: server,
		template:   t,
	}, nil
}

func (h *StatHandler) GenStatReport() *StatReport {
	logger.Trace("[Stats] generating stats")
	generated := time.Now()
	var reports []StatReportByPeriod
	reportCount := len(h.StatServer.Config.Reports)
	logger.WithField("reportCount", reportCount).Trace("[Stats] reports to generate")
	if reportCount > 0 {
		reports = make([]StatReportByPeriod, reportCount)
		for i := range reportCount {
			period := h.StatServer.Config.Reports[i]
			logger.WithField("period", period).Trace("[Stats] generating report for period")
			availability, err := h.StatServer.Status.StateChangeTracker.
				CalculateUptime(h.StatServer.Status.Up, period, generated)
			if err != nil {
				logger.WithError(err).WithField("period", period).Debug("[Stats] invalid range for stat report")
			}
			reports[i] = StatReportByPeriod{
				Period:       CustomDuration(period),
				Availability: PercentAvailable(availability),
			}
			logger.WithField("report", reports[i]).Trace("[Stats] generated report for period")
		}
	}
	logger.WithField("reports", reports).Trace("[Stats] computed reports")
	return &StatReport{
		Generated: generated,
		Uptime:    CustomDuration(generated.Sub(h.StatServer.Status.StateChangeTracker.Started)),
		Up:        h.StatServer.Status.Up,
		Version:   h.StatServer.Status.Version,
		Stats:     reports,
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

func htmlHandler(h *StatHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonURL := r.RequestURI + ".json"

		err := h.template.Execute(w, jsonURL)
		if err != nil {
			logger.WithError(err).Error("[Stats] error rendering HTML")
		}
	}
}
