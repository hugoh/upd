package pkg

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestTcpProbe(t *testing.T) {
	listen := newTestTCPServer(t)
	defer listen.Close()
	go func() {
		connection, err := listen.Accept()
		if err != nil {
			t.Logf("Error accepting connection: %v", err)
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
			tcpProbe := NewTCPProbe(hostPort)
			report := tcpProbe.Probe(context.Background(), testTimeout)
			if report.error != nil {
				t.Fatal(report.error)
			}
			got := report.response
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
		tcpProbe := NewTCPProbe("localhost:80")
		report := tcpProbe.Probe(context.Background(), 1)
		if report.error == nil {
			t.Fatal("got nil, want an error")
		}
		got := report.response
		if got != "" {
			t.Fatalf("got %q should be zero", got)
		}
		got = report.error.Error()
		want := "error making request to localhost:80: dial tcp: lookup localhost:"
		if !strings.Contains(got, want) {
			t.Fatalf("got %q, want to contain %q", got, want)
		}
	})
	t.Run(
		"returns an error if the request times out",
		func(t *testing.T) {
			tcpProbe := NewTCPProbe("192.0.2.1:53")
			report := tcpProbe.Probe(context.Background(), testTimeoutFail)
			checkTimeout(t, report, "i/o timeout")
		},
	)
}

func newTestTCPServer(t *testing.T) net.Listener {
	hostPort := net.JoinHostPort("127.0.0.1", "8081")
	listen, err := net.Listen("tcp", hostPort)
	if err != nil {
		t.Fatalf("starting tcp server: %v", err)
	}
	return listen
}
