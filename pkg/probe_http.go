package pkg

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	// UserAgentPrefix is prepended to the version in the User-Agent header.
	UserAgentPrefix = "upd/"
)

// updClient is a shared HTTP client for all HTTP probes.
// Using a single client enables connection pooling and improves performance.
//
//nolint:gochecknoglobals // Intentional singleton for connection pooling
var updClient = &http.Client{
	Transport: &updTransport{version: version},
}

type updTransport struct {
	version string
}

func (t *updTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", UserAgentPrefix+t.version)
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("updTransport RoundTrip error: %w", err)
	}

	return resp, nil
}

// HTTPProbe performs HTTP connectivity checks.
type HTTPProbe struct {
	URL    string
	client *http.Client
}

// NewHTTPProbe creates a new HTTP probe for the given URL.
func NewHTTPProbe(url string) *HTTPProbe {
	return &HTTPProbe{URL: url, client: updClient}
}

// Scheme returns the protocol scheme (http or https).
func (p *HTTPProbe) Scheme() string {
	return HTTP
}

// Probe executes the HTTP request and returns a report.
func (p *HTTPProbe) Probe(ctx context.Context, timeout time.Duration) *Report {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, bErr := http.NewRequestWithContext(ctxWithTimeout, http.MethodGet, p.URL, nil)
	if bErr != nil {
		start := time.Now()
		report := BuildReport(p, start)
		report.error = fmt.Errorf("error building request to %s: %w", p.URL, bErr)

		return report
	}
	start := time.Now()
	resp, err := p.client.Do(req)
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
