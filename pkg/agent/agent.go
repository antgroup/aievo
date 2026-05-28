// Package agent provides the Agent interface and a ReAct-style driver that
// plugs into the queryloop state machine.
//
// Agent here is small on purpose. Anything that can Plan/Act/Reflect once is
// an agent; multi-agent orchestration lives in pkg/scheduler.
//
// Compared with aievo (agent/base.go ~430 lines mixing LLM call, tool dispatch,
// retry, and prompt assembly), our split is:
//
//	pkg/llm       — provider streaming
//	pkg/tool      — tool interface + concurrent scheduler
//	pkg/queryloop — explicit state machine
//	pkg/agent     — wires the three above, owns prompt + context window
package agent

import (
	"context"

	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// Agent is the smallest contract every agent satisfies. It is intentionally
// narrow: name + Run. Construction-time wiring (LLM, tools, prompt) is a
// concern of concrete types (e.g. ReActAgent).
type Agent interface {
	Name() string
	// Run drives the agent to terminal state with the given user input,
	// returning the final assistant message and any error.
	Run(ctx context.Context, input string) (Result, error)
}

// Result is the outcome of one Agent.Run.
type Result struct {
	Final string
	Usage llm.Usage
	Steps int       // total state transitions taken
	Calls []toolLog // per-call audit trail
}

// toolLog records one tool invocation for diagnostics.
type toolLog struct {
	Name   string
	Input  string
	Output string
	Err    error
}

// SystemPromptFn computes the system prompt for one Run call. Often a
// closure over the agent's role / context; using a function (not a string)
// lets the prompt incorporate time-varying state (current time, memory).
type SystemPromptFn func(ctx context.Context) string

// StaticSystemPrompt is a convenience wrapper for prompts that don't vary.
func StaticSystemPrompt(s string) SystemPromptFn {
	return func(context.Context) string { return s }
}

// toolSpecsOf converts a Tool slice into the llm.ToolSpec slice that
// providers need to advertise tools to the model.
func toolSpecsOf(tools []tool.Tool) []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(tools))
	for _, t := range tools {
		specs = append(specs, llm.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return specs
}
