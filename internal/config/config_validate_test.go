package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "upd.toml")

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)

	return path
}

// checksConfig builds a [checks] block with the given timeout and
// [checks.list] body (e.g. `ordered = [...]` or `shuffled = [...]`).
func checksConfig(timeout, listBody string) string {
	return fmt.Sprintf(`[checks]
timeout = %q

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
%s`, timeout, listBody)
}

func validConfigBase() string {
	return checksConfig("2000ms", `ordered = ["http://example.com/"]`)
}

func TestValidate_missingChecks(t *testing.T) {
	path := writeTestConfig(t, `logLevel = "debug"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_normalZero(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "0s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_downZero(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "0s"

[checks.list]
ordered = ["http://example.com/"]`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_timeoutZero(t *testing.T) {
	path := writeTestConfig(t, checksConfig("0s", `ordered = ["http://example.com/"]`))

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_orderedInvalidURI(t *testing.T) {
	path := writeTestConfig(
		t,
		checksConfig("2000ms", `ordered = ["http://example.com/", "://invalid"]`),
	)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_shuffledInvalidURI(t *testing.T) {
	path := writeTestConfig(
		t,
		checksConfig("2000ms", `shuffled = ["http://example.com/", "://invalid"]`),
	)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_tcpMissingPort(t *testing.T) {
	path := writeTestConfig(t, checksConfig("2000ms", `ordered = ["tcp://example.com"]`))

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_dnsMissingDomain(t *testing.T) {
	path := writeTestConfig(t, checksConfig("2000ms", `ordered = ["dns://8.8.8.8"]`))

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_downActionMissingExec(t *testing.T) {
	config := validConfigBase() + `

[downAction.every]
after = "60s"
repeat = "300s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
	assert.Contains(t, err.Error(), "downAction: exec: required when downAction is configured")
}

func TestValidate_afterNegative(t *testing.T) {
	config := validConfigBase() + `

[downAction]
exec = "echo"

[downAction.every]
after = "-5s"
repeat = "300s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_repeatNegative(t *testing.T) {
	config := validConfigBase() + `

[downAction]
exec = "echo"

[downAction.every]
after = "60s"
repeat = "-5s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_backoffLimitNegative(t *testing.T) {
	config := validConfigBase() + `

[downAction]
exec = "echo"

[downAction.every]
after = "60s"
repeat = "300s"
expBackoffLimit = "-5s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_portTooHigh(t *testing.T) {
	config := validConfigBase() + `

[downAction]
exec = "echo"

[downAction.every]
after = "60s"
repeat = "300s"

[stats]
port = 99999`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_readTimeoutNegative(t *testing.T) {
	config := validConfigBase() + `

[stats]
port = 8080
readTimeout = "-5s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_writeTimeoutNegative(t *testing.T) {
	config := validConfigBase() + `

[stats]
port = 8080
writeTimeout = "-5s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_idleTimeoutNegative(t *testing.T) {
	config := validConfigBase() + `

[stats]
port = 8080
idleTimeout = "-5s"`
	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_logLevelInvalid(t *testing.T) {
	path := writeTestConfig(t, `logLevel = "invalid"

`+validConfigBase())

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_bucketsValid(t *testing.T) {
	config := validConfigBase() + `

[stats]
reports = ["1m", "168h"]

[stats.buckets]
min = 50
maxSpan = "30m"`

	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.NoError(t, err)
}

func TestValidate_bucketsMinNegative(t *testing.T) {
	config := validConfigBase() + `

[stats.buckets]
min = -1`

	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "buckets.min")
}

func TestValidate_bucketsMaxSpanNegative(t *testing.T) {
	config := validConfigBase() + `

[stats.buckets]
maxSpan = "-5s"`

	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "buckets.maxSpan")
}

func TestValidate_reportTooManyBuckets(t *testing.T) {
	config := validConfigBase() + `

[stats]
reports = ["168h"]

[stats.buckets]
maxSpan = "1s"`

	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many buckets")
}

func TestValidate_reportNotPositive(t *testing.T) {
	config := validConfigBase() + `

[stats]
reports = ["1m", "0s"]`

	path := writeTestConfig(t, config)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reports")
}
