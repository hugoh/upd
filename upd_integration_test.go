//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func buildAndStartUpd(t *testing.T, configContent string) *exec.Cmd {
	t.Helper()

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "upd")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	err := buildCmd.Run()
	require.NoError(t, err, "failed to build binary")

	configPath := filepath.Join(tmpDir, "upd.toml")
	err = os.WriteFile(configPath, []byte(configContent), 0o600)
	require.NoError(t, err)

	ctx := t.Context()
	cmd := exec.CommandContext(ctx, binaryPath, "-c", configPath)
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	require.NoError(t, err, "failed to start upd")

	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	})

	return cmd
}

func waitForUpd(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()

			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("upd did not become ready within %v at %s", timeout, url)
}

// TestEndToEnd builds the binary, starts it with a minimal config, and
// verifies the binary starts and responds on the stats endpoint.
func TestEndToEnd(t *testing.T) {
	configContent := strings.TrimSpace(`
[checks]
timeout = "10s"

[checks.every]
normal = "30s"
down = "10s"

[checks.list]
ordered = ["http://captive.apple.com/hotspot-detect.html"]

[stats]
port = 18765
`)
	buildAndStartUpd(t, configContent)
	waitForUpd(t, "http://127.0.0.1:18765/stats.json", 5*time.Second)
}

// TestEndToEnd_StatsServer builds, starts upd with stats on a known port,
// then queries the /stats endpoint.
func TestEndToEnd_StatsServer(t *testing.T) {
	port := "19789"
	configContent := fmt.Sprintf(strings.TrimSpace(`
[checks]
timeout = "10s"

[checks.every]
normal = "2s"
down = "1s"

[checks.list]
ordered = ["http://captive.apple.com/hotspot-detect.html"]

[stats]
port = %s
reports = ["1m"]
retention = "1h"
`), port)
	buildAndStartUpd(t, configContent)

	url := fmt.Sprintf("http://127.0.0.1:%s/stats.json", port)
	waitForUpd(t, url, 5*time.Second)

	resp, err := http.Get(url)
	require.NoError(t, err, "failed to query stats endpoint")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var raw map[string]any

	err = json.NewDecoder(resp.Body).Decode(&raw)
	require.NoError(t, err, "response should be valid JSON")

	require.Contains(t, raw, "isUp")
	require.Contains(t, raw, "updVersion")
	require.Contains(t, raw, "reports")
}
