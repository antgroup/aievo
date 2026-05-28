// Package tool defines the Tool interface and a concurrency-aware scheduler.
//
// The single biggest difference vs. aievo (agent/base.go:228 doAction, which
// loops over tools serially) is that the model can request multiple tools
// per turn, and our scheduler executes those that are declared concurrency
// safe in parallel. This was directly inspired by Claude Code's
// Tool.isConcurrencySafe (see book ch. 4).
//
// Concurrency model:
//   - calls whose tool.IsConcurrencySafe() == true run in an errgroup with
//     a bounded worker pool (default GOMAXPROCS)
//   - calls whose tool.IsConcurrencySafe() == false run sequentially after
//     the safe batch, in the order the model emitted them
//   - ctx cancellation propagates: in-flight calls receive ctx, and unstarted
//     calls are skipped (their ToolResult.Err is ctx.Err())
package tool

import (
	"context"
	"encoding/json"
	"errors"
)

// Tool is the contract every executable capability satisfies.
// Methods are documented in terms of *the call site's* responsibilities to
// make the framework's invariants explicit.
type Tool interface {
	// Name uniquely identifies the tool within an agent's tool set.
	// Names must be stable: they are the contract between the LLM and the
	// executor. Use lower_snake or PascalCase consistently per project.
	Name() string

	// Description is shown to the LLM. Write it like a function docstring:
	// what this does, when to use it, and any non-obvious constraints.
	Description() string

	// Schema returns the JSON Schema for the tool's input. Used by the LLM
	// provider to validate / format model output. Return nil to accept any
	// JSON string (rare; usually a schema improves model reliability).
	Schema() []byte

	// IsConcurrencySafe reports whether multiple concurrent invocations of
	// this tool can run safely in parallel. File-write, shell, and stateful
	// remote APIs typically return false. Pure-read and idempotent network
	// calls return true. The scheduler partitions calls accordingly.
	IsConcurrencySafe() bool

	// Call executes the tool. input is the raw JSON the model produced and
	// should be validated against Schema. Implementations MUST honour ctx
	// for cancellation. Return a *errors.ToolError (Retryable as appropriate)
	// or a plain error (treated as non-retryable).
	Call(ctx context.Context, input string) (string, error)
}

// Call describes a model-requested invocation.
type Call struct {
	ID    string // model-issued correlation ID; echoed back in the Result
	Name  string
	Input string // raw JSON; see Tool.Schema
}

// Result is the executor's reply for one Call.
type Result struct {
	CallID string
	Name   string
	Output string
	Err    error
}

// FindByName looks up a tool by name. Returns nil if not found — callers
// should treat that as a non-retryable error (the model hallucinated a tool).
func FindByName(name string, tools []Tool) Tool {
	for _, t := range tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

// ValidateInput parses input against the tool's schema, if any. Returns nil
// for the no-op case (no schema or input parses as valid JSON of any shape).
// This is a *light* validation — strict schema validation is delegated to
// the caller via a third-party library when needed.
func ValidateInput(t Tool, input string) error {
	if len(t.Schema()) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return errors.New("input is not valid JSON")
	}
	return nil
}
