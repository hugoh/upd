package logic

import (
	"context"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/status"
	"github.com/stretchr/testify/assert"
)

func TestRun_StopsOnContextCancel(t *testing.T) {
	loop := NewLoop()
	emptyCheckList := &check.List{}
	loop.Configure(
		emptyCheckList,
		Delays{true: 1 * time.Second, false: 1 * time.Second},
		nil,
		0,
		&status.StatServerConfig{},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		loop.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestRun_ProcessesChecks(_ *testing.T) {
	loop := NewLoop()
	probe := check.Probe(check.NewHTTPProbe("http://example.invalid"))
	dummyCheck := &check.Check{
		Probe:   probe,
		Timeout: 1 * time.Second,
	}
	checkList := &check.List{
		Ordered: check.Checks{dummyCheck},
	}

	loop.Configure(
		checkList,
		Delays{true: 10 * time.Millisecond, false: 10 * time.Millisecond},
		nil,
		0,
		&status.StatServerConfig{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		loop.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestStop_StopsStatServer(t *testing.T) {
	loop := NewLoop()
	emptyCheckList := &check.List{}
	loop.Configure(
		emptyCheckList,
		Delays{true: 1 * time.Second, false: 1 * time.Second},
		nil,
		0,
		nil,
	)

	ctx := context.Background()

	assert.Nil(t, loop.statServer)

	loop.Stop(ctx)

	assert.Nil(t, loop.statServer)
}

func TestRun_StopsTimerOnContextCancel(t *testing.T) {
	loop := NewLoop()
	emptyCheckList := &check.List{}
	longDelay := 10 * time.Second
	loop.Configure(
		emptyCheckList,
		Delays{true: longDelay, false: longDelay},
		nil,
		0,
		&status.StatServerConfig{},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	start := time.Now()

	go func() {
		loop.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		elapsed := time.Since(start)
		assert.Less(
			t,
			elapsed,
			500*time.Millisecond,
			"Run() should exit quickly after context cancel, not wait for timer",
		)
	case <-time.After(1 * time.Second):
		t.Fatal("Run() did not exit within expected time - timer may not be stopped properly")
	}
}
