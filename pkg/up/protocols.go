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

const (
	DNS         string = "dns"
	HTTP        string = "http"
	HTTPS       string = "https"
	TCP         string = "tcp"
	DNSResolver string = "dnsResolver"
)

//nolint:gochecknoglobals
var protocols = map[string]Protocol{
	DNS:  DNSProtocol{},
	HTTP: HTTPProtocol{},
	TCP:  TCPProtocol{},
}

type Protocol interface {
	Type() string
	Probe(target string, extra map[string]string, timeout time.Duration) (string, error)
}

type DNSProtocol struct{}

type HTTPProtocol struct{}

type TCPProtocol struct{}

func ProtocolByScheme(scheme string) (*Protocol, bool) {
	if scheme == HTTPS {
		scheme = HTTP
	}
	p, ok := protocols[scheme]
	if !ok {
		return nil, ok
	}
	return &p, ok
}

func (p DNSProtocol) Type() string {
	return DNS
}

func (p HTTPProtocol) Type() string {
	return HTTP
}

func (p TCPProtocol) Type() string {
	return TCP
}

func (p HTTPProtocol) Probe(u string, _ map[string]string, timeout time.Duration) (string, error) {
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

func (p TCPProtocol) Probe(hostPort string, _ map[string]string, timeout time.Duration) (string, error) {
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

func (p DNSProtocol) Probe(domain string, extra map[string]string, timeout time.Duration) (string, error) {
	dnsResolver, ok := extra[DNSResolver]
	if ok {
		r := &net.Resolver{ //nolint:exhaustruct
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{ //nolint:exhaustruct
					Timeout: timeout,
				}
				return d.DialContext(ctx, network, dnsResolver)
			},
		}
		addr, err := r.LookupHost(context.Background(), domain)
		if err != nil {
			return "", fmt.Errorf("error resolving %s: %w", domain, err)
		}
		return fmt.Sprintf("%s @ %s", addr[0], dnsResolver), nil
	}
	addrs, err := net.LookupHost(domain)
	if err != nil {
		return "", fmt.Errorf("error resolving %s: %w", domain, err)
	}
	return addrs[0] + " @ default", nil
}
