package config

import (
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
	path := writeTestConfig(t, `[checks]
timeout = "0s"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_orderedInvalidURI(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/", "://invalid"]`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_shuffledInvalidURI(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
shuffled = ["http://example.com/", "://invalid"]`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_afterNegative(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[downAction]
exec = "echo"

[downAction.every]
after = "-5s"
repeat = "300s"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_repeatNegative(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[downAction]
exec = "echo"

[downAction.every]
after = "60s"
repeat = "-5s"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_backoffLimitNegative(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[downAction]
exec = "echo"

[downAction.every]
after = "60s"
repeat = "300s"
expBackoffLimit = "-5s"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_portTooHigh(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[downAction]
exec = "echo"

[downAction.every]
after = "60s"
repeat = "300s"

[stats]
port = 99999`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_readTimeoutNegative(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[stats]
port = 8080
readTimeout = "-5s"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_writeTimeoutNegative(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[stats]
port = 8080
writeTimeout = "-5s"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_idleTimeoutNegative(t *testing.T) {
	path := writeTestConfig(t, `[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]

[stats]
port = 8080
idleTimeout = "-5s"`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}

func TestValidate_logLevelInvalid(t *testing.T) {
	path := writeTestConfig(t, `logLevel = "invalid"

[checks]
timeout = "2000ms"

[checks.every]
normal = "120s"
down = "20s"

[checks.list]
ordered = ["http://example.com/"]`)

	_, err := ReadConf(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required attributes")
}
