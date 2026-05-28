package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/event"
)

// MultiAgentScheduler executes a SOP graph against a roster of agents and
// a shared environment. Each step's output becomes the next step's input
// (via {{prev}} substitution) and is appended to env.Memory as a MessageEvent.
type MultiAgentScheduler struct {
	Agents map[string]agent.Agent
	Env    *env.Environment
}

// NewMultiAgent constructs a scheduler from a slice of agents.
func NewMultiAgent(agents []agent.Agent, e *env.Environment) *MultiAgentScheduler {
	m := make(map[string]agent.Agent, len(agents))
	for _, a := range agents {
		m[a.Name()] = a
	}
	return &MultiAgentScheduler{Agents: m, Env: e}
}

// RunResult is the outcome of one SOP execution.
type RunResult struct {
	Visited []string // node IDs in visitation order
	Outputs map[string]string
	Final   string // last visited node's output
}

// Run executes graph and returns the trail. Cancellation propagates: any
// node that the current ctx targets respects ctx; pending nodes are skipped.
// Errors abort the run (or jump to OnError if defined).
//
// Concurrency model: Next slices fan out *sequentially* in v0.1 — this is
// intentional to keep memory ordering predictable. v0.2 will add a typed
// FanOut node that runs siblings in parallel via errgroup.
func (s *MultiAgentScheduler) Run(ctx context.Context, g *Graph, initial string) (*RunResult, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	res := &RunResult{Outputs: map[string]string{}}
	prev := initial

	var step func(id string) error
	step = func(id string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		node, ok := g.Nodes[id]
		if !ok {
			return fmt.Errorf("scheduler: unknown node %q", id)
		}
		a, ok := s.Agents[node.Agent]
		if !ok {
			return fmt.Errorf("scheduler: unknown agent %q (in node %q)", node.Agent, id)
		}
		input := templateInput(node.Input, prev)
		ar, err := a.Run(ctx, input)
		if err != nil {
			if node.OnError != "" {
				return step(node.OnError)
			}
			return fmt.Errorf("node %q (agent %q) failed: %w", id, node.Agent, err)
		}
		res.Visited = append(res.Visited, id)
		res.Outputs[id] = ar.Final
		res.Final = ar.Final
		prev = ar.Final
		if _, err := s.Env.Memory.Append(ctx, event.NewMessageEvent(node.Agent, nil, ar.Final)); err != nil {
			return err
		}
		s.Env.Bus.Publish(ctx, event.NewMessageEvent(node.Agent, nil, ar.Final))
		// Sequential fan-out for v0.1.
		for _, nx := range node.Next {
			if err := step(nx); err != nil {
				return err
			}
		}
		return nil
	}

	if err := step(g.Start); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return res, err
		}
		return res, err
	}
	return res, nil
}
