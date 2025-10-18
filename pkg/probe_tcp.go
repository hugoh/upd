package pkg

import (
	"context"
	"fmt"
	"net"
	"time"
)

type TCPProbe struct {
	HostPort string
}

func NewTCPProbe(hostPort string) *TCPProbe {
	tcpProbe := TCPProbe{
		HostPort: hostPort,
	}
	return &tcpProbe
}

func (p TCPProbe) Scheme() string {
	return TCP
}

func (p TCPProbe) Probe(ctx context.Context, timeout time.Duration) *Report {
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
