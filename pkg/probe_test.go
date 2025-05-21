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
	"net/http/httptest" // Import httptest
	"net/url"
	"strings"
	"testing"
	"time"
)

var (
	dnsResolver = "1.1.1.1:53"
	tout        = 1 * time.Second
	toutFail    = 1 * time.Microsecond
)

func checkError(t *testing.T, report *Report) error {
	t.Helper()
	err := report.Error
	if err == nil {
		t.Fatal("got nil, want an error")
	}
	got := report.Response
	if got != "" {
		t.Fatalf("got %q should be zero", got)
	}
	return err
}

func checkTimeout(t *testing.T, report *Report, want string) {
	t.Helper()
	checkError(t, report)
	got := report.Error.Error()
	if !strings.Contains(got, want) {
		t.Fatalf("got %q, missing %q", got, want)
	}
}

func TestHttpProbe(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		expectedStatus string
		expectError    bool
		errorContains  string
		timeout        time.Duration
	}{
		{
			name:           "200 OK",
			path:           "/ok",
			expectedStatus: "200 OK",
			timeout:        tout,
		},
		{
			name:           "Root path (default 200 OK)", // For backward compatibility if any test hits root
			path:           "/",
			expectedStatus: "200 OK",
			timeout:        tout,
		},
		{
			name:           "404 Not Found",
			path:           "/notfound",
			expectedStatus: "404 Not Found",
			timeout:        tout,
		},
		{
			name:           "500 Internal Server Error",
			path:           "/servererror",
			expectedStatus: "500 Internal Server Error",
			timeout:        tout,
		},
		{
			name:           "302 Found (Redirect)",
			path:           "/redirect",
			expectedStatus: "200 OK", // Assuming client follows redirect to /ok
			timeout:        tout,
		},
		{
			name:          "Request fails (non-existent server)",
			path:          "", // Host will be different
			expectError:   true,
			errorContains: "error making request", // Generic part of the error
			timeout:       tout,
			// Special handling for URL in test logic
		},
		{
			name:          "Request times out",
			path:          "/ok", // Any valid path on the server
			expectError:   true,
			errorContains: "Client.Timeout", // Error string for context deadline exceeded
			timeout:       toutFail,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var targetURL string
			if tc.name == "Request fails (non-existent server)" {
				// Use a URL that is expected to fail connection.
				badURL := url.URL{Scheme: "http", Host: "localhost:12345"} // Non-existent port
				targetURL = badURL.String()
			} else {
				targetURL = server.URL + tc.path
			}

			httpProbe := GetHTTPProbe(targetURL)
			report := httpProbe.Probe(context.Background(), tc.timeout)

			if tc.expectError {
				if report.Error == nil {
					t.Fatalf("expected an error, but got nil. Response: %s", report.Response)
				}
				if tc.errorContains != "" && !strings.Contains(report.Error.Error(), tc.errorContains) {
					t.Fatalf("expected error to contain %q, but got %q", tc.errorContains, report.Error.Error())
				}
			} else {
				if report.Error != nil {
					t.Fatalf("expected no error, but got: %v", report.Error)
				}
				if report.Response != tc.expectedStatus {
					t.Fatalf("expected status %q, but got %q", tc.expectedStatus, report.Response)
				}
			}
		})
	}
}

// Creates an HTTP server for testing using httptest.
func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Default handler for root, can be 200 or a specific test page if needed
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Root OK")
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK response")
	})
	mux.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "Not Found response")
	})
	mux.HandleFunc("/servererror", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Server Error response")
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusFound) // 302
	})

	server := httptest.NewServer(mux)
	return server
}

func TestTcpProbe(t *testing.T) {
	listen := newTestTCPServer(t)
	defer listen.Close()
	go func() {
		connection, err := listen.Accept()
		if err != nil {
			// Using t.Logf or similar as t.Error/Fatal from non-test goroutine can be problematic
			t.Logf("TCP Server Accept error: %v", err)
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			conn.Write([]byte("pong"))
		}(connection)
	}()
	hostPort := listen.Addr().String()
	t.Run(
		"returns the local host/port if the request is successful",
		func(t *testing.T) {
			tcpProbe := GetTCPProbe(hostPort)
			report := tcpProbe.Probe(context.Background(), tout)
			if report.Error != nil {
				t.Fatal(report.Error)
			}
			got := report.Response
			// fmt.Println("Got: ", got) // Keep for debugging if needed.
			localHost, localPort, err := net.SplitHostPort(got)
			if err != nil {
				t.Fatalf("invalid host/port %s: %v", got, err)
			}
			if localHost != "127.0.0.1" { // Assuming tests run on localhost
				// Check if localHost is any loopback IP, e.g. for IPv6 "::1"
				ip := net.ParseIP(localHost)
				if ip == nil || !ip.IsLoopback() {
					t.Fatalf("got host %q, want a loopback address (e.g. 127.0.0.1 or ::1)", localHost)
				}
			}
			if localPort < "1024" || localPort > "65535" { // Basic port range check
				t.Fatalf("invalid port %s", localPort)
			}
		},
	)
	t.Run("returns an error if the request fails", func(t *testing.T) {
		tcpProbe := GetTCPProbe("localhost:12345") // Use a port that's likely not listened on
		report := tcpProbe.Probe(context.Background(), 20*time.Millisecond) // Short timeout
		err := checkError(t, report)
		got := err.Error()
		// Error message can vary ("connection refused", "i/o timeout" if firewall drops)
		// Just check for the prefix.
		prefix := fmt.Sprintf("error making request to %s:", "localhost:12345")
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
	t.Run(
		"returns an error if the request is times out",
		func(t *testing.T) {
			// To reliably test timeout, we'd need a server that accepts connection but doesn't respond.
			// The current newTestTCPServer responds immediately.
			// Instead, we can try to connect to a non-routable IP or a known blackhole.
			// For simplicity, using a very short timeout against the responsive test server
			// might sometimes work if the network stack is slow, but it's not ideal.
			// A better way: dial a known non-responsive port on localhost.
			tcpProbe := GetTCPProbe("localhost:12346") // Another unlikely port
			report := tcpProbe.Probe(context.Background(), toutFail) // Extremely short timeout
			checkTimeout(t, report, "i/o timeout") // Error might be "connection refused" before timeout.
		},
	)
}

// Creates a TCP server for testing.
func newTestTCPServer(t *testing.T) net.Listener {
	t.Helper()
	// OS will choose an available ephemeral port.
	listen, err := net.Listen("tcp", "127.0.0.1:0")
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
			if report.Error != nil {
				t.Fatal(report.Error)
			}
			got := report.Response
			var ip, server string
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
		dnsProbe := GetDNSProbe(dnsResolver, "invalid.domain.that.does.not.exist.local")
		report := dnsProbe.Probe(context.Background(), tout)
		err := checkError(t, report)
		got := err.Error()
		prefix := "error resolving invalid.domain.that.does.not.exist.local"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
	t.Run(
		"returns an error if the request is times out",
		func(t *testing.T) {
			dnsProbe := GetDNSProbe("8.8.8.8:53", "google.com") // Use a public resolver
			report := dnsProbe.Probe(context.Background(), toutFail) // Extremely short timeout
			checkTimeout(t, report, "i/o timeout")
		},
	)
}
