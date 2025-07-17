package pkg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	tout     = 1 * time.Second
	toutFail = 1 * time.Microsecond
)

func checkError(t *testing.T, report *Report) error {
	err := report.error
	assert.NotNil(t, err, "should have error")
	got := report.response
	assert.Equal(t, "", got, "response should be empty when error is set")
	return err
}

func checkTimeout(t *testing.T, report *Report, want string) {
	checkError(t, report)
	got := report.error.Error()
	assert.Contains(t, got, want, "error message should contain expected substring")
}
