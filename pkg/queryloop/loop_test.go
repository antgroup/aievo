package queryloop

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samson-samson/aievo-next/pkg/tool"
)

func TestIsLegal_HappyPath(t *testing.T) {
	cases := []struct{ from, to State }{
		{StatePending, StatePlanning},
		{StatePlanning, StateActing},
		{StateActing, StateReflecting},
		{StateReflecting, StatePlanning},
		{StateReflecting, StateTerminalOK},
		{StatePlanning, StateTerminalOK},
	}
	for _, c := range cases {
		if !IsLegal(c.from, c.to) {
			t.Errorf("%s → %s should be legal", c.from, c.to)
		}
	}
}

func TestIsLegal_Illegal(t *testing.T) {
	bad := []struct{ from, to State }{
		{StatePending, StateActing},     // can't act without planning
		{StatePending, StateReflecting}, // can't reflect without acting
		{StateActing, StatePlanning},    // must reflect first
	}
	for _, c := range bad {
		if IsLegal(c.from, c.to) {
			t.Errorf("%s → %s should be illegal", c.from, c.to)
		}
	}
}

// noopDriver returns predetermined Steps from a script.
type noopDriver struct {
	planSteps    []Step
	reflectSteps []Step
	planIdx      int
	reflectIdx   int
	acts         int64
}

func (d *noopDriver) Plan(ctx context.Context) Step {
	if d.planIdx >= len(d.planSteps) {
		return DecidePlanToFinish("done", "exhausted plans")
	}
	s := d.planSteps[d.planIdx]
	d.planIdx++
	return s
}

func (d *noopDriver) Act(ctx context.Context, calls []tool.Call) Step {
	atomic.AddInt64(&d.acts, 1)
	results := make([]tool.Result, len(calls))
	for i, c := range calls {
		results[i] = tool.Result{CallID: c.ID, Name: c.Name, Output: "ok"}
	}
	return DecideActToReflect(results)
}

func (d *noopDriver) Reflect(ctx context.Context, _ []tool.Result) Step {
	if d.reflectIdx >= len(d.reflectSteps) {
		return DecideReflectToFinish("done", "exhausted reflects")
	}
	s := d.reflectSteps[d.reflectIdx]
	d.reflectIdx++
	return s
}

func TestLoop_PlanActReflectFinish(t *testing.T) {
	d := &noopDriver{
		planSteps:    []Step{DecidePlanToAct([]tool.Call{{ID: "1", Name: "X"}})},
		reflectSteps: []Step{DecideReflectToFinish("answer", "ok")},
	}
	l := New(d, Config{AgentName: "test"})
	step, err := l.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if step.Next != StateTerminalOK {
		t.Fatalf("got %v, want TerminalOK", step.Next)
	}
	if step.Final != "answer" {
		t.Fatalf("final = %q", step.Final)
	}
	if d.acts != 1 {
		t.Fatalf("expected 1 act, got %d", d.acts)
	}
}

func TestLoop_RespectsContextCancellation(t *testing.T) {
	// A driver that blocks in Plan; ctx cancellation must surface.
	d := &slowDriver{planDelay: 100 * time.Millisecond}
	l := New(d, Config{AgentName: "test", MaxIterations: 100})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := l.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx error, got %v", err)
	}
	if l.State() != StateTerminalCxl {
		t.Fatalf("expected TerminalCxl, got %v", l.State())
	}
}

// slowDriver makes Plan respect ctx so cancellation can be observed.
type slowDriver struct{ planDelay time.Duration }

func (d *slowDriver) Plan(ctx context.Context) Step {
	select {
	case <-ctx.Done():
		return DecideError(ctx.Err())
	case <-time.After(d.planDelay):
	}
	return DecidePlanToFinish("done", "ok")
}
func (d *slowDriver) Act(ctx context.Context, _ []tool.Call) Step {
	return DecideActToReflect(nil)
}
func (d *slowDriver) Reflect(ctx context.Context, _ []tool.Result) Step {
	return DecideReflectToFinish("done", "ok")
}


func TestLoop_HitsMaxIterations(t *testing.T) {
	d := &noopDriver{
		planSteps: []Step{
			DecidePlanToAct([]tool.Call{{ID: "1", Name: "X"}}),
			DecidePlanToAct([]tool.Call{{ID: "2", Name: "X"}}),
		},
		reflectSteps: []Step{DecideReflectToPlan(), DecideReflectToPlan()},
	}
	l := New(d, Config{AgentName: "test", MaxIterations: 3})
	step, _ := l.Run(context.Background())
	if step.Next != StateTerminalMax {
		t.Fatalf("got %v, want TerminalMax", step.Next)
	}
}
