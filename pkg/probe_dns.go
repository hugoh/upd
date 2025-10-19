package pkg

import (
	"context"
	"fmt"
	"net"
	"time"
)

type DNSProbe struct {
	DNSResolver string
	Domain      string
}

func NewDNSProbe(dnsResolver string, domain string) *DNSProbe {
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
