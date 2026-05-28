package queryloop

import (
	"context"
	stderrors "errors"

	"github.com/samson-samson/aievo-next/pkg/event"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// Step is one iteration of the loop, computed by a Driver. The Driver decides
// the next state based on what the LLM produced.
type Step struct {
	Next        State
	ToolCalls   []tool.Call    // populated when Next == StateActing
	ToolResults []tool.Result  // populated when entering StateReflecting (driver hands them back)
	Final       string         // populated when terminating with content
	Reason      string         // human-readable explanation for terminations
	Err         error
}

// Driver is the agent-specific brain plugged into the loop. Drivers are
// pure data → data; all I/O (LLM, tools) is performed via the dependencies
// the driver was constructed with. The loop owns ctx-cancellation and
// state-transition validation; the driver owns "what to do next".
type Driver interface {
	// Plan is called from StatePending or StateReflecting. The driver decides
	// to either request tool calls (Next=StateActing), finish (Next=Terminal*),
	// or escalate an error.
	Plan(ctx context.Context) Step

	// Act is called from StateActing. The driver executes the tool calls it
	// requested in the previous Plan step and returns the results, plus
	// transition to StateReflecting on success.
	Act(ctx context.Context, calls []tool.Call) Step

	// Reflect is called from StateReflecting after Act produced results.
	// The driver folds results into its own state and decides whether to
	// loop (Next=StatePlanning) or terminate.
	Reflect(ctx context.Context, results []tool.Result) Step
}

// Config tunes the loop.
type Config struct {
	MaxIterations int          // safety cap; 0 means 16
	AgentName     string       // for event labelling
	Bus           *event.Bus   // optional; nil to skip event emission
}

// Loop drives a Driver to terminal state, emitting events along the way.
type Loop struct {
	cfg    Config
	driver Driver
	state  State
}

func New(driver Driver, cfg Config) *Loop {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 16
	}
	return &Loop{driver: driver, cfg: cfg, state: StatePending}
}

// State exposes the current state. Useful for tests and debugging.
func (l *Loop) State() State { return l.state }

// Run drives the loop until terminal. The returned Step carries the
// terminating reason and any final content. The returned error mirrors
// Step.Err for callers that prefer Go's error idiom.
func (l *Loop) Run(ctx context.Context) (Step, error) {
	var lastStep Step
	var calls []tool.Call

	l.emit(ctx, event.NewAgentStartedEvent(l.cfg.AgentName, 0))

	for iter := 0; iter < l.cfg.MaxIterations; iter++ {
		// Cancellation is checked every iteration — the single most
		// important property the original aievo loop lacked.
		if err := ctx.Err(); err != nil {
			lastStep = Step{Next: StateTerminalCxl, Reason: "context cancelled", Err: err}
			l.transition(ctx, StateTerminalCxl, err)
			return lastStep, err
		}

		var step Step
		switch l.state {
		case StatePending, StateReflecting:
			if l.state == StateReflecting {
				step = l.driver.Reflect(ctx, lastStep.ToolResults)
			} else {
				step = l.driver.Plan(ctx)
			}
			// Both paths feed into a Plan-like decision.
			if l.state == StatePending {
				// Pending → Planning is implicit; the driver emits the post-Planning move.
				if err := l.transitionTo(ctx, StatePlanning); err != nil {
					return Step{Next: StateTerminalErr, Err: err}, err
				}
			}
			if l.state == StateReflecting {
				// Reflecting → driver's choice
			}
		case StatePlanning:
			step = l.driver.Plan(ctx)
		case StateActing:
			step = l.driver.Act(ctx, calls)
		default:
			err := &InvalidTransitionError{From: l.state, To: l.state}
			return Step{Next: StateTerminalErr, Err: err}, err
		}

		lastStep = step
		if step.Err != nil {
			// ctx-related errors funnel to TerminalCxl, not Err — the loop
			// honoured cancellation, it didn't fail.
			if isCtxErr(step.Err) {
				l.transition(ctx, StateTerminalCxl, nil)
				return Step{Next: StateTerminalCxl, Reason: "context cancelled", Err: step.Err}, step.Err
			}
			l.transition(ctx, StateTerminalErr, step.Err)
			return step, step.Err
		}
		if err := l.transitionTo(ctx, step.Next); err != nil {
			return Step{Next: StateTerminalErr, Err: err}, err
		}
		if step.Next == StateActing {
			calls = step.ToolCalls
		}
		if l.state.IsTerminal() {
			l.emit(ctx, event.NewTerminalEvent(l.cfg.AgentName, mapReason(l.state)))
			return step, nil
		}
	}
	// Exhausted iterations.
	l.transition(ctx, StateTerminalMax, nil)
	return Step{Next: StateTerminalMax, Reason: "max iterations reached"}, nil
}

func (l *Loop) transitionTo(ctx context.Context, to State) error {
	if to == l.state {
		return nil
	}
	if err := Transition(l.state, to); err != nil {
		return err
	}
	l.state = to
	return nil
}

func (l *Loop) transition(ctx context.Context, to State, err error) {
	// terminal moves never need legality check (we always allow them)
	l.state = to
	if err != nil {
		l.emit(ctx, event.NewErrorEvent(l.cfg.AgentName, err))
	}
}

func (l *Loop) emit(ctx context.Context, e event.Event) {
	if l.cfg.Bus != nil {
		l.cfg.Bus.Publish(ctx, e)
	}
}

func mapReason(s State) event.TerminalReason {
	switch s {
	case StateTerminalOK:
		return event.TerminalDone
	case StateTerminalMax:
		return event.TerminalMaxIter
	case StateTerminalCxl:
		return event.TerminalCanceled
	case StateTerminalErr:
		return event.TerminalFatal
	}
	return event.TerminalDone
}

// Decision constructors used by drivers to keep call sites readable.

func DecidePlanToAct(calls []tool.Call) Step {
	return Step{Next: StateActing, ToolCalls: calls}
}
func DecidePlanToFinish(final, reason string) Step {
	return Step{Next: StateTerminalOK, Final: final, Reason: reason}
}
func DecideActToReflect(results []tool.Result) Step {
	return Step{Next: StateReflecting, ToolResults: results}
}
func DecideReflectToPlan() Step {
	return Step{Next: StatePlanning}
}
func DecideReflectToFinish(final, reason string) Step {
	return Step{Next: StateTerminalOK, Final: final, Reason: reason}
}
func DecideError(err error) Step {
	return Step{Next: StateTerminalErr, Err: err}
}

// Compile-time check that LLM-related symbols stay used (helps editors).
var _ = llm.FinishStop

// isCtxErr reports whether err is (or wraps) context.Canceled / DeadlineExceeded.
func isCtxErr(err error) bool {
	return stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded)
}
