// Integration test: drives the undercover SOP with a mock LLM so the example
// can be CI-tested without burning OpenAI tokens.
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
	"github.com/samson-samson/aievo-next/pkg/scheduler"
)

// staticResponse builds a one-shot mock that returns `text` and finishes.
func staticResponse(text string) llm.LLM {
	return mock.NewMock().ExpectAndRespond(mock.Script{
		Delta:  text,
		Finish: llm.FinishStop,
	})
}

func TestUndercoverExample_WithMockLLM(t *testing.T) {
	e := env.New()
	defer e.Close()

	make := func(name, response string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name:          name,
			LLM:           staticResponse(response),
			Model:         "mock",
			System:        agent.StaticSystemPrompt("You are " + name),
			MaxIterations: 2,
		})
	}

	agents := []agent.Agent{
		make("GameMaster", "Round 1: please describe your word."),
		make("CivilianA", "It's something I eat for breakfast."),
		make("CivilianB", "It is red and round."),
		make("CivilianC", "It grows on trees in cooler climates."),
		make("Undercover", "It is round and grows on trees."),
		make("Judge", "Undercover is most likely the Undercover."),
	}
	sched := scheduler.NewMultiAgent(agents, e)

	graph := &scheduler.Graph{
		Start: "intro",
		Nodes: map[string]scheduler.Node{
			"intro":   {ID: "intro", Agent: "GameMaster", Input: "go", Next: []string{"pa"}},
			"pa":      {ID: "pa", Agent: "CivilianA", Input: "{{prev}}", Next: []string{"pb"}},
			"pb":      {ID: "pb", Agent: "CivilianB", Input: "{{prev}}", Next: []string{"pc"}},
			"pc":      {ID: "pc", Agent: "CivilianC", Input: "{{prev}}", Next: []string{"under"}},
			"under":   {ID: "under", Agent: "Undercover", Input: "{{prev}}", Next: []string{"verdict"}},
			"verdict": {ID: "verdict", Agent: "Judge", Input: "{{prev}}"},
		},
	}
	res, err := sched.Run(context.Background(), graph, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visited) != 6 {
		t.Fatalf("visited %v", res.Visited)
	}
	if !strings.Contains(res.Final, "Undercover") {
		t.Fatalf("Final didn't mention Undercover: %q", res.Final)
	}
}
