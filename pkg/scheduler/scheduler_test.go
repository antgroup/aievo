package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
)

// staticAgent returns a fixed response — useful for orchestration tests.
type staticAgent struct {
	name string
	resp string
	err  error
}

func (a *staticAgent) Name() string { return a.name }
func (a *staticAgent) Run(ctx context.Context, input string) (agent.Result, error) {
	if a.err != nil {
		return agent.Result{}, a.err
	}
	return agent.Result{Final: a.resp + "(" + input + ")"}, nil
}

func TestGraph_Validate_Cycle(t *testing.T) {
	g := &Graph{
		Start: "a",
		Nodes: map[string]Node{
			"a": {ID: "a", Agent: "X", Next: []string{"b"}},
			"b": {ID: "b", Agent: "X", Next: []string{"a"}},
		},
	}
	if err := g.Validate(); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestGraph_Validate_DanglingRef(t *testing.T) {
	g := &Graph{
		Start: "a",
		Nodes: map[string]Node{
			"a": {ID: "a", Agent: "X", Next: []string{"missing"}},
		},
	}
	if err := g.Validate(); err == nil {
		t.Fatal("expected dangling ref error")
	}
}

func TestScheduler_LinearChain(t *testing.T) {
	e := env.New()
	defer e.Close()
	agents := []agent.Agent{
		&staticAgent{name: "A1", resp: "A1"},
		&staticAgent{name: "A2", resp: "A2"},
		&staticAgent{name: "A3", resp: "A3"},
	}
	s := NewMultiAgent(agents, e)
	g := &Graph{
		Start: "n1",
		Nodes: map[string]Node{
			"n1": {ID: "n1", Agent: "A1", Input: "start", Next: []string{"n2"}},
			"n2": {ID: "n2", Agent: "A2", Input: "{{prev}}-then", Next: []string{"n3"}},
			"n3": {ID: "n3", Agent: "A3", Input: "{{prev}}-final"},
		},
	}
	res, err := s.Run(context.Background(), g, "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visited) != 3 {
		t.Fatalf("visited %v", res.Visited)
	}
	if !strings.Contains(res.Final, "A3") {
		t.Fatalf("final = %q", res.Final)
	}
	// Verify {{prev}} substitution chained through.
	if !strings.Contains(res.Outputs["n2"], "A1") {
		t.Fatalf("n2 didn't see A1: %q", res.Outputs["n2"])
	}
}

func TestScheduler_OnErrorBranch(t *testing.T) {
	e := env.New()
	defer e.Close()
	agents := []agent.Agent{
		&staticAgent{name: "main", err: errors.New("boom")},
		&staticAgent{name: "fallback", resp: "rescued"},
	}
	s := NewMultiAgent(agents, e)
	g := &Graph{
		Start: "n1",
		Nodes: map[string]Node{
			"n1": {ID: "n1", Agent: "main", Input: "x", OnError: "n2"},
			"n2": {ID: "n2", Agent: "fallback", Input: "y"},
		},
	}
	res, err := s.Run(context.Background(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Final, "rescued") {
		t.Fatalf("final = %q", res.Final)
	}
}

func TestScheduler_ContextCancellation(t *testing.T) {
	e := env.New()
	defer e.Close()
	s := NewMultiAgent([]agent.Agent{&staticAgent{name: "A", resp: "X"}}, e)
	g := &Graph{
		Start: "n1",
		Nodes: map[string]Node{"n1": {ID: "n1", Agent: "A"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Run(ctx, g, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled, got %v", err)
	}
}
