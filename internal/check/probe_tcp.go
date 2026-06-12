package check

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Dialer dials a TCP connection.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// TCPProbe performs TCP connectivity checks.
type TCPProbe struct {
	HostPort string
	dialer   Dialer
}

// NewTCPProbe creates a new TCP probe for the given host:port.
func NewTCPProbe(hostPort string) *TCPProbe {
	return &TCPProbe{HostPort: hostPort}
}

// Scheme returns the protocol scheme (tcp).
func (TCPProbe) Scheme() string {
	return TCP
}

// Target returns the host:port being probed.
func (p TCPProbe) Target() string {
	return p.HostPort
}

// Execute runs the TCP connection attempt and returns a report.
func (p TCPProbe) Execute(ctx context.Context, timeout time.Duration) *Report {
	start := time.Now()

	var (
		conn net.Conn
		err  error
	)

	if p.dialer != nil {
		conn, err = p.dialer.DialContext(ctx, "tcp", p.HostPort)
	} else {
		d := &net.Dialer{
			Timeout: timeout,
		}
		conn, err = d.DialContext(ctx, "tcp", p.HostPort)
	}

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
