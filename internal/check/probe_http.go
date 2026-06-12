package check

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hugoh/upd/internal/version"
)

const (
	// UserAgentPrefix is prepended to the version in the User-Agent header.
	UserAgentPrefix = "upd/"

	// maxBodyDrain caps how much of the response body is read before closing
	// so the pooled connection can be reused without downloading arbitrarily
	// large bodies.
	maxBodyDrain = 4096
)

// updClient is a shared HTTP client for all HTTP probes.
// Using a single client enables connection pooling and improves performance.
//
//nolint:gochecknoglobals // Intentional singleton for connection pooling
var updClient = &http.Client{}

// HTTPProbe performs HTTP connectivity checks.
type HTTPProbe struct {
	URL    string
	scheme string
	client *http.Client
}

// NewHTTPProbe creates a new HTTP probe for the given URL.
func NewHTTPProbe(url string) *HTTPProbe {
	scheme := HTTP
	if strings.HasPrefix(url, HTTPS+":") {
		scheme = HTTPS
	}

	return &HTTPProbe{URL: url, scheme: scheme, client: updClient}
}

// Scheme returns the protocol scheme (http or https).
func (p *HTTPProbe) Scheme() string {
	return cmp.Or(p.scheme, HTTP)
}

// Target returns the URL being probed.
func (p *HTTPProbe) Target() string {
	return p.URL
}

// Execute runs the HTTP request and returns a report.
func (p *HTTPProbe) Execute(ctx context.Context, timeout time.Duration) *Report {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, bErr := http.NewRequestWithContext(ctxWithTimeout, http.MethodGet, p.URL, http.NoBody)
	if bErr != nil {
		start := time.Now()
		report := BuildReport(p, start)
		report.error = fmt.Errorf("error building request to %s: %w", p.URL, bErr)

		return report
	}

	req.Header.Set("User-Agent", UserAgentPrefix+version.Version())

	start := time.Now()
	resp, err := p.client.Do(req)

	report := BuildReport(p, start)
	if err != nil {
		report.error = fmt.Errorf("error making request to %s: %w", p.URL, err)

		return report
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && report.error == nil {
			report.error = fmt.Errorf("error closing response body: %w", closeErr)
		}
	}()

	// Drain (bounded) so the pooled connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyDrain))

	report.response = resp.Status

	return report
}
