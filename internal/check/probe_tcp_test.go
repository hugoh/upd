package check

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestTcpProbe_Success(t *testing.T) {
	listen := newTestTCPServer(t)

	defer func() {
		if err := listen.Close(); err != nil {
			t.Error(err)
		}
	}()

	go func() {
		connection, err := listen.Accept()
		if err != nil {
			t.Logf("Error accepting connection: %v", err)

			return
		}

		go func(conn net.Conn) {
			if _, err := conn.Write([]byte("pong")); err != nil {
				t.Error(err)
			}

			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		}(connection)
	}()

	hostPort := listen.Addr().String()
	tcpProbe := NewTCPProbe(hostPort)

	report := tcpProbe.Execute(context.Background(), testTimeout)
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
}

func TestTcpProbe_RequestFails(t *testing.T) {
	tcpProbe := NewTCPProbe("localhost:80")

	report := tcpProbe.Execute(context.Background(), 1)
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
}

func TestTcpProbe_Timeout(t *testing.T) {
	tcpProbe := NewTCPProbe("192.0.2.1:53")
	report := tcpProbe.Execute(context.Background(), testTimeoutFail)
	checkTimeout(t, report, "i/o timeout")
}

func newTestTCPServer(t *testing.T) net.Listener {
	t.Helper()

	hostPort := net.JoinHostPort("127.0.0.1", "8081")

	listen, err := net.Listen("tcp", hostPort)
	if err != nil {
		t.Fatalf("starting tcp server: %v", err)
	}

	return listen
}
