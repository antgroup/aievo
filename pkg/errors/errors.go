// Package errors defines typed errors and retry policies for aievo-next.
//
// Why typed errors (vs. aievo's bare `return err`)?
//   - Caller can decide via errors.As whether to retry, surface to user, or escalate.
//   - Each typed error declares its own Retryable() so the retry policy is local
//     to the source of failure rather than encoded in distant call sites.
//   - Wrap/Unwrap behaviour stays compatible with the stdlib errors package.
package errors

import (
	stderrors "errors"
	"fmt"
)

// Retryable is implemented by error values that know whether retry is safe.
type Retryable interface {
	error
	Retryable() bool
}

// IsRetryable reports whether err (or any error wrapped by it) implements
// Retryable and returns true. Non-Retryable errors are treated as terminal.
func IsRetryable(err error) bool {
	var r Retryable
	for err != nil {
		if stderrors.As(err, &r) {
			return r.Retryable()
		}
		err = stderrors.Unwrap(err)
	}
	return false
}

// ToolError reports a failure executing a tool. The Retryable flag is set
// by the tool's Call site (e.g. ETIMEDOUT → true, schema-validation → false).
type ToolError struct {
	ToolName string
	Inner    error
	retry    bool
}

func NewToolError(tool string, inner error, retryable bool) *ToolError {
	return &ToolError{ToolName: tool, Inner: inner, retry: retryable}
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("tool %q failed: %v", e.ToolName, e.Inner)
}
func (e *ToolError) Unwrap() error    { return e.Inner }
func (e *ToolError) Retryable() bool  { return e.retry }

// LLMError reports a failure from an LLM provider. HTTP 5xx and 429 are
// retryable; 4xx (except 429) are not.
type LLMError struct {
	Provider   string
	StatusCode int
	Inner      error
}

func NewLLMError(provider string, statusCode int, inner error) *LLMError {
	return &LLMError{Provider: provider, StatusCode: statusCode, Inner: inner}
}

func (e *LLMError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s LLM error (HTTP %d): %v", e.Provider, e.StatusCode, e.Inner)
	}
	return fmt.Sprintf("%s LLM error: %v", e.Provider, e.Inner)
}
func (e *LLMError) Unwrap() error { return e.Inner }
func (e *LLMError) Retryable() bool {
	if e.StatusCode == 429 {
		return true
	}
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// TransientError marks an arbitrary error as retryable. Useful when wrapping
// errors from libraries that don't implement Retryable themselves
// (e.g. network timeouts surfaced as *net.OpError).
type TransientError struct{ Inner error }

func NewTransient(err error) *TransientError { return &TransientError{Inner: err} }

func (e *TransientError) Error() string   { return "transient: " + e.Inner.Error() }
func (e *TransientError) Unwrap() error   { return e.Inner }
func (e *TransientError) Retryable() bool { return true }

// FatalError marks an arbitrary error as non-retryable.
type FatalError struct{ Inner error }

func NewFatal(err error) *FatalError { return &FatalError{Inner: err} }

func (e *FatalError) Error() string   { return "fatal: " + e.Inner.Error() }
func (e *FatalError) Unwrap() error   { return e.Inner }
func (e *FatalError) Retryable() bool { return false }
