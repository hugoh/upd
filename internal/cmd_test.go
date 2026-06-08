package internal

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/logic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

const testConfigDir = "../testdata"

func TestRun_NoMultipleRestartsOnSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	ctx, cancel := context.WithCancel(context.Background())

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

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
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
	assert.Equal(t, 5*time.Second, conf.GetDelays()[true])
	assert.Equal(t, 1*time.Second, conf.GetDelays()[false])

	conf, err = SetupLoop(loop, testConfigDir+"/upd_test_reload_b.toml")
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, conf.GetDelays()[true])
	assert.Equal(t, 2*time.Second, conf.GetDelays()[false])
}
