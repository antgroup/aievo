// Example: undercover-custom-sop — explicit 2-round version of the
// undercover game, mirroring examples/multi-agent-example/undercover-with-custom-sop
// from aievo.
//
// Players: GameMaster + 3 civilians (Alice/Bob/David, word=apple) + 1
// undercover (Cathy, word=orange). After two rounds of descriptions, the
// GameMaster announces a vote.
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
	mk := func(name, sys string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name: name, LLM: client, Model: model, Temperature: 0.7,
			System:        agent.StaticSystemPrompt(sys),
			MaxIterations: 3, Bus: e.Bus,
		})
	}
	agents := []agent.Agent{
		mk("GameMaster", "You are the GameMaster of Who-is-the-Undercover. Announce phases and ask players one at a time."),
		mk("Alice", "You are Alice (civilian, word=apple). Describe in one sentence without saying the word."),
		mk("Bob", "You are Bob (civilian, word=apple). Describe in one sentence without saying the word."),
		mk("David", "You are David (civilian, word=apple). Describe in one sentence without saying the word."),
		mk("Cathy", "You are Cathy (undercover, word=orange). Describe in one sentence loosely mimicking what 'apple' players would say."),
	}
	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "r1_intro",
		Nodes: map[string]scheduler.Node{
			"r1_intro":  {ID: "r1_intro", Agent: "GameMaster", Input: "Round 1: each player describe your word.", Next: []string{"r1_alice"}},
			"r1_alice":  {ID: "r1_alice", Agent: "Alice", Input: "{{prev}}", Next: []string{"r1_bob"}},
			"r1_bob":    {ID: "r1_bob", Agent: "Bob", Input: "{{prev}}", Next: []string{"r1_david"}},
			"r1_david":  {ID: "r1_david", Agent: "David", Input: "{{prev}}", Next: []string{"r1_cathy"}},
			"r1_cathy":  {ID: "r1_cathy", Agent: "Cathy", Input: "{{prev}}", Next: []string{"r2_intro"}},
			"r2_intro":  {ID: "r2_intro", Agent: "GameMaster", Input: "Round 2: another descriptor.\nR1:\n{{prev}}", Next: []string{"r2_alice"}},
			"r2_alice":  {ID: "r2_alice", Agent: "Alice", Input: "{{prev}}", Next: []string{"r2_bob"}},
			"r2_bob":    {ID: "r2_bob", Agent: "Bob", Input: "{{prev}}", Next: []string{"r2_david"}},
			"r2_david":  {ID: "r2_david", Agent: "David", Input: "{{prev}}", Next: []string{"r2_cathy"}},
			"r2_cathy":  {ID: "r2_cathy", Agent: "Cathy", Input: "{{prev}}", Next: []string{"verdict"}},
			"verdict":   {ID: "verdict", Agent: "GameMaster", Input: "Based on the two rounds:\n{{prev}}\nVote out the most likely undercover and explain in one paragraph."},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	res, err := sched.Run(ctx, g, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== VERDICT ===\n", res.Final)
}
