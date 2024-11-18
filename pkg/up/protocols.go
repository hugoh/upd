// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package up

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

//nolint:gochecknoglobals
var probes = map[string]func(p *Protocol, rhost string, timeout time.Duration) (string, error){
	"dns":  dnsProbe,
	"http": httpProbe,
	"tcp":  tcpProbe,
}

func ProtocolByID(id string) (Protocol, error) {
	if id == "https" || id == "http" {
		return Protocol{ //nolint:exhaustruct
			ID: "http",
		}, nil
	}
	if id == "tcp" {
		return Protocol{ //nolint:exhaustruct
			ID: "tcp",
		}, nil
	}
	if id == "dns" {
		return Protocol{ //nolint:exhaustruct
			ID: "dns",
		}, nil
	}
	return Protocol{}, fmt.Errorf("unknown protocol id %s", id)
}

// Protocol defines a probe attempt.
type Protocol struct {
	ID string
	// customDNSResolver
	DNSResolver string
}

// String returns an human-readable representation of the protocol.
func (p *Protocol) String() string {
	return p.ID
}

// Probe implementation for this protocol.
// Returns extra information about the attempt or an error if it failed.
func (p *Protocol) Probe(rhost string, timeout time.Duration) (string, error) {
	probe, ok := probes[p.ID]
	if !ok {
		return "", fmt.Errorf("internal error: no probe for protocol %s", p.ID)
	}
	return probe(p, rhost, timeout)
}

// Makes an HTTP request.
//
// The extra information is the status code.
func httpProbe(_ *Protocol, u string, timeout time.Duration) (string, error) {
	cli := &http.Client{Timeout: timeout} //nolint:exhaustruct
	resp, err := cli.Get(u)               //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("making request to %s: %w", u, err)
	}
	err = resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("closing response body: %w", err)
	}
	return resp.Status, nil
}

// Makes a TCP request.
//
// The extra information is the local host/port.
func tcpProbe(_ *Protocol, hostPort string, timeout time.Duration) (string, error) {
	conn, err := net.DialTimeout("tcp", hostPort, timeout)
	if err != nil {
		return "", fmt.Errorf("making request to %s: %w", hostPort, err)
	}
	err = conn.Close()
	if err != nil {
		return "", fmt.Errorf("closing connection: %w", err)
	}
	return conn.LocalAddr().String(), nil
}

// Resolves a domain name.
//
// The extra information is the first resolved IP address.
func dnsProbe(p *Protocol, domain string, timeout time.Duration) (string, error) {
	if p.DNSResolver != "" {
		r := &net.Resolver{ //nolint:exhaustruct
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{ //nolint:exhaustruct
					Timeout: timeout,
				}
				return d.DialContext(ctx, network, p.DNSResolver)
			},
		}
		addr, err := r.LookupHost(context.Background(), domain)
		if err != nil {
			return "", fmt.Errorf("error resolving %s: %w", domain, err)
		}
		return fmt.Sprintf("%s @ %s", addr[0], p.DNSResolver), nil
	}
	addrs, err := net.LookupHost(domain)
	if err != nil {
		return "", fmt.Errorf("error resolving %s: %w", domain, err)
	}
	return addrs[0] + " @ default", nil
}
