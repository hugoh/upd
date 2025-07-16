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
