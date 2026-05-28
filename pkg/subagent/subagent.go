// Package subagent lets a parent agent spawn isolated child agents through
// a tool call — the Claude Code AgentTool equivalent.
//
// The child runs in its own conversation (no parent message history leaks),
// has its own tool set and prompt, and returns a single string. This gives
// agents the power to delegate large self-contained sub-problems without
// blowing up parent context.
package subagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/samson-samson/aievo-next/pkg/agent"
	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// Spec describes one kind of subagent the parent can spawn. Multiple Specs
// register under distinct ToolName values.
type Spec struct {
	// ToolName the parent uses to invoke this subagent. Convention:
	// "AgentExplore", "AgentReview" etc.
	ToolName string

	// Description shown to the parent's LLM.
	Description string

	// Build constructs a fresh child agent given the spawn input. Called
	// once per spawn; the returned agent.Agent is discarded after Run.
	Build func(input string) (agent.Agent, error)
}

// NewTool wraps Spec as a tool.Tool. The tool is concurrency-safe — multiple
// subagents may run in parallel as long as Build returns independent state.
func NewTool(s Spec) tool.Tool {
	return &spawner{spec: s}
}

type spawner struct{ spec Spec }

func (s *spawner) Name() string             { return s.spec.ToolName }
func (s *spawner) Description() string      { return s.spec.Description }
func (s *spawner) IsConcurrencySafe() bool  { return true }
func (s *spawner) Schema() []byte {
	return []byte(`{"type":"object","properties":{"task":{"type":"string","description":"the subagent's task"}},"required":["task"]}`)
}
func (s *spawner) Call(ctx context.Context, input string) (string, error) {
	var in struct{ Task string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("%s: %w", s.spec.ToolName, err))
	}
	child, err := s.spec.Build(in.Task)
	if err != nil {
		return "", err
	}
	res, err := child.Run(ctx, in.Task)
	if err != nil {
		return "", err
	}
	return res.Final, nil
}
