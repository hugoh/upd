package internal

import (
	"encoding/json"
	"errors"
	"fmt"
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

var ErrCompilingTemplate = errors.New("error compiling HTML template")

func NewStatHandler(server *StatServer) (*StatHandler, error) {
	// HTML template with Prism.js integration
	tmpl := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Struct Pretty Print</title>
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

func (h *StatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.WithField("requestor", r.RemoteAddr).Info("[Stats] requested")

	data := StatReport{
		Up: true,
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logrus.WithError(err).Error("[Stats] error output JSON stats")
	}
}

func htmlHandler(h *StatHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonURL := r.RequestURI + ".json"

		err := h.template.Execute(w, jsonURL)
		if err != nil {
			logrus.WithError(err).Error("[Stats] error rendering HTML")
		}
	}
}
