// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

var dnsResolver = "1.1.1.1:53"

func TestHttpProbe(t *testing.T) {
	tout := 1 * time.Second
	server := newTestHTTPServer(t)
	defer server.Close()
	t.Run(
		"returns the status code if the request is successful",
		func(t *testing.T) {
			u := url.URL{Scheme: "http", Host: server.Addr}
			httpProbe := GetHTTPProbe(u.String())
			report := httpProbe.Probe(tout)
			if report.Error != nil {
				t.Fatal(report.Error)
			}
			want := "200 OK"
			got := report.Response
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		u := url.URL{Scheme: "http", Host: "localhost"}
		httpProbe := GetHTTPProbe(u.String())
		report := httpProbe.Probe(tout)
		err := report.Error
		if err == nil {
			t.Fatal("got nil, want an error")
		}
		got := report.Response
		if got != "" {
			t.Fatalf("got %q should be zero", got)
		}
		got = err.Error()
		prefix := "error making request to http://localhost: Get \"http://localhost\""
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
}

// Creates an HTTP server for testing.
func newTestHTTPServer(t *testing.T) *http.Server {
	hostPort := net.JoinHostPort("127.0.0.1", "8080")
	server := &http.Server{Addr: hostPort}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "pong ")
	})
	l, err := net.Listen("tcp", hostPort)
	if err != nil {
		t.Fatalf("create listener %v", err)
	}
	go func() {
		err := server.Serve(l)
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("starting http server: %v", err)
		}
	}()
	return server
}

func TestTcpProbe(t *testing.T) {
	tout := 1 * time.Second
	listen := newTestTCPServer(t)
	defer listen.Close()
	go func() {
		connection, err := listen.Accept()
		if err != nil {
			fmt.Println("Error: ", err.Error())
			return
		}
		go func(conn net.Conn) {
			conn.Write([]byte("pong"))
			conn.Close()
		}(connection)
	}()
	hostPort := listen.Addr().String()
	t.Run(
		"returns the local host/port if the request is successful",
		func(t *testing.T) {
			tcpProbe := GetTCPProbe(hostPort)
			report := tcpProbe.Probe(tout)
			if report.Error != nil {
				t.Fatal(report.Error)
			}
			got := report.Response
			fmt.Println("Got: ", got)
			localHost, localPort, err := net.SplitHostPort(got)
			if err != nil {
				t.Fatalf("invalid host/port %s: %v", got, err)
			}
			if localHost != "127.0.0.1" {
				t.Fatalf("got %q, want %q", localHost, "127.0.0.1")
			}
			if localPort < "1024" || localPort > "65535" {
				t.Fatalf("invalid port %s", localPort)
			}
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		tcpProbe := GetTCPProbe("localhost:80")
		report := tcpProbe.Probe(1)
		if report.Error == nil {
			t.Fatal("got nil, want an error")
		}
		got := report.Response
		if got != "" {
			t.Fatalf("got %q should be zero", got)
		}
		got = report.Error.Error()
		want := "error making request to localhost:80: dial tcp: lookup localhost: i/o timeout"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// Creates a TCP server for testing.
func newTestTCPServer(t *testing.T) net.Listener {
	hostPort := net.JoinHostPort("127.0.0.1", "8081")
	listen, err := net.Listen("tcp", hostPort)
	if err != nil {
		t.Fatalf("starting tcp server: %v", err)
	}
	return listen
}

func TestDnsProbe(t *testing.T) {
	tout := 1 * time.Second
	// We need to support custom resolvers first.
	t.Run(
		"returns the first resolved IP address if the request is successful",
		func(t *testing.T) {
			dnsProbe := GetDNSProbe(dnsResolver, "google.com")
			report := dnsProbe.Probe(tout)
			if report.Error != nil {
				t.Fatal(report.Error)
			}
			got := report.Response
			var ip, server string
			// Parse the output string
			_, err := fmt.Sscanf(got, "%s @ %s", &ip, &server)
			if err != nil {
				t.Fatalf("the output is not ip @ service: %s: %v", got, err)
			}
			ipAddr := net.ParseIP(ip)
			if ipAddr == nil {
				t.Fatalf("invalid IP address %s: %v", got, err)
			}
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		dnsProbe := GetDNSProbe(dnsResolver, "invalid.aa")
		report := dnsProbe.Probe(1)
		err := report.Error
		if err == nil {
			t.Fatal("got nil, want an error")
		}
		got := report.Response
		if got != "" {
			t.Fatalf("got %q should be zero", got)
		}
		got = err.Error()
		prefix := "error resolving invalid.aa"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
}
