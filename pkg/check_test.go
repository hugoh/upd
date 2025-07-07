package pkg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeProbe struct {
	ret *Report
}

func (f *fakeProbe) Probe(ctx context.Context, timeout time.Duration) *Report {
	return f.ret
}
func (f *fakeProbe) Scheme() string { return "fake" }

type fakeCheckListIterator struct {
	checks []*Check
	idx    int
}

func (it *fakeCheckListIterator) Fetch() *Check {
	if it.idx >= len(it.checks) {
		return nil
	}
	c := it.checks[it.idx]
	it.idx++
	return c
}

type recordChecker struct {
	run  []Check
	succ []*Report
	fail []*Report
}

func (r *recordChecker) CheckRun(c Check)         { r.run = append(r.run, c) }
func (r *recordChecker) ProbeSuccess(rep *Report) { r.succ = append(r.succ, rep) }
func (r *recordChecker) ProbeFailure(rep *Report) { r.fail = append(r.fail, rep) }

func TestCheckerRun_SuccessFirst(t *testing.T) {
	probe := &fakeProbe{ret: &Report{}}
	probeIface := Probe(probe)
	check := &Check{Probe: &probeIface, Timeout: 1 * time.Second}
	it := &fakeCheckListIterator{checks: []*Check{check}}
	checker := &recordChecker{}
	ctx := context.Background()
	ok, err := CheckerRun(ctx, checker, it)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Len(t, checker.run, 1)
	assert.Len(t, checker.succ, 1)
	assert.Len(t, checker.fail, 0)
}

func TestCheckerRun_AllFail(t *testing.T) {
	rep := &Report{error: errors.New("fail")}
	probe := &fakeProbe{ret: rep}
	probeIface := Probe(probe)
	check := &Check{Probe: &probeIface, Timeout: 1 * time.Second}
	it := &fakeCheckListIterator{checks: []*Check{check, check}}
	checker := &recordChecker{}
	ctx := context.Background()
	ok, err := CheckerRun(ctx, checker, it)
	assert.False(t, ok)
	assert.NoError(t, err)
	assert.Len(t, checker.run, 2)
	assert.Len(t, checker.succ, 0)
	assert.Len(t, checker.fail, 2)
}

func TestCheckerRun_Empty(t *testing.T) {
	it := &fakeCheckListIterator{checks: []*Check{}}
	checker := &recordChecker{}
	ctx := context.Background()
	ok, err := CheckerRun(ctx, checker, it)
	assert.False(t, ok)
	assert.NoError(t, err)
	assert.Len(t, checker.run, 0)
	assert.Len(t, checker.succ, 0)
	assert.Len(t, checker.fail, 0)
}

func TestCheckerRun_WithCheckListIterator(t *testing.T) {
	probe := &fakeProbe{ret: &Report{}}
	probeIface := Probe(probe)
	check := &Check{Probe: &probeIface, Timeout: 1 * time.Second}
	cl := &CheckList{Ordered: Checks{check}}
	it := cl.GetIterator()
	checker := &recordChecker{}
	ctx := context.Background()
	ok, err := CheckerRun(ctx, checker, it)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Len(t, checker.run, 1)
	assert.Len(t, checker.succ, 1)
	assert.Len(t, checker.fail, 0)
}
