package queryloop

import "github.com/samson-samson/aievo-next/pkg/featureflag"

// allowedTransitions is the canonical table of legal state moves.
// Reading top-to-bottom:
//   - From Pending you can only start Planning.
//   - Planning either issues tool calls (→ Acting) or finishes (→ TerminalOK / Err).
//   - Acting collects results then Reflects (→ Reflecting) or fails (→ TerminalErr).
//   - Reflecting either loops back to Planning (more work) or finishes (TerminalOK).
//
// Any move not listed here is illegal. Strict mode (feature flag) panics
// on illegal moves to surface bugs in CI; production mode returns an error.
var allowedTransitions = map[State]map[State]struct{}{
	StatePending: {
		StatePlanning:    {},
		StateTerminalCxl: {},
	},
	StatePlanning: {
		StateActing:      {},
		StateTerminalOK:  {},
		StateTerminalErr: {},
		StateTerminalMax: {},
		StateTerminalCxl: {},
	},
	StateActing: {
		StateReflecting:  {},
		StateTerminalErr: {},
		StateTerminalCxl: {},
	},
	StateReflecting: {
		StatePlanning:    {},
		StateTerminalOK:  {},
		StateTerminalMax: {},
		StateTerminalCxl: {},
	},
}

// InvalidTransitionError is returned for illegal moves when strict mode is off.
type InvalidTransitionError struct {
	From, To State
}

func (e *InvalidTransitionError) Error() string {
	return "invalid transition: " + e.From.String() + " → " + e.To.String()
}

// IsLegal reports whether moving from → to is in the allowed table.
func IsLegal(from, to State) bool {
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

// Transition validates and returns an error (or panics in strict mode).
// Pure check — does not mutate any state.
func Transition(from, to State) error {
	if IsLegal(from, to) {
		return nil
	}
	err := &InvalidTransitionError{From: from, To: to}
	if featureflag.Enabled(featureflag.FlagStrictTransitions) {
		panic(err)
	}
	return err
}
