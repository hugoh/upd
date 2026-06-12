package check

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeProbe struct {
	ret *Report
}

func (f *fakeProbe) Execute(_ context.Context, _ time.Duration) *Report {
	return f.ret
}
func (*fakeProbe) Scheme() string { return "fake" }
func (*fakeProbe) Target() string { return "fake" }

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
	check := &Check{Probe: probeIface, Timeout: 1 * time.Second}
	checker := &recordChecker{}
	ok := CheckerRun(t.Context(), checker, slices.Values([]*Check{check}))
	assert.True(t, ok)
	assert.Len(t, checker.run, 1)
	assert.Len(t, checker.succ, 1)
	assert.Empty(t, checker.fail)
}

func TestCheckerRun_AllFail(t *testing.T) {
	rep := &Report{error: errors.New("fail")}
	probe := &fakeProbe{ret: rep}
	probeIface := Probe(probe)
	check := &Check{Probe: probeIface, Timeout: 1 * time.Second}
	checker := &recordChecker{}
	ok := CheckerRun(t.Context(), checker, slices.Values([]*Check{check, check}))
	assert.False(t, ok)
	assert.Len(t, checker.run, 2)
	assert.Empty(t, checker.succ)
	assert.Len(t, checker.fail, 2)
}

func TestCheckerRun_Empty(t *testing.T) {
	checker := &recordChecker{}
	ok := CheckerRun(t.Context(), checker, slices.Values([]*Check{}))
	assert.False(t, ok)
	assert.Empty(t, checker.run)
	assert.Empty(t, checker.succ)
	assert.Empty(t, checker.fail)
}

func TestCheckerRun_WithList(t *testing.T) {
	probe := &fakeProbe{ret: &Report{}}
	probeIface := Probe(probe)
	check := &Check{Probe: probeIface, Timeout: 1 * time.Second}
	cl := &List{Ordered: Checks{check}}
	checker := &recordChecker{}
	ok := CheckerRun(t.Context(), checker, cl.All())
	assert.True(t, ok)
	assert.Len(t, checker.run, 1)
	assert.Len(t, checker.succ, 1)
	assert.Empty(t, checker.fail)
}
