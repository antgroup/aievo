// Package scheduler orchestrates multiple agents along an explicit SOP
// (Standard Operating Procedure) graph.
//
// vs. aievo:
//   - aievo/scheduler.go:61 mixes message dispatch and SOP traversal in
//     one long goroutine.
//   - aievo's "auto SOP" path requires a separate SopAgent that mutates a
//     mutable Graph object during execution.
//
// Our approach:
//   - The SOP is *data* (Graph), typed and frozen for a run.
//   - The scheduler is a stateless function over (Graph, Environment, Agents).
//   - Auto-SOP is a separate concern: an agent can produce a Graph value,
//     which is then handed back to MultiAgentScheduler.
package scheduler

import (
	"errors"
	"fmt"
	"strings"
)

// Node is one step in a SOP — one agent producing one message.
type Node struct {
	ID      string   // unique within the graph
	Agent   string   // name of the Agent to invoke
	Input   string   // canned prompt; can reference {{prev}} placeholders (see Run)
	Next    []string // IDs of follow-up nodes (fan-out by default)
	OnError string   // optional: ID to jump to if Agent errors; empty = fail run
}

// Graph is a DAG of Nodes with one Start. We forbid cycles to keep traversal
// simple; long-running loops should be modelled as iteration *within* an agent.
type Graph struct {
	Start string
	Nodes map[string]Node
}

// Validate checks invariants. Call before passing to Run.
func (g *Graph) Validate() error {
	if g.Start == "" {
		return errors.New("graph: Start is empty")
	}
	if _, ok := g.Nodes[g.Start]; !ok {
		return fmt.Errorf("graph: Start %q not in Nodes", g.Start)
	}
	for id, n := range g.Nodes {
		if n.ID != id {
			return fmt.Errorf("graph: node key %q != ID %q", id, n.ID)
		}
		for _, nx := range n.Next {
			if _, ok := g.Nodes[nx]; !ok {
				return fmt.Errorf("graph: node %q references missing Next %q", id, nx)
			}
		}
		if n.OnError != "" {
			if _, ok := g.Nodes[n.OnError]; !ok {
				return fmt.Errorf("graph: node %q references missing OnError %q", id, n.OnError)
			}
		}
	}
	return g.checkAcyclic()
}

func (g *Graph) checkAcyclic() error {
	// Standard DFS with WHITE/GRAY/BLACK coloring.
	const white, gray, black = 0, 1, 2
	color := make(map[string]int, len(g.Nodes))
	var dfs func(string) error
	dfs = func(id string) error {
		switch color[id] {
		case gray:
			return fmt.Errorf("graph: cycle detected at %q", id)
		case black:
			return nil
		}
		color[id] = gray
		for _, nx := range g.Nodes[id].Next {
			if err := dfs(nx); err != nil {
				return err
			}
		}
		color[id] = black
		return nil
	}
	for id := range g.Nodes {
		if err := dfs(id); err != nil {
			return err
		}
	}
	return nil
}

// templateInput replaces {{prev}} in the input string with the previous
// node's output. Unknown placeholders are left intact.
func templateInput(input, prev string) string {
	return strings.ReplaceAll(input, "{{prev}}", prev)
}
