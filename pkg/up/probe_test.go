// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package up

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

var testProto = &Protocol{
	ID:    "test-proto",
	RHost: func() (string, error) { return "", nil },
}

var testProtoID2 = "test-proto-2"

var testProtoProbe = func(_ *Protocol, _ string, _ time.Duration) (string, error) { return "", nil }

func TestProtocolValidate(t *testing.T) {
	probes[testProto.ID] = testProtoProbe
	t.Run("returns nil with valid setup", func(t *testing.T) {
		err := testProto.validate()
		if err != nil {
			t.Fatalf("got %q, want nil", err)
		}
	})
	t.Run("returns an error if 'Probe' property is nil", func(t *testing.T) {
		p := &Protocol{ID: testProtoID2, RHost: testProto.RHost}
		err := p.validate()
		want := fmt.Sprintf("unknown probe for protocol %s", testProtoID2)
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
	t.Run("returns an error if 'RHost' property is nil", func(t *testing.T) {
		p := &Protocol{ID: testProto.ID}
		err := p.validate()
		want := "required property: RHost"
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
}

func TestProbeValidate(t *testing.T) {
	protocols := []*Protocol{testProto}
	probes[testProto.ID] = testProtoProbe
	t.Run("returns nil with valid setup", func(t *testing.T) {
		reportCh := make(chan *Report)
		defer close(reportCh)
		p := Probe{
			Protocols: protocols,
			Timeout:   1 * time.Second,
			Logger:    slog.Default(),
			ReportCh:  reportCh,
		}
		err := p.validate()
		if err != nil {
			t.Fatalf("got %q, want nil", err)
		}
	})
	t.Run("returns an error if Protocols is nil", func(t *testing.T) {
		p := Probe{}
		err := p.validate()
		want := "required property: Protocols"
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
	t.Run("returns an error if a protocol is invalid", func(t *testing.T) {
		p := Probe{Protocols: []*Protocol{{}}}
		err := p.validate()
		want := "invalid protocol: unknown probe for protocol "
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
	t.Run("returns an error if Timeout is zero", func(t *testing.T) {
		p := Probe{Protocols: protocols}
		err := p.validate()
		want := "required property: Timeout"
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
	t.Run("returns an error if Logger is nil", func(t *testing.T) {
		p := Probe{
			Protocols: protocols,
			Timeout:   1 * time.Second,
		}
		err := p.validate()
		want := "required property: Logger"
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
	t.Run("returns an error if ReportCh is nil", func(t *testing.T) {
		p := Probe{
			Protocols: protocols,
			Timeout:   1 * time.Second,
			Logger:    slog.Default(),
		}
		err := p.validate()
		want := "required property: ReportCh"
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
}

func TestProbeRun(t *testing.T) {
	probes[testProto.ID] = testProtoProbe
	t.Run("returns an error if the setup is invalid", func(t *testing.T) {
		p := Probe{}
		err := p.Run(context.Background())
		want := "invalid setup: required property: Protocols"
		if err.Error() != want {
			t.Fatalf("got %q, want %q", err, want)
		}
	})
	hostPort := "192.168.1.1:22"
	localHostPort := "127.0.0.1:3355"
	proto := &Protocol{
		ID:    "test-proto-2",
		RHost: func() (string, error) { return hostPort, nil },
	}
	probes[proto.ID] = func(_ *Protocol, _ string, _ time.Duration) (string, error) { return localHostPort, nil }
	t.Run("returns nil if 'Count' property is defined", func(t *testing.T) {
		reportCh := make(chan *Report)
		defer close(reportCh)
		p := Probe{
			Protocols: []*Protocol{proto},
			Count:     2,
			Timeout:   1 * time.Second,
			Logger:    slog.Default(),
			ReportCh:  reportCh,
		}
		go func(t *testing.T) {
			for report := range p.ReportCh {
				if report.ProtocolID != proto.ID {
					t.Errorf("got %q, want %q", report.ProtocolID, proto.ID)
				}
				if report.RHost != hostPort {
					t.Errorf("got %q, want %q", report.RHost, hostPort)
				}
				if report.Time == 0 {
					t.Errorf("got %q, want > 0", report.Time)
				}
				if report.Extra != localHostPort {
					t.Errorf("got %q, want %q", report.Extra, localHostPort)
				}
				if report.Error != nil {
					t.Errorf("got %q, want nil", report.Error)
				}
			}
		}(t)
		err := p.Run(context.Background())
		if err != nil {
			t.Fatalf("got %q, want nil", err)
		}
	})
	t.Run("returns nil if context is canceled", func(t *testing.T) {
		reportCh := make(chan *Report)
		defer close(reportCh)
		p := Probe{
			Protocols: []*Protocol{proto},
			Timeout:   1 * time.Second,
			Logger:    slog.Default(),
			ReportCh:  reportCh,
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := p.Run(ctx)
		if err != nil {
			t.Fatalf("got %q, want nil", err)
		}
	})
}
