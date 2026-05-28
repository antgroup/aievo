// Example: battle-custom-sop — same lineup as battle-auto-sop, but with a
// 2-round explicit SOP (Host opens → Alice → Bob → Host summarises round 1
// → Alice → Bob → Expert1/2/3 each comment).
//
// Demonstrates that hand-crafted SOPs scale to longer flows without losing
// readability — every node is a single Graph entry.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/llm/openai"
	"github.com/samson-samson/aievo-next/pkg/scheduler"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY required")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	client := openai.New(openai.Config{APIKey: apiKey, BaseURL: os.Getenv("OPENAI_BASE_URL")})

	e := env.New()
	defer e.Close()
	mk := func(name, role string, temp float32) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name: name, LLM: client, Model: model, Temperature: temp,
			System:        agent.StaticSystemPrompt(role),
			MaxIterations: 3, Bus: e.Bus,
		})
	}

	agents := []agent.Agent{
		mk("Host", "Debate host. Run the round per the user's instruction; be brief.", 0.3),
		mk("Alice", "Argue *affirmative* side, brief (<80 words).", 0.7),
		mk("Bob", "Argue *negative* side, brief (<80 words).", 0.7),
		mk("Expert1", "Moderator expert; comment in one sentence.", 0.4),
		mk("Expert2", "Ethicist expert; comment in one sentence.", 0.4),
		mk("Expert3", "Economist expert; comment in one sentence.", 0.4),
	}
	sched := scheduler.NewMultiAgent(agents, e)

	// 2-round explicit SOP. Compare against auto-sop's 6-node linear path.
	g := &scheduler.Graph{
		Start: "open",
		Nodes: map[string]scheduler.Node{
			"open":      {ID: "open", Agent: "Host", Input: "Open the debate on climate change with one sentence.", Next: []string{"r1_alice"}},
			"r1_alice":  {ID: "r1_alice", Agent: "Alice", Input: "Host: {{prev}}\nMake your opening argument.", Next: []string{"r1_bob"}},
			"r1_bob":    {ID: "r1_bob", Agent: "Bob", Input: "Alice: {{prev}}\nRebut.", Next: []string{"summary1"}},
			"summary1":  {ID: "summary1", Agent: "Host", Input: "Round 1 summary in 1 sentence:\n{{prev}}", Next: []string{"r2_alice"}},
			"r2_alice":  {ID: "r2_alice", Agent: "Alice", Input: "Round 2. New angle. {{prev}}", Next: []string{"r2_bob"}},
			"r2_bob":    {ID: "r2_bob", Agent: "Bob", Input: "Alice (R2): {{prev}}\nRebut.", Next: []string{"e1"}},
			"e1":        {ID: "e1", Agent: "Expert1", Input: "Whole debate so far:\n{{prev}}", Next: []string{"e2"}},
			"e2":        {ID: "e2", Agent: "Expert2", Input: "{{prev}}", Next: []string{"e3"}},
			"e3":        {ID: "e3", Agent: "Expert3", Input: "{{prev}}"},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	res, err := sched.Run(ctx, g, "")
	if err != nil {
		log.Fatal(err)
	}
	for _, id := range res.Visited {
		fmt.Printf("[%s] %s\n", id, res.Outputs[id])
	}
}
