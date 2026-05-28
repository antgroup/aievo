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

func TestWerewolfSOP_WithMockLLM(t *testing.T) {
	e := env.New()
	defer e.Close()
	mk := func(n, r string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{Name: n, LLM: canned(r), Model: "mock", MaxIterations: 2})
	}
	agents := []agent.Agent{
		mk("God", "god"),
		mk("Alex", "alex"), mk("Bella", "bella"),
		mk("Diana", "diana"), mk("Ethan", "ethan"),
		mk("George", "george"), mk("Hannah", "hannah"),
	}
	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "n0",
		Nodes: map[string]scheduler.Node{
			"n0":  {ID: "n0", Agent: "God", Next: []string{"n1"}},
			"n1":  {ID: "n1", Agent: "Alex", Input: "{{prev}}", Next: []string{"n2"}},
			"n2":  {ID: "n2", Agent: "Bella", Input: "{{prev}}", Next: []string{"n3"}},
			"n3":  {ID: "n3", Agent: "George", Input: "{{prev}}", Next: []string{"n4"}},
			"n4":  {ID: "n4", Agent: "Hannah", Input: "{{prev}}", Next: []string{"n5"}},
			"n5":  {ID: "n5", Agent: "God", Input: "{{prev}}", Next: []string{"n6"}},
			"n6":  {ID: "n6", Agent: "Diana", Input: "{{prev}}", Next: []string{"n7"}},
			"n7":  {ID: "n7", Agent: "Ethan", Input: "{{prev}}", Next: []string{"n8"}},
			"n8":  {ID: "n8", Agent: "Alex", Input: "{{prev}}", Next: []string{"n9"}},
			"n9":  {ID: "n9", Agent: "Bella", Input: "{{prev}}", Next: []string{"n10"}},
			"n10": {ID: "n10", Agent: "God", Input: "{{prev}}"},
		},
	}
	res, err := sched.Run(context.Background(), g, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visited) != 11 {
		t.Fatalf("visited=%v", res.Visited)
	}
}
