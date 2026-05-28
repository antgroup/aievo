// Package event defines a sealed, typed event hierarchy for the framework.
//
// Why a sealed interface (vs. a Type-string enum like aievo's schema/message.go)?
//   - Compiler-enforced exhaustiveness via type switch
//   - No risk of typos in event-type strings
//   - Each variant carries its own typed payload (no map[string]any)
//
// External packages cannot satisfy the interface (the isEvent method is unexported),
// so consumers can rely on the closed set of variants defined here.
package event

import "time"

// Event is the sealed interface every framework event implements.
type Event interface {
	// isEvent is unexported so types outside this package cannot satisfy Event.
	isEvent()
	// Timestamp returns when the event was produced.
	Timestamp() time.Time
}

type base struct{ at time.Time }

func (b base) isEvent()           {}
func (b base) Timestamp() time.Time { return b.at }

// now is a package-level seam so tests can pin time.
var now = time.Now

func newBase() base { return base{at: now()} }

// ----- Variants -----

// AgentStartedEvent fires when an agent begins processing.
type AgentStartedEvent struct {
	base
	Agent string
	Turn  int
}

func NewAgentStartedEvent(agent string, turn int) AgentStartedEvent {
	return AgentStartedEvent{base: newBase(), Agent: agent, Turn: turn}
}

// LLMChunkEvent carries one streaming delta from an LLM provider.
type LLMChunkEvent struct {
	base
	Agent string
	Delta string
}

func NewLLMChunkEvent(agent, delta string) LLMChunkEvent {
	return LLMChunkEvent{base: newBase(), Agent: agent, Delta: delta}
}

// ToolCallEvent fires when the agent decides to invoke one or more tools.
type ToolCallEvent struct {
	base
	Agent string
	Calls []ToolCall
}

func NewToolCallEvent(agent string, calls []ToolCall) ToolCallEvent {
	return ToolCallEvent{base: newBase(), Agent: agent, Calls: calls}
}

// ToolResultEvent carries the outcomes of a batch of tool calls.
type ToolResultEvent struct {
	base
	Agent   string
	Results []ToolResult
}

func NewToolResultEvent(agent string, results []ToolResult) ToolResultEvent {
	return ToolResultEvent{base: newBase(), Agent: agent, Results: results}
}

// MessageEvent represents an inter-agent message (replaces aievo's schema.Message).
type MessageEvent struct {
	base
	Sender    string
	Receivers []string // empty means broadcast
	Content   string
}

func NewMessageEvent(sender string, receivers []string, content string) MessageEvent {
	return MessageEvent{base: newBase(), Sender: sender, Receivers: receivers, Content: content}
}

// TerminalEvent signals an agent has reached a terminal state.
// Reason mirrors Claude Code's transitions.ts Terminal enum.
type TerminalEvent struct {
	base
	Agent  string
	Reason TerminalReason
}

func NewTerminalEvent(agent string, reason TerminalReason) TerminalEvent {
	return TerminalEvent{base: newBase(), Agent: agent, Reason: reason}
}

// ErrorEvent carries a non-recoverable failure.
type ErrorEvent struct {
	base
	Agent string
	Err   error
}

func NewErrorEvent(agent string, err error) ErrorEvent {
	return ErrorEvent{base: newBase(), Agent: agent, Err: err}
}

// TerminalReason enumerates why a loop terminated.
type TerminalReason int

const (
	TerminalDone TerminalReason = iota
	TerminalMaxIter
	TerminalCanceled
	TerminalFatal
)

func (t TerminalReason) String() string {
	switch t {
	case TerminalDone:
		return "done"
	case TerminalMaxIter:
		return "max_iter"
	case TerminalCanceled:
		return "canceled"
	case TerminalFatal:
		return "fatal"
	}
	return "unknown"
}

// ToolCall is the typed input for a tool invocation.
type ToolCall struct {
	ID    string
	Name  string
	Input string // JSON string; tool decodes via its Schema
}

// ToolResult is the typed output of a tool invocation.
type ToolResult struct {
	CallID string
	Name   string
	Output string
	Err    error // nil on success; on failure carries a typed *errors.ToolError
}
