package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestMainExitCode(t *testing.T) {
	if os.Getenv("BE_TEST_MAIN") == "1" {
		main()

		return
	}
	//nolint:gosec // G204: Test file, os.Args[0] is safe
	cmd := exec.Command(
		os.Args[0],
		"-test.run=TestMainExitCode",
	)

	cmd.Env = append(os.Environ(), "BE_TEST_MAIN=1")

	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit code when no config file exists")
	}
}
