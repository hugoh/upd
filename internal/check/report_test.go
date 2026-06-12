package check

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogAttrs_Response(t *testing.T) {
	report := &Report{
		protocol: HTTP,
		target:   "http://example.com",
		response: "OK",
		elapsed:  123 * time.Millisecond,
	}
	attr := report.LogAttrs()

	assert.Equal(t, "report", attr.Key)
	group := attr.Value.Group()
	assert.Len(t, group, 4)
	assert.Equal(t, "protocol", group[0].Key)
	assert.Equal(t, "http", group[0].Value.String())
	assert.Equal(t, "target", group[1].Key)
	assert.Equal(t, "http://example.com", group[1].Value.String())
	assert.Equal(t, "elapsed", group[2].Key)
	assert.Equal(t, 123*time.Millisecond, group[2].Value.Duration())
	assert.Equal(t, "response", group[3].Key)
	assert.Equal(t, "OK", group[3].Value.String())
}

func TestLogAttrs_Error(t *testing.T) {
	err := errors.New("network error")
	report := &Report{
		protocol: "tcp",
		target:   "127.0.0.1:80",
		elapsed:  456 * time.Millisecond,
		error:    err,
	}
	attr := report.LogAttrs()

	assert.Equal(t, "report", attr.Key)
	group := attr.Value.Group()
	assert.Len(t, group, 4)
	assert.Equal(t, "protocol", group[0].Key)
	assert.Equal(t, "tcp", group[0].Value.String())
	assert.Equal(t, "target", group[1].Key)
	assert.Equal(t, "127.0.0.1:80", group[1].Value.String())
	assert.Equal(t, "elapsed", group[2].Key)
	assert.Equal(t, 456*time.Millisecond, group[2].Value.Duration())
	assert.Equal(t, "error", group[3].Key)
	assert.Equal(t, err, group[3].Value.Any())
}

func TestLogAttrs_Empty(t *testing.T) {
	report := &Report{
		protocol: "udp",
		target:   "192.168.1.1:53",
		elapsed:  789 * time.Millisecond,
	}
	attr := report.LogAttrs()

	assert.Equal(t, "report", attr.Key)
	group := attr.Value.Group()
	assert.Len(t, group, 3)
	assert.Equal(t, "protocol", group[0].Key)
	assert.Equal(t, "udp", group[0].Value.String())
	assert.Equal(t, "target", group[1].Key)
	assert.Equal(t, "192.168.1.1:53", group[1].Value.String())
	assert.Equal(t, "elapsed", group[2].Key)
	assert.Equal(t, 789*time.Millisecond, group[2].Value.Duration())
}

func TestProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
	}{
		{"HTTP", HTTP},
		{"HTTPS", HTTPS},
		{"TCP", TCP},
		{"DNS", DNS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{protocol: tt.protocol}
			assert.Equal(t, tt.protocol, report.protocol)
		})
	}
}

func TestResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{"HTTP Status", "200 OK"},
		{"TCP Address", "192.168.1.1:12345"},
		{"DNS IP", "8.8.8.8"},
		{"Empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{response: tt.response}
			assert.Equal(t, tt.response, report.response)
		})
	}
}

func TestResponse_WithErrors(t *testing.T) {
	err := errors.New("network error")
	report := &Report{error: err}
	assert.Empty(t, report.response, "Response should be empty when there's an error")
}

func TestElapsed(t *testing.T) {
	tests := []struct {
		name    string
		elapsed time.Duration
	}{
		{"Milliseconds", 123 * time.Millisecond},
		{"Seconds", 5 * time.Second},
		{"Zero", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{elapsed: tt.elapsed}
			assert.Equal(t, tt.elapsed, report.elapsed)
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"With Error", errors.New("network error"), true},
		{"No Error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{error: tt.err}
			assert.Equal(t, tt.err, report.error)
			assert.Equal(t, tt.wantErr, report.error != nil)
		})
	}
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

func TestIsError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		isError bool
	}{
		{"With Error", errors.New("network error"), true},
		{"With Wrapped Error", errors.New("wrapped: " + "network error"), true},
		{"No Error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{error: tt.err}
			assert.Equal(t, tt.isError, report.error != nil)
		})
	}
}
