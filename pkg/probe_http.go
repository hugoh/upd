package pkg

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

var updClient = &http.Client{ //nolint:gochecknoglobals
	Transport: &updTransport{version: version},
}

type updTransport struct {
	version string
}

func (t *updTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", "upd/"+t.version)
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("updTransport RoundTrip error: %w", err)
	}
	return resp, nil
}

type HTTPProbe struct {
	URL    string
	client *http.Client
}

func NewHTTPProbe(url string) *HTTPProbe {
	return &HTTPProbe{URL: url, client: updClient}
}

func (p *HTTPProbe) Scheme() string {
	return HTTP
}

func (p *HTTPProbe) Probe(ctx context.Context, timeout time.Duration) *Report {
	client := p.client
	client.Timeout = timeout
	req, bErr := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if bErr != nil {
		report := &Report{protocol: p.Scheme()}
		report.error = fmt.Errorf("error building request to %s: %w", p.URL, bErr)
		return report
	}
	start := time.Now()
	resp, err := client.Do(req)
	report := BuildReport(p, start)
	if err != nil {
		report.error = fmt.Errorf("error making request to %s: %w", p.URL, err)
		return report
	}
	err = resp.Body.Close()
	if err != nil {
		report.error = fmt.Errorf("error closing response body: %w", err)
		return report
	}
	report.response = resp.Status
	return report
}
