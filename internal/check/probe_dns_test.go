package check

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
)

type fakeResolver struct {
	result []string
	err    error
}

func (r *fakeResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	return r.result, r.err
}

func TestDnsProbe(t *testing.T) {
	t.Run("returns the first resolved IP address if the request is successful", func(t *testing.T) {
		resolver := &fakeResolver{result: []string{"1.2.3.4"}}
		dnsProbe := &DNSProbe{Domain: "example.com", resolver: resolver}

		report := dnsProbe.Execute(context.Background(), testTimeout)
		if report.error != nil {
			t.Fatal(report.error)
		}

		got := report.response

		var ip string

		_, err := fmt.Sscanf(got, "%s @", &ip)
		if err != nil {
			t.Fatalf("the output is not ip @ service: %s: %v", got, err)
		}

		ipAddr := net.ParseIP(ip)
		if ipAddr == nil {
			t.Fatalf("invalid IP address %s: %v", got, err)
		}
	})
	t.Run("returns an error if the request fails", func(t *testing.T) {
		resolver := &fakeResolver{err: errors.New("no such host")}
		dnsProbe := &DNSProbe{Domain: "invalid.aa", resolver: resolver}

		report := dnsProbe.Execute(context.Background(), testTimeout)
		err := checkError(t, report)
		got := err.Error()

		prefix := "error resolving invalid.aa"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
	t.Run("returns an error if the request times out", func(t *testing.T) {
		resolver := &fakeResolver{err: context.DeadlineExceeded}
		dnsProbe := &DNSProbe{Domain: "example.com", resolver: resolver}

		report := dnsProbe.Execute(context.Background(), testTimeout)
		checkTimeout(t, report, "context deadline exceeded")
	})
}
