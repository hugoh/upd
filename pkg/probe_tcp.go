package pkg

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCPProbe performs TCP connectivity checks.
type TCPProbe struct {
	HostPort string
}

// NewTCPProbe creates a new TCP probe for the given host:port.
func NewTCPProbe(hostPort string) *TCPProbe {
	return &TCPProbe{HostPort: hostPort}
}

// Scheme returns the protocol scheme (tcp).
func (p TCPProbe) Scheme() string {
	return TCP
}

// Execute runs the TCP connection attempt and returns a report.
func (p TCPProbe) Execute(ctx context.Context, timeout time.Duration) *Report {
	start := time.Now()
	dialer := &net.Dialer{
		Timeout: timeout,
	}
	conn, err := dialer.DialContext(ctx, "tcp", p.HostPort)

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
