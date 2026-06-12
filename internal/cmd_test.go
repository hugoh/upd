package internal

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/logic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// Must match internal/config/config_test.go.
const testConfigDir = "../testdata"

func TestRun_NoMultipleRestartsOnSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  ConfigConfig,
				Value: testConfigDir + "/upd_test_minimal.toml",
			},
			&cli.BoolFlag{
				Name: ConfigDebug,
			},
		},
	}

	startTime := time.Now()
	_ = Run(ctx, cmd)
	duration := time.Since(startTime)

	assert.Less(t, duration, 2*time.Second, "should not have restarted multiple times")
}

func TestRun_WaitsForWorkerCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  ConfigConfig,
				Value: testConfigDir + "/upd_test_minimal.toml",
			},
			&cli.BoolFlag{
				Name: ConfigDebug,
			},
		},
	}

	done := make(chan struct{})

	go func() {
		err := Run(ctx, cmd)
		assert.NoError(t, err)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)

	cancel()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestRun_SighupReloadsConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "upd.toml")
	good, err := os.ReadFile(testConfigDir + "/upd_test_minimal.toml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, good, 0o600)) // #nosec G703 -- path under t.TempDir()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  ConfigConfig,
				Value: cfgPath,
			},
			&cli.BoolFlag{
				Name: ConfigDebug,
			},
		},
	}

	errResult := make(chan error, 1)

	go func() { errResult <- Run(ctx, cmd) }()

	// Let the worker start and the main loop settle into waiting.
	time.Sleep(300 * time.Millisecond)

	// Break the config; a SIGHUP-triggered reload must surface the error.
	require.NoError(t, os.WriteFile(cfgPath, []byte("not valid toml ["), 0o600))
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGHUP))

	select {
	case err := <-errResult:
		require.Error(t, err, "reload should fail on broken config")
	case <-time.After(2 * time.Second):
		t.Fatal("SIGHUP did not trigger a configuration reload")
	}
}

func TestCmd_Version(t *testing.T) {
	oldArgs := os.Args

	defer func() { os.Args = oldArgs }()

	os.Args = []string{"upd", "--version"}

	buf := &bytes.Buffer{}
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Cmd()
	require.NoError(t, err)

	_ = w.Close()
	os.Stdout = oldStdout

	_, _ = buf.ReadFrom(r)
}

func TestCmd_Help(t *testing.T) {
	oldArgs := os.Args

	defer func() { os.Args = oldArgs }()

	os.Args = []string{"upd", "--help"}

	err := Cmd()
	require.NoError(t, err)
}

func TestSetupLoop_reload(t *testing.T) {
	loop := logic.NewLoop()

	conf, err := SetupLoop(loop, testConfigDir+"/upd_test_reload_a.toml")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, conf.GetDelays().Up)
	assert.Equal(t, 1*time.Second, conf.GetDelays().Down)

	conf, err = SetupLoop(loop, testConfigDir+"/upd_test_reload_b.toml")
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, conf.GetDelays().Up)
	assert.Equal(t, 2*time.Second, conf.GetDelays().Down)
}
