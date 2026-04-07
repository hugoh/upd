package pkg

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogrusFields_Response(t *testing.T) {
	report := &Report{
		protocol: "http",
		response: "OK",
		elapsed:  123 * time.Millisecond,
	}
	fields := report.LogrusFields()
	assert.Equal(t, "http", fields["protocol"], "protocol should be 'http'")
	assert.Equal(t, "OK", fields["response"], "response should be 'OK'")
	assert.Equal(t, 123*time.Millisecond, fields["elapsed"], "elapsed should be 123ms")
	_, ok := fields["error"]
	assert.False(t, ok, "should not have error field")
}

func TestLogrusFields_Error(t *testing.T) {
	err := errors.New("network error")
	report := &Report{
		protocol: "tcp",
		elapsed:  456 * time.Millisecond,
		error:    err,
	}
	fields := report.LogrusFields()
	assert.Equal(t, "tcp", fields["protocol"], "protocol should be 'tcp'")
	assert.Equal(t, "network error", fields["error"], "error should be 'network error'")
	assert.Equal(t, 456*time.Millisecond, fields["elapsed"], "elapsed should be 456ms")
	_, ok := fields["response"]
	assert.False(t, ok, "should not have response field")
}

func TestLogrusFields_Empty(t *testing.T) {
	report := &Report{
		protocol: "udp",
		elapsed:  789 * time.Millisecond,
	}
	fields := report.LogrusFields()
	assert.Equal(t, "udp", fields["protocol"], "protocol should be 'udp'")
	assert.Equal(t, 789*time.Millisecond, fields["elapsed"], "elapsed should be 789ms")
	_, ok := fields["response"]
	assert.False(t, ok, "should not have response field")
	_, ok = fields["error"]
	assert.False(t, ok, "should not have error field")
}

func TestProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
	}{
		{"HTTP", "http"},
		{"HTTPS", "https"},
		{"TCP", "tcp"},
		{"DNS", "dns"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{protocol: tt.protocol}
			assert.Equal(t, tt.protocol, report.Protocol())
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
			assert.Equal(t, tt.response, report.Response())
		})
	}
}

func TestResponse_WithErrors(t *testing.T) {
	err := errors.New("network error")
	report := &Report{error: err}
	resp := report.Response()
	assert.Empty(t, resp, "Response should be empty when there's an error")
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
			assert.Equal(t, tt.elapsed, report.Elapsed())
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
			err := report.Error()
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
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
			assert.Equal(t, tt.isError, report.IsError())
		})
	}
}
