package pkg

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Probe interface {
	Probe(timeout time.Duration) *Report
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

func GetDNSProbe(dnsResolver string, domain string) Probe { //nolint:ireturn
	dnsProbe := DNSProbe{
		DNSResolver: dnsResolver,
		Domain:      domain,
	}
	return &dnsProbe
}

func (p *DNSProbe) Scheme() string {
	return DNS
}

func (p *DNSProbe) Probe(timeout time.Duration) *Report {
	r := &net.Resolver{ //nolint:exhaustruct
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{ //nolint:exhaustruct
				Timeout: timeout,
			}
			return d.DialContext(ctx, network, p.DNSResolver)
		},
	}
	start := time.Now()
	addr, err := r.LookupHost(context.Background(), p.Domain)
	report := BuildReport(p, start)
	if err != nil {
		report.Error = fmt.Errorf("error resolving %s: %w", p.Domain, err)
		return report
	}
	report.Response = fmt.Sprintf("%s @ %s", addr[0], p.DNSResolver)
	return report
}

func GetHTTPProbe(url string) Probe { //nolint:ireturn
	httpProbe := HTTPProbe{
		URL: url,
	}
	return &httpProbe
}

func (p *HTTPProbe) Scheme() string {
	return HTTP
}

func (p *HTTPProbe) Probe(timeout time.Duration) *Report {
	cli := &http.Client{Timeout: timeout} //nolint:exhaustruct
	start := time.Now()
	resp, err := cli.Get(p.URL) //nolint:noctx
	report := BuildReport(p, start)
	if err != nil {
		report.Error = fmt.Errorf("making request to %s: %w", p.URL, err)
		return report
	}
	err = resp.Body.Close()
	if err != nil {
		report.Error = fmt.Errorf("closing response body: %w", err)
		return report
	}
	report.Response = resp.Status
	return report
}

func GetTCPProbe(hostPort string) Probe { //nolint:ireturn
	tcpProbe := TCPProbe{
		HostPort: hostPort,
	}
	return &tcpProbe
}

func (p *TCPProbe) Scheme() string {
	return TCP
}

func (p *TCPProbe) Probe(timeout time.Duration) *Report {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", p.HostPort, timeout)
	report := BuildReport(p, start)
	if err != nil {
		report.Error = fmt.Errorf("making request to %s: %w", p.HostPort, err)
		return report
	}
	err = conn.Close()
	if err != nil {
		report.Error = fmt.Errorf("closing connection: %w", err)
		return report
	}
	report.Response = conn.LocalAddr().String()
	return report
}
