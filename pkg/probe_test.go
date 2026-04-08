package pkg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout     = 1 * time.Second
	testTimeoutFail = 1 * time.Microsecond
)

func checkError(t *testing.T, report *Report) error {
	t.Helper()

	err := report.error
	require.Error(t, err, "should have error")

	got := report.response
	assert.Empty(t, got, "response should be empty when error is set")

	return err
}

func checkTimeout(t *testing.T, report *Report, want string) {
	t.Helper()
	_ = checkError(t, report)
	got := report.error.Error()
	assert.Contains(t, got, want, "error message should contain expected substring")
}
