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
		Delays{Up: 1 * time.Second, Down: 1 * time.Second},
		nil,
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		loop.Run(ctx, &status.StatServerConfig{})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestRun_ProcessesChecks(t *testing.T) {
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
		Delays{Up: 10 * time.Millisecond, Down: 10 * time.Millisecond},
		nil,
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		loop.Run(ctx, &status.StatServerConfig{})
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestStop_StopsStatServer(t *testing.T) {
	loop := NewLoop()
	emptyCheckList := &check.List{}
	loop.Configure(
		emptyCheckList,
		Delays{Up: 1 * time.Second, Down: 1 * time.Second},
		nil,
	)

	ctx := t.Context()

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
		Delays{Up: longDelay, Down: longDelay},
		nil,
		0,
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	start := time.Now()

	go func() {
		loop.Run(ctx, &status.StatServerConfig{})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	select {
	case <-done:
		timer.Stop()

		elapsed := time.Since(start)
		assert.Less(
			t,
			elapsed,
			500*time.Millisecond,
			"Run() should exit quickly after context cancel, not wait for timer",
		)

	case <-timer.C:
		t.Fatal("Run() did not exit within expected time - timer may not be stopped properly")
	}
}
