package internal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestRun_NoMultipleRestartsOnSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  ConfigConfig,
				Value: testConfigDir + "/upd_test_minimal.yaml",
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
				Value: testConfigDir + "/upd_test_minimal.yaml",
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
