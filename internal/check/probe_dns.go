package check

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DNSProbe performs DNS resolution connectivity checks.
type DNSProbe struct {
	DNSResolver string
	Domain      string
}

// NewDNSProbe creates a new DNS probe for the given resolver and domain.
func NewDNSProbe(dnsResolver, domain string) *DNSProbe {
	return &DNSProbe{
		DNSResolver: dnsResolver,
		Domain:      domain,
	}
}

// Scheme returns the protocol scheme (dns).
func (DNSProbe) Scheme() string {
	return DNS
}

// Execute runs the DNS resolution and returns a report.
func (p DNSProbe) Execute(ctx context.Context, timeout time.Duration) *Report {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: timeout,
			}

			return d.DialContext(ctx, network, p.DNSResolver)
		},
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
