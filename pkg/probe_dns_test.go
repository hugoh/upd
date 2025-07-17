package pkg

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
)

var (
	dnsResolver       = "1.1.1.1:53"
	addressForTimeout = "192.0.2.1:53"
)

func TestDnsProbe(t *testing.T) {
	t.Run(
		"returns the first resolved IP address if the request is successful",
		func(t *testing.T) {
			dnsProbe := NewDNSProbe(dnsResolver, "google.com")
			report := dnsProbe.Probe(context.Background(), tout)
			if report.error != nil {
				t.Fatal(report.error)
			}
			got := report.response
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
		dnsProbe := NewDNSProbe(dnsResolver, "invalid.aa")
		report := dnsProbe.Probe(context.Background(), tout)
		err := checkError(t, report)
		got := err.Error()
		prefix := "error resolving invalid.aa"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got %q, want prefix %q", got, prefix)
		}
	})
	t.Run(
		"returns an error if the request times out",
		func(t *testing.T) {
			dnsProbe := NewDNSProbe(addressForTimeout, "google.com")
			report := dnsProbe.Probe(context.Background(), toutFail)
			checkTimeout(t, report, "i/o timeout")
		},
	)
}
