package internal

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type StatReport struct {
	Up     bool          `json:"currentlyUp"`
	Uptime time.Duration `json:"uptime"`
	Stats  []struct {
		Period       time.Duration `json:"period"`
		Availability float64       `json:"availability"`
	} `json:"stats"`
}

type StatHandler struct {
	StatServer *StatServer
	template   *template.Template
}

func NewStatHandler(server *StatServer) *StatHandler {
	// HTML template with Prism.js integration
	tmpl := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Struct Pretty Print</title>
		<link href="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/themes/prism.min.css" rel="stylesheet" />
		<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/prism.min.js"></script>
		<script src="https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/prism-json.min.js"></script>
		<style>
			body { font-family: Arial, sans-serif; margin: 20px; }
			pre { border: 1px solid #ddd; padding: 10px; border-radius: 5px; background: #f9f9f9; }
		</style>
	</head>
	<body>
		<h1>Upd Status</h1>
		<pre><code class="language-json">{{.}}</code></pre>
	</body>
	</html>`

	t, err := template.New("stats").Parse(tmpl)
	if err != nil {
		// FIXME: message
		return nil
	}
	return &StatHandler{
		StatServer: server,
		template:   t,
	}
}

func (h *StatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.WithField("requestor", r.RemoteAddr).Info("[Stats] requested")

	data := StatReport{
		Up: true,
	}

	// Convert struct to pretty-printed JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Error converting to JSON", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	escapedJSON := string(jsonData)

	err = h.template.Execute(w, escapedJSON)
	if err != nil {
		logrus.WithError(err).Error("formatting stats")
	}
}
