// Package mock is a deterministic LLM for tests and examples that should
// not hit a real provider.
//
// Usage:
//
//	mock := NewMock().
//	  ExpectAndRespond(Script{
//	    Delta:    "hello",
//	    ToolCall: &llm.ToolCall{Name: "Echo", Input: `{"msg":"hi"}`},
//	    Finish:   llm.FinishToolCalls,
//	  })
package mock

import (
	"context"
	"sync"

	"github.com/samson-samson/aievo-next/pkg/llm"
)

// Script is one canned response. Set Delta for text, ToolCall for a tool
// invocation, or both. Finish defaults to FinishStop.
type Script struct {
	Delta     string
	Reasoning string
	ToolCall  *llm.ToolCall
	Finish    llm.FinishReason
	Usage     llm.Usage
	Err       error // if set, emits ErrorEvent and closes
}

// Mock implements llm.LLM by replaying canned Scripts in FIFO order.
// One Stream call consumes one Script.
type Mock struct {
	mu      sync.Mutex
	scripts []Script
	calls   []llm.Request
}

func NewMock() *Mock { return &Mock{} }

// ExpectAndRespond appends a canned response. Returns the receiver for chaining.
func (m *Mock) ExpectAndRespond(s Script) *Mock {
	m.mu.Lock()
	m.scripts = append(m.scripts, s)
	m.mu.Unlock()
	return m
}

// Calls returns every Request seen so far (snapshot).
func (m *Mock) Calls() []llm.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]llm.Request, len(m.calls))
	copy(out, m.calls)
	return out
}

// Stream pops the next Script (or returns a default stop if none queued).
func (m *Mock) Stream(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	m.mu.Lock()
	m.calls = append(m.calls, req)
	var s Script
	if len(m.scripts) > 0 {
		s = m.scripts[0]
		m.scripts = m.scripts[1:]
	} else {
		s = Script{Finish: llm.FinishStop}
	}
	m.mu.Unlock()

	ch := make(chan llm.StreamEvent, 4)
	go func() {
		defer close(ch)
		if s.Err != nil {
			select {
			case <-ctx.Done():
			case ch <- llm.ErrorEvent{Err: s.Err}:
			}
			return
		}
		if s.Reasoning != "" {
			select {
			case <-ctx.Done():
				return
			case ch <- llm.ReasoningDeltaEvent{Content: s.Reasoning}:
			}
		}
		if s.Delta != "" {
			select {
			case <-ctx.Done():
				return
			case ch <- llm.DeltaEvent{Content: s.Delta}:
			}
		}
		if s.ToolCall != nil {
			select {
			case <-ctx.Done():
				return
			case ch <- llm.ToolUseEvent{Call: *s.ToolCall}:
			}
		}
		select {
		case <-ctx.Done():
		case ch <- llm.FinishEvent{Reason: s.Finish, Usage: s.Usage}:
		}
	}()
	return ch, nil
}
