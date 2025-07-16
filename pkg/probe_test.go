// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	dnsResolver       = "1.1.1.1:53"
	addressForTimeout = "192.0.2.1:53"
	tout              = 1 * time.Second
	toutFail          = 1 * time.Microsecond
)

func checkError(t *testing.T, report *Report) error {
	err := report.error
	assert.NotNil(t, err, "should have error")
	got := report.response
	assert.Equal(t, "", got, "response should be empty when error is set")
	return err
}

func checkTimeout(t *testing.T, report *Report, want string) {
	checkError(t, report)
	got := report.error.Error()
	assert.Contains(t, got, want, "error message should contain expected substring")
}

func TestHttpProbe(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()
	t.Run(
		"returns the status code if the request is successful",
		func(t *testing.T) {
			u := url.URL{Scheme: "http", Host: server.Addr}
			httpProbe := GetHTTPProbe(u.String())
			report := httpProbe.Probe(context.Background(), tout)
			assert.NoError(t, report.error)
			want := "200 OK"
			got := report.response
			assert.Equal(t, want, got)
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		u := url.URL{Scheme: "http", Host: "localhost"}
		httpProbe := GetHTTPProbe(u.String())
		report := httpProbe.Probe(context.Background(), tout)
		err := checkError(t, report)
		got := err.Error()
		prefix := "error making request to http://localhost: Get \"http://localhost\""
		assert.True(t, strings.HasPrefix(got, prefix), "error should have expected prefix")
	})
	t.Run(
		"returns an error if the request times out",
		func(t *testing.T) {
			u := url.URL{Scheme: "http", Host: addressForTimeout}
			httpProbe := GetHTTPProbe(u.String())
			report := httpProbe.Probe(context.Background(), toutFail)
			checkTimeout(t, report, "Client.Timeout")
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

func TestTcpProbe(t *testing.T) {
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
			report := tcpProbe.Probe(context.Background(), tout)
			assert.NoError(t, report.error)
			got := report.response
			fmt.Println("Got: ", got)
			localHost, localPort, err := net.SplitHostPort(got)
			assert.NoError(t, err, "should split host/port")
			assert.Equal(t, "127.0.0.1", localHost)
			assert.True(t, localPort >= "1024" && localPort <= "65535", "port should be valid")
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		tcpProbe := GetTCPProbe("localhost:80")
		report := tcpProbe.Probe(context.Background(), 1)
		assert.NotNil(t, report.error, "should have error")
		got := report.response
		assert.Equal(t, "", got, "response should be empty when error is set")
		gotErr := report.error.Error()
		want := "error making request to localhost:80: dial tcp: lookup localhost: i/o timeout"
		assert.Equal(t, want, gotErr)
	})
	t.Run(
		"returns an error if the request times out",
		func(t *testing.T) {
			tcpProbe := GetTCPProbe(addressForTimeout)
			report := tcpProbe.Probe(context.Background(), toutFail)
			checkTimeout(t, report, "i/o timeout")
		},
	)
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
	t.Run(
		"returns the first resolved IP address if the request is successful",
		func(t *testing.T) {
			dnsProbe := GetDNSProbe(dnsResolver, "google.com")
			report := dnsProbe.Probe(context.Background(), tout)
			assert.NoError(t, report.error)
			got := report.response
			var ip, server string
			// Parse the output string
			_, err := fmt.Sscanf(got, "%s @ %s", &ip, &server)
			assert.NoError(t, err, "should parse ip @ service")
			ipAddr := net.ParseIP(ip)
			assert.NotNil(t, ipAddr, "should be valid IP address")
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		dnsProbe := GetDNSProbe(dnsResolver, "invalid.aa")
		report := dnsProbe.Probe(context.Background(), tout)
		err := checkError(t, report)
		got := err.Error()
		prefix := "error resolving invalid.aa"
		assert.True(t, strings.HasPrefix(got, prefix), "error should have expected prefix")
	})
	t.Run(
		"returns an error if the request times out",
		func(t *testing.T) {
			dnsProbe := GetDNSProbe(addressForTimeout, "google.com")
			report := dnsProbe.Probe(context.Background(), toutFail)
			checkTimeout(t, report, "i/o timeout")
		},
	)
}
