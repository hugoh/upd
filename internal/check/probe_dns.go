package check

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// DNSResolver resolves hostnames to IP addresses.
type DNSResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// DNSProbe performs DNS resolution connectivity checks.
type DNSProbe struct {
	DNSResolver string
	Domain      string
	resolver    DNSResolver
}

// DefaultDNSPort is the default DNS resolver port.
const DefaultDNSPort = "53"

var (
	// ErrDNSMissingDomain is returned when no domain is specified.
	ErrDNSMissingDomain = errors.New("DNS probe missing domain")
	// ErrDNSMissingResolver is returned when no resolver is specified.
	ErrDNSMissingResolver = errors.New("DNS probe missing resolver")
)

// NewDNSProbe creates a new DNS probe for the given resolver host (host:port
// or host-only, port defaults to 53) and domain.
func NewDNSProbe(host, domain string) (*DNSProbe, error) {
	if domain == "" {
		return nil, ErrDNSMissingDomain
	}

	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
		port = DefaultDNSPort
	}

	if hostname == "" {
		return nil, ErrDNSMissingResolver
	}

	return &DNSProbe{
		DNSResolver: net.JoinHostPort(hostname, port),
		Domain:      domain,
	}, nil
}

// Scheme returns the protocol scheme (dns).
func (DNSProbe) Scheme() string {
	return DNS
}

// Execute runs the DNS resolution and returns a report.
func (p DNSProbe) Execute(ctx context.Context, timeout time.Duration) *Report {
	resolver := p.resolver
	if resolver == nil {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: timeout,
				}

				return d.DialContext(ctx, network, p.DNSResolver)
			},
		}
	}

	start := time.Now()
	addr, err := resolver.LookupHost(ctx, p.Domain)

	report := BuildReport(p, start)
	if err != nil {
		report.error = fmt.Errorf("error resolving %s: %w", p.Domain, err)

		return report
	}

	report.response = fmt.Sprintf("%s @ %s", addr[0], p.DNSResolver)

	return report
}
