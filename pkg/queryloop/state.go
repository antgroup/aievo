// Package queryloop implements the agent's main control loop as an
// explicit, testable state machine.
//
// Why a state machine (vs. aievo's BaseAgent.Run unstructured for-loop)?
//   - Every transition is named and inspectable; easy to debug "why did it
//     stop here?".
//   - Termination is an enum, not a bool — callers see *why* the loop ended.
//   - The transition table is data, not code paths — adding a state is a
//     map entry, not a refactor.
//   - Inspired by Claude Code's query/transitions.ts (11 Terminal + 7 Continue).
package queryloop

import "fmt"

// State enumerates the phases of one agent turn.
type State int

const (
	StatePending     State = iota // initial; no work done yet
	StatePlanning                 // calling LLM to decide next action
	StateActing                   // executing tool calls
	StateReflecting               // incorporating tool results
	StateTerminalOK               // finished cleanly
	StateTerminalErr              // unrecoverable error
	StateTerminalMax              // hit max iterations
	StateTerminalCxl              // ctx cancelled
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "Pending"
	case StatePlanning:
		return "Planning"
	case StateActing:
		return "Acting"
	case StateReflecting:
		return "Reflecting"
	case StateTerminalOK:
		return "TerminalOK"
	case StateTerminalErr:
		return "TerminalErr"
	case StateTerminalMax:
		return "TerminalMaxIter"
	case StateTerminalCxl:
		return "TerminalCancelled"
	}
	return fmt.Sprintf("State(%d)", s)
}

// IsTerminal reports whether s ends the loop.
func (s State) IsTerminal() bool {
	return s == StateTerminalOK ||
		s == StateTerminalErr ||
		s == StateTerminalMax ||
		s == StateTerminalCxl
}
