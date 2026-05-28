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

func TestBattleCustomSOP_WithMockLLM(t *testing.T) {
	e := env.New()
	defer e.Close()
	mk := func(n, r string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{Name: n, LLM: canned(r), Model: "mock", MaxIterations: 2})
	}
	agents := []agent.Agent{
		mk("Host", "host"), mk("Alice", "aff"), mk("Bob", "neg"),
		mk("Expert1", "e1"), mk("Expert2", "e2"), mk("Expert3", "e3"),
	}
	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "open",
		Nodes: map[string]scheduler.Node{
			"open":     {ID: "open", Agent: "Host", Next: []string{"r1_alice"}},
			"r1_alice": {ID: "r1_alice", Agent: "Alice", Input: "{{prev}}", Next: []string{"r1_bob"}},
			"r1_bob":   {ID: "r1_bob", Agent: "Bob", Input: "{{prev}}", Next: []string{"summary1"}},
			"summary1": {ID: "summary1", Agent: "Host", Input: "{{prev}}", Next: []string{"r2_alice"}},
			"r2_alice": {ID: "r2_alice", Agent: "Alice", Input: "{{prev}}", Next: []string{"r2_bob"}},
			"r2_bob":   {ID: "r2_bob", Agent: "Bob", Input: "{{prev}}", Next: []string{"e1"}},
			"e1":       {ID: "e1", Agent: "Expert1", Input: "{{prev}}", Next: []string{"e2"}},
			"e2":       {ID: "e2", Agent: "Expert2", Input: "{{prev}}", Next: []string{"e3"}},
			"e3":       {ID: "e3", Agent: "Expert3", Input: "{{prev}}"},
		},
	}
	res, err := sched.Run(context.Background(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visited) != 9 {
		t.Fatalf("visited=%v", res.Visited)
	}
}
