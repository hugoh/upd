package pkg

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpProbe_Success(t *testing.T) {
	server := newTestHTTPServer(t)

	defer func() {
		err := server.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	u := url.URL{Scheme: "http", Host: server.Addr}
	httpProbe := NewHTTPProbe(u.String())

	report := httpProbe.Probe(context.Background(), testTimeout)
	if report.error != nil {
		t.Fatal(report.error)
	}

	want := "200 OK"

	got := report.response
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHttpProbe_UserAgentHeader(t *testing.T) {
	uaCh := make(chan string, 1)
	hostPort := net.JoinHostPort("127.0.0.1", "8081")
	server := &http.Server{
		Addr:              hostPort,
		ReadHeaderTimeout: 1 * time.Second,
	}

	http.HandleFunc("/ua", func(w http.ResponseWriter, r *http.Request) {
		uaCh <- r.Header.Get("User-Agent")

		_, err := io.WriteString(w, "pong")
		if err != nil {
			t.Error(err)
		}
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

	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()

	u := url.URL{Scheme: "http", Host: hostPort, Path: "/ua"}
	httpProbe := NewHTTPProbe(u.String())

	report := httpProbe.Probe(context.Background(), testTimeout)
	if report.error != nil {
		t.Fatal(report.error)
	}

	select {
	case ua := <-uaCh:
		wantUA := "upd/dev"
		if ua != wantUA {
			t.Fatalf("User-Agent header = %q, want %q", ua, wantUA)
		}
	default:
		t.Fatal("User-Agent header not received")
	}
}

func TestHttpProbe_RequestFails(t *testing.T) {
	u := url.URL{Scheme: "http", Host: "localhost"}
	httpProbe := NewHTTPProbe(u.String())
	report := httpProbe.Probe(context.Background(), testTimeout)
	err := checkError(t, report)
	got := err.Error()

	prefix := "error making request to http://localhost: Get \"http://localhost\""
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("got %q, want prefix %q", got, prefix)
	}
}

func TestHttpProbe_Timeout(t *testing.T) {
	u := url.URL{Scheme: "http", Host: "192.0.2.1:53"}
	httpProbe := NewHTTPProbe(u.String())
	report := httpProbe.Probe(context.Background(), testTimeoutFail)
	checkTimeout(t, report, "context deadline exceeded")
}

func newTestHTTPServer(t *testing.T) *http.Server {
	t.Helper()

	hostPort := net.JoinHostPort("127.0.0.1", "8080")
	server := &http.Server{
		Addr:              hostPort,
		ReadHeaderTimeout: 1 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, err := io.WriteString(w, "pong ")
		if err != nil {
			t.Error(err)
		}
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

func TestHTTPProbe_RoundTrip(t *testing.T) {
	uaCh := make(chan string, 1)
	hostPort := net.JoinHostPort("127.0.0.1", "8082")
	server := &http.Server{
		Addr:              hostPort,
		ReadHeaderTimeout: 1 * time.Second,
	}

	http.HandleFunc("/rt", func(w http.ResponseWriter, r *http.Request) {
		uaCh <- r.Header.Get("User-Agent")

		_, err := io.WriteString(w, "ok")
		if err != nil {
			t.Error(err)
		}
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

	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()

	trans := &updTransport{version: "test"}
	req := httptest.NewRequest(http.MethodGet, "http://"+hostPort+"/rt", nil)
	resp, err := trans.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	select {
	case ua := <-uaCh:
		assert.Equal(t, "upd/test", ua)
	default:
		t.Fatal("User-Agent header not received")
	}
}

func TestHTTPProbe_RoundTrip_NetworkFailure(t *testing.T) {
	trans := &updTransport{version: "test"}
	req := httptest.NewRequest(http.MethodGet, "http://192.0.2.1:9999/test", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req = req.WithContext(ctx)

	resp, err := trans.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestHTTPProbe_ProbeWithTimeout(t *testing.T) {
	httpProbe := &HTTPProbe{URL: "://invalid", client: http.DefaultClient}
	report := httpProbe.Probe(context.Background(), time.Second)
	require.Error(t, report.error)
	assert.Contains(t, report.error.Error(), "error building request")
}

func TestHTTPProbe_Scheme(t *testing.T) {
	probe := NewHTTPProbe("http://example.com")
	assert.Equal(t, "http", probe.Scheme())
}
