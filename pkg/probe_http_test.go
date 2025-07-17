package pkg

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestHttpProbe(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()
	t.Run(
		"returns the status code if the request is successful",
		func(t *testing.T) {
			u := url.URL{Scheme: "http", Host: server.Addr}
			httpProbe := NewHTTPProbe("test-version").WithURL(u.String())
			report := httpProbe.Probe(context.Background(), tout)
			if report.error != nil {
				t.Fatal(report.error)
			}
			want := "200 OK"
			got := report.response
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		},
	)

	t.Run("sets the correct User-Agent header", func(t *testing.T) {
		uaCh := make(chan string, 1)
		hostPort := net.JoinHostPort("127.0.0.1", "8081")
		server := &http.Server{Addr: hostPort}
		http.HandleFunc("/ua", func(w http.ResponseWriter, r *http.Request) {
			uaCh <- r.Header.Get("User-Agent")
			io.WriteString(w, "pong")
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
		defer server.Close()
		u := url.URL{Scheme: "http", Host: hostPort, Path: "/ua"}
		httpProbe := NewHTTPProbe("test-version").WithURL(u.String())
		report := httpProbe.Probe(context.Background(), tout)
		if report.error != nil {
			t.Fatal(report.error)
		}
		select {
		case ua := <-uaCh:
			wantUA := "upd/test-version"
			if ua != wantUA {
				t.Fatalf("User-Agent header = %q, want %q", ua, wantUA)
			}
		default:
			t.Fatal("User-Agent header not received")
		}
	})
	t.Run("returns an error if the request fails", func(t *testing.T) {
		u := url.URL{Scheme: "http", Host: "localhost"}
		httpProbe := NewHTTPProbe("test-version").WithURL(u.String())
		report := httpProbe.Probe(context.Background(), tout)
		err := checkError(t, report)
		got := err.Error()
		prefix := "error making request to http://localhost: Get \"http://localhost\""
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
	t.Run(
		"returns an error if the request times out",
		func(t *testing.T) {
			u := url.URL{Scheme: "http", Host: "192.0.2.1:53"}
			httpProbe := NewHTTPProbe("test-version").WithURL(u.String())
			report := httpProbe.Probe(context.Background(), toutFail)
			checkTimeout(t, report, "context deadline exceeded")
		},
	)
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
