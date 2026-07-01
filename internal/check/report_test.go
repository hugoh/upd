package check

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//nolint:funlen // table-driven test with inline struct/closure literals
func TestLogAttrs(t *testing.T) {
	netErr := errors.New("network error")

	tests := []struct {
		name     string
		report   *Report
		wantLen  int
		extraKey string
		checkFn  func(t *testing.T, v slog.Value)
	}{
		{
			name: "response",
			report: &Report{
				protocol: HTTP,
				target:   "http://example.com",
				response: "OK",
				elapsed:  123 * time.Millisecond,
			},
			wantLen:  4,
			extraKey: "response",
			checkFn: func(t *testing.T, v slog.Value) {
				t.Helper()
				assert.Equal(t, "OK", v.String())
			},
		},
		{
			name: "error",
			report: &Report{
				protocol: "tcp",
				target:   "127.0.0.1:80",
				elapsed:  456 * time.Millisecond,
				error:    netErr,
			},
			wantLen:  4,
			extraKey: "error",
			checkFn: func(t *testing.T, v slog.Value) {
				t.Helper()
				assert.Equal(t, netErr, v.Any())
			},
		},
		{
			name: "neither response nor error",
			report: &Report{
				protocol: "udp",
				target:   "192.168.1.1:53",
				elapsed:  789 * time.Millisecond,
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := tt.report.LogAttrs()

			assert.Equal(t, "report", attr.Key)
			group := attr.Value.Group()
			assert.Len(t, group, tt.wantLen)
			assert.Equal(t, "protocol", group[0].Key)
			assert.Equal(t, tt.report.protocol, group[0].Value.String())
			assert.Equal(t, "target", group[1].Key)
			assert.Equal(t, tt.report.target, group[1].Value.String())
			assert.Equal(t, "elapsed", group[2].Key)
			assert.Equal(t, tt.report.elapsed, group[2].Value.Duration())

			if tt.checkFn != nil {
				assert.Equal(t, tt.extraKey, group[3].Key)
				tt.checkFn(t, group[3].Value)
			}
		})
	}
}

func TestResponse_WithErrors(t *testing.T) {
	err := errors.New("network error")
	report := &Report{error: err}
	assert.Empty(t, report.response, "Response should be empty when there's an error")
}

type targetProbe struct {
	Probe

	target string
}

func (p *targetProbe) Target() string { return p.target }
func (*targetProbe) Scheme() string   { return "chk" }

func TestBuildReport_Target(t *testing.T) {
	probe := &targetProbe{target: "example.com:443"}
	report := BuildReport(probe, time.Now())
	assert.Equal(t, "example.com:443", report.target)
}
