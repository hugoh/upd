package logic

import (
	"context"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/status"
	"github.com/stretchr/testify/assert"
)

// newTestLoop builds and configures a Loop for tests that drive Run/Stop
// directly, keeping the check-list/delays/periods setup out of each test.
func newTestLoop(
	t *testing.T,
	checkList *check.List,
	delays Delays,
	periods ...time.Duration,
) *Loop {
	t.Helper()

	loop := NewLoop()
	loop.Configure(checkList, delays, nil, status.BucketConfig{}, periods...)

	return loop
}

// runLoopAsync starts loop.Run in a goroutine bound to a cancelable context
// and returns the cancel func plus a channel closed once Run() returns; the
// context is also canceled on test cleanup.
func runLoopAsync(t *testing.T, loop *Loop) (context.CancelFunc, <-chan struct{}) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan struct{})

	go func() {
		defer close(done)

		loop.Run(ctx, &status.StatServerConfig{})
	}()

	return cancel, done
}

// waitDone blocks until done is closed or fails the test after timeout.
func waitDone(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("Run() did not exit within expected time")
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	loop := newTestLoop(t, &check.List{}, Delays{Up: 1 * time.Second, Down: 1 * time.Second})
	cancel, done := runLoopAsync(t, loop)

	cancel()
	waitDone(t, done, 1*time.Second)
}

func TestRun_ProcessesChecks(t *testing.T) {
	probe := check.Probe(check.NewHTTPProbe("http://example.invalid"))
	dummyCheck := &check.Check{
		Probe:   probe,
		Timeout: 1 * time.Second,
	}
	checkList := &check.List{
		Ordered: check.Checks{dummyCheck},
	}

	loop := newTestLoop(
		t,
		checkList,
		Delays{Up: 10 * time.Millisecond, Down: 10 * time.Millisecond},
	)
	runLoopAsync(t, loop)

	assert.Eventually(t, func() bool {
		return loop.status.GenStatReport(nil).Loop.NextCheck != 0
	}, 1*time.Second, 5*time.Millisecond, "loop should have processed at least one check")
}

func TestRun_DoesNotUpdateLastSuccessOnFailure(t *testing.T) {
	// An empty check list means CheckerRun always returns false (no checks
	// pass), simulating an outage on every iteration.
	loop := newTestLoop(
		t,
		&check.List{},
		Delays{Up: 10 * time.Millisecond, Down: 10 * time.Millisecond},
	)
	cancel, done := runLoopAsync(t, loop)

	assert.Eventually(t, func() bool {
		return loop.status.GenStatReport(nil).Loop.NextCheck != 0
	}, 1*time.Second, 5*time.Millisecond, "loop should have processed at least one check")

	cancel()
	waitDone(t, done, 1*time.Second)

	assert.True(t, loop.lastSuccess.IsZero(),
		"lastSuccess should never be set when every check fails")
}

func TestStop_StopsStatServer(t *testing.T) {
	loop := newTestLoop(t, &check.List{}, Delays{Up: 1 * time.Second, Down: 1 * time.Second})

	ctx := t.Context()

	assert.Nil(t, loop.statServer)

	loop.Stop(ctx)

	assert.Nil(t, loop.statServer)
}

func TestRun_StopsTimerOnContextCancel(t *testing.T) {
	longDelay := 10 * time.Second
	loop := newTestLoop(t, &check.List{}, Delays{Up: longDelay, Down: longDelay}, 0)

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
