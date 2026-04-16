package check

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogAttrs_Response(t *testing.T) {
	report := &Report{
		protocol: "http",
		response: "OK",
		elapsed:  123 * time.Millisecond,
	}
	attrs := report.LogAttrs()

	assert.Len(t, attrs, 6, "should have 3 key-value pairs (6 items)")
	assert.Equal(t, "protocol", attrs[0])
	assert.Equal(t, "http", attrs[1])
	assert.Equal(t, "elapsed", attrs[2])
	assert.Equal(t, 123*time.Millisecond, attrs[3])
	assert.Equal(t, "response", attrs[4])
	assert.Equal(t, "OK", attrs[5])
}

func TestLogAttrs_Error(t *testing.T) {
	err := errors.New("network error")
	report := &Report{
		protocol: "tcp",
		elapsed:  456 * time.Millisecond,
		error:    err,
	}
	attrs := report.LogAttrs()

	assert.Len(t, attrs, 6, "should have 3 key-value pairs (6 items)")
	assert.Equal(t, "protocol", attrs[0])
	assert.Equal(t, "tcp", attrs[1])
	assert.Equal(t, "elapsed", attrs[2])
	assert.Equal(t, 456*time.Millisecond, attrs[3])
	assert.Equal(t, "error", attrs[4])
	assert.Equal(t, "network error", attrs[5])
}

func TestLogAttrs_Empty(t *testing.T) {
	report := &Report{
		protocol: "udp",
		elapsed:  789 * time.Millisecond,
	}
	attrs := report.LogAttrs()

	assert.Len(t, attrs, 4, "should have 2 key-value pairs (4 items)")
	assert.Equal(t, "protocol", attrs[0])
	assert.Equal(t, "udp", attrs[1])
	assert.Equal(t, "elapsed", attrs[2])
	assert.Equal(t, 789*time.Millisecond, attrs[3])
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
