// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Probe interface {
	Probe(ctx context.Context, timeout time.Duration) *Report
	Scheme() string
}

type DNSProbe struct {
	DNSResolver string
	Domain      string
}

type HTTPProbe struct {
	URL string
}

type TCPProbe struct {
	HostPort string
}

func GetDNSProbe(dnsResolver string, domain string) *DNSProbe {
	dnsProbe := DNSProbe{
		DNSResolver: dnsResolver,
		Domain:      domain,
	}
	return &dnsProbe
}

func (p DNSProbe) Scheme() string {
	return DNS
}

func (p DNSProbe) Probe(ctx context.Context, timeout time.Duration) *Report {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: timeout,
			}
			return d.DialContext(ctx, network, p.DNSResolver)
		},
	}
	start := time.Now()
	addr, err := r.LookupHost(ctx, p.Domain)
	report := BuildReport(p, start)
	if err != nil {
		report.error = fmt.Errorf("error resolving %s: %w", p.Domain, err)
		return report
	}
	report.response = fmt.Sprintf("%s @ %s", addr[0], p.DNSResolver)
	return report
}

func GetHTTPProbe(url string) *HTTPProbe {
	httpProbe := HTTPProbe{
		URL: url,
	}
	return &httpProbe
}

func (p HTTPProbe) Scheme() string {
	return HTTP
}

func (p HTTPProbe) Probe(ctx context.Context, timeout time.Duration) *Report {
	client := &http.Client{Timeout: timeout}
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

func GetTCPProbe(hostPort string) *TCPProbe {
	tcpProbe := TCPProbe{
		HostPort: hostPort,
	}
	return &tcpProbe
}

func (p TCPProbe) Scheme() string {
	return TCP
}

func (p TCPProbe) Probe(_ context.Context, timeout time.Duration) *Report {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", p.HostPort, timeout)
	report := BuildReport(p, start)
	if err != nil {
		report.error = fmt.Errorf("error making request to %s: %w", p.HostPort, err)
		return report
	}
	err = conn.Close()
	if err != nil {
		report.error = fmt.Errorf("error closing connection: %w", err)
		return report
	}
	report.response = conn.LocalAddr().String()
	return report
}
