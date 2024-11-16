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

// Protocols included in the library.
var Protocols []*Protocol //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	httpProtocol := &Protocol{ //nolint:exhaustruct
		ID:    "http",
		RHost: RandomCaptivePortal,
	}
	httpProtocol.Probe = func(domain string, timeout time.Duration) (string, error) {
		return httpProtocol.httpProbe(domain, timeout)
	}

	tcpProtocol := &Protocol{ //nolint:exhaustruct
		ID:    "tcp",
		RHost: RandomTCPServer,
	}
	tcpProtocol.Probe = func(domain string, timeout time.Duration) (string, error) {
		return tcpProtocol.tcpProbe(domain, timeout)
	}

	dnsProtocol := &Protocol{ //nolint:exhaustruct
		ID:    "dns",
		RHost: RandomDomain,
	}
	dnsProtocol.Probe = func(domain string, timeout time.Duration) (string, error) {
		return dnsProtocol.dnsProbe(domain, timeout)
	}

	Protocols = []*Protocol{httpProtocol, tcpProtocol, dnsProtocol}
}

// Protocol defines a probe attempt.
type Protocol struct {
	ID string
	// Probe implementation for this protocol.
	// Returns extra information about the attempt or an error if it failed.
	Probe func(rhost string, timeout time.Duration) (string, error)
	// Function to create a random remote
	RHost func() (string, error)
	// customDNSResolver
	dnsResolver string
}

func (p *Protocol) WithDNSResolver(dnsResolver string) {
	p.dnsResolver = dnsResolver
}

// String returns an human-readable representation of the protocol.
func (p *Protocol) String() string {
	return p.ID
}

// Ensures the required properties are set.
func (p *Protocol) validate() error {
	if p.Probe == nil {
		return fmt.Errorf(tmplRequiredProp, "Probe")
	}
	if p.RHost == nil {
		return fmt.Errorf(tmplRequiredProp, "RHost")
	}
	return nil
}

// Makes an HTTP request.
//
// The extra information is the status code.
func (p *Protocol) httpProbe(u string, timeout time.Duration) (string, error) {
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
func (p *Protocol) tcpProbe(hostPort string, timeout time.Duration) (string, error) {
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
// TODO(#31)
//
//nolint:godox
func (p *Protocol) dnsProbe(domain string, timeout time.Duration) (string, error) {
	if p != nil && p.dnsResolver != "" {
		r := &net.Resolver{ //nolint:exhaustruct
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{ //nolint:exhaustruct
					Timeout: timeout,
				}
				return d.DialContext(ctx, network, p.dnsResolver)
			},
		}
		addr, err := r.LookupHost(context.Background(), domain)
		if err != nil {
			return "", fmt.Errorf("resolving %s: %w", domain, err)
		}
		return addr[0], nil
	}
	addrs, err := net.LookupHost(domain)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", domain, err)
	}
	return addrs[0], nil
}
