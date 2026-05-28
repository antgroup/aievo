package main

import (
	"context"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
	"github.com/samson-samson/aievo-next/pkg/scheduler"
)

func canned(text string) llm.LLM {
	return mock.NewMock().ExpectAndRespond(mock.Script{Delta: text, Finish: llm.FinishStop})
}

func TestBattleAutoSOP_WithMockLLM(t *testing.T) {
	e := env.New()
	defer e.Close()
	mk := func(name, r string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{Name: name, LLM: canned(r), Model: "mock", MaxIterations: 2})
	}
	agents := []agent.Agent{
		mk("Host", "host:open"), mk("Alice", "alice:aff"), mk("Bob", "bob:neg"),
		mk("Expert1", "e1"), mk("Expert2", "e2"), mk("Expert3", "e3"),
	}
	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "host",
		Nodes: map[string]scheduler.Node{
			"host":    {ID: "host", Agent: "Host", Next: []string{"alice"}},
			"alice":   {ID: "alice", Agent: "Alice", Input: "{{prev}}", Next: []string{"bob"}},
			"bob":     {ID: "bob", Agent: "Bob", Input: "{{prev}}", Next: []string{"expert1"}},
			"expert1": {ID: "expert1", Agent: "Expert1", Input: "{{prev}}", Next: []string{"expert2"}},
			"expert2": {ID: "expert2", Agent: "Expert2", Input: "{{prev}}", Next: []string{"expert3"}},
			"expert3": {ID: "expert3", Agent: "Expert3", Input: "{{prev}}"},
		},
	}
	res, err := sched.Run(context.Background(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visited) != 6 {
		t.Fatalf("visited=%v", res.Visited)
	}
}
