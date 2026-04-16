package logic

import (
	"context"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/status"
	"github.com/hugoh/upd/pkg"
	"github.com/stretchr/testify/assert"
)

func TestRun_StopsOnContextCancel(t *testing.T) {
	loop := NewLoop()
	emptyCheckList := &pkg.CheckList{}
	loop.Configure(
		emptyCheckList,
		Delays{true: 1 * time.Second, false: 1 * time.Second},
		nil,
		0,
		&status.StatServerConfig{},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	assert.Nil(t, loop.statServer)

	go func() {
		loop.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	firstServer := loop.statServer

	time.Sleep(50 * time.Millisecond)

	assert.Same(t, firstServer, loop.statServer, "stat server should not change during Run")
}

func TestRun_ProcessesChecks(_ *testing.T) {
	loop := NewLoop()
	probe := pkg.Probe(pkg.NewHTTPProbe("http://example.invalid"))
	dummyCheck := &pkg.Check{
		Probe:   &probe,
		Timeout: 1 * time.Second,
	}
	checkList := &pkg.CheckList{
		Ordered: pkg.Checks{dummyCheck},
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
	emptyCheckList := &pkg.CheckList{}
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
	emptyCheckList := &pkg.CheckList{}
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
