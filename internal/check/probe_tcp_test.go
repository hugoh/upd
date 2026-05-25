package check

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeDialer struct {
	conn net.Conn
	err  error
}

func (d *fakeDialer) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	return d.conn, d.err
}

type fakeTCPAddr struct{ addr string }

func (*fakeTCPAddr) Network() string  { return "tcp" }
func (a *fakeTCPAddr) String() string { return a.addr }

type stubTCPConn struct {
	net.Conn

	localAddr net.Addr
}

func (c *stubTCPConn) LocalAddr() net.Addr { return c.localAddr }

func TestTcpProbe_Success(t *testing.T) {
	wantLocalAddr := &fakeTCPAddr{addr: "127.0.0.1:54321"}
	_, clientEnd := net.Pipe()
	conn := &stubTCPConn{Conn: clientEnd, localAddr: wantLocalAddr}

	dialer := &fakeDialer{conn: conn}
	tcpProbe := &TCPProbe{HostPort: "127.0.0.1:12345", dialer: dialer}

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
	dialer := &fakeDialer{err: errors.New("connection refused")}
	tcpProbe := &TCPProbe{HostPort: "localhost:80", dialer: dialer}

	report := tcpProbe.Execute(context.Background(), 1)
	if report.error == nil {
		t.Fatal("got nil, want an error")
	}

	got := report.response
	if got != "" {
		t.Fatalf("got %q should be zero", got)
	}

	got = report.error.Error()

	want := "error making request to localhost:80: connection refused"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTcpProbe_Timeout(t *testing.T) {
	dialer := &fakeDialer{err: context.DeadlineExceeded}
	tcpProbe := &TCPProbe{HostPort: "192.0.2.1:53", dialer: dialer}

	report := tcpProbe.Execute(context.Background(), testTimeout)
	checkTimeout(t, report, "context deadline exceeded")
}
