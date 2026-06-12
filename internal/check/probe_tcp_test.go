package check

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestTcpProbe_Target(t *testing.T) {
	probe := NewTCPProbe("192.168.1.1:8080")
	assert.Equal(t, "192.168.1.1:8080", probe.Target())
}

func TestTcpProbe_Success(t *testing.T) {
	wantLocalAddr := &fakeTCPAddr{addr: "127.0.0.1:54321"}
	_, clientEnd := net.Pipe()
	conn := &stubTCPConn{Conn: clientEnd, localAddr: wantLocalAddr}

	dialer := &fakeDialer{conn: conn}
	tcpProbe := &TCPProbe{HostPort: "127.0.0.1:12345", dialer: dialer}

	report := tcpProbe.Execute(t.Context(), testTimeout)
	require.NoError(t, report.error)

	got := report.response

	localHost, localPort, err := net.SplitHostPort(got)
	require.NoError(t, err, "invalid host/port %s", got)

	assert.Equal(t, "127.0.0.1", localHost)
	assert.True(t, localPort >= "1024" && localPort <= "65535", "invalid port %s", localPort)
}

func TestTcpProbe_RequestFails(t *testing.T) {
	dialer := &fakeDialer{err: errors.New("connection refused")}
	tcpProbe := &TCPProbe{HostPort: "localhost:80", dialer: dialer}

	report := tcpProbe.Execute(t.Context(), 1)
	require.Error(t, report.error)

	got := report.response
	assert.Empty(t, got)

	got = report.error.Error()
	assert.Equal(t, "error making request to localhost:80: connection refused", got)
}

func TestTcpProbe_Timeout(t *testing.T) {
	dialer := &fakeDialer{err: context.DeadlineExceeded}
	tcpProbe := &TCPProbe{HostPort: "192.0.2.1:53", dialer: dialer}

	report := tcpProbe.Execute(t.Context(), testTimeout)
	checkTimeout(t, report, "context deadline exceeded")
}
