//go:build integration

package internal_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionIsSetByLdflags(t *testing.T) {
	// This test builds the binary with ldflags to ensure the version is correctly injected,
	// mimicking the behavior of GoReleaser during a release.

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "upd")
	testVersion := "v1.2.3-test"

	// Your .goreleaser.yaml uses the shorthand `-X pkg.version`. For a direct `go build`
	// command, we should use the full package path for the variable, which is more robust.
	// The module path `github.com/hugoh/upd` is inferred from your import paths.
	ldflags := fmt.Sprintf("-X github.com/hugoh/upd/pkg.version=%s", testVersion)

	// Build the main package with the ldflags.
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "-ldflags", ldflags, "github.com/hugoh/upd")
	err := buildCmd.Run()
	require.NoError(t, err, "failed to build binary for testing")

	// Run the compiled binary with the --version flag and capture its output.
	versionCmd := exec.Command(binaryPath, "--version")
	output, err := versionCmd.CombinedOutput()
	require.NoError(t, err, "failed to run binary to get version")

	// Verify that the output matches the version we injected.
	expectedOutput := fmt.Sprintf("upd version %s", testVersion)
	require.Equal(t, expectedOutput, strings.TrimSpace(string(output)), "version output is not correct")
}
