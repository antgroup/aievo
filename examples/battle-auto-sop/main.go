// Example: battle-auto-sop — Host + Alice/Bob + 3 experts debate "climate change".
// Port of examples/multi-agent-example/battle-with-auto-sop.
//
// Original used a SopAgent to auto-generate the procedure. v0.1 of aievo-next
// uses an explicit hand-crafted Graph; LLM-driven SOP synthesis is on the
// v0.2 roadmap (the input to scheduler.Run is just a *Graph value, so any
// future SopAgent could simply emit one).
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
		mk("Host", "You are the host of a debate on climate change. Open the round with the motion and invite Alice (affirmative) to speak.", 0.3),
		mk("Alice", "You are Alice, arguing the *affirmative* side. Make one strong concise argument (<80 words) in response to the host.", 0.7),
		mk("Bob", "You are Bob, arguing the *negative* side. Rebut Alice with one strong concise argument (<80 words).", 0.7),
		mk("Expert1", "You are an expert moderator. Comment on both arguments in one sentence, naming the stronger side.", 0.4),
		mk("Expert2", "You are an expert ethicist. Comment in one sentence focused on moral implications.", 0.4),
		mk("Expert3", "You are an expert economist. Comment in one sentence focused on cost/benefit.", 0.4),
	}
	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "host",
		Nodes: map[string]scheduler.Node{
			"host":    {ID: "host", Agent: "Host", Input: "Topic: climate change.", Next: []string{"alice"}},
			"alice":   {ID: "alice", Agent: "Alice", Input: "Host said:\n{{prev}}", Next: []string{"bob"}},
			"bob":     {ID: "bob", Agent: "Bob", Input: "Alice said:\n{{prev}}", Next: []string{"expert1"}},
			"expert1": {ID: "expert1", Agent: "Expert1", Input: "Arguments so far:\n{{prev}}", Next: []string{"expert2"}},
			"expert2": {ID: "expert2", Agent: "Expert2", Input: "Prior comment:\n{{prev}}", Next: []string{"expert3"}},
			"expert3": {ID: "expert3", Agent: "Expert3", Input: "Prior comment:\n{{prev}}"},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	res, err := sched.Run(ctx, g, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== DEBATE TRACE ===")
	for _, id := range res.Visited {
		fmt.Printf("[%s]\n%s\n\n", id, res.Outputs[id])
	}
}
