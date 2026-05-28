// Example: undercover — minimal multi-agent SOP demo.
//
// Port of aievo's examples/multi-agent-example/undercover-with-auto-sop
// (~150 lines + SopAgent + WatcherAgent dependencies = ~600 LOC total).
//
// We use an explicit hand-built Graph instead of an "auto-SOP" agent for
// v0.1 — this makes the orchestration legible (no LLM-generated control
// flow) and avoids depending on yet-to-be-ported SopAgent / WatcherAgent.
//
// Run:
//
//	OPENAI_API_KEY=sk-... OPENAI_MODEL=gpt-4o-mini \
//	  go run ./examples/undercover-auto-sop
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/event"
	"github.com/samson-samson/aievo-next/pkg/llmenv"
	"github.com/samson-samson/aievo-next/pkg/scheduler"
)

const gameMasterPrompt = `You are the Game Master of "Who is the Undercover".
There are 4 players: 3 civilians (assigned the word "apple") and 1 undercover ("orange").
Announce the start of round 1 and ask each player to describe their word in one sentence
without saying it. Be concise.`

const civilianPrompt = `You are a civilian whose secret word is "apple". When asked to
describe it, give ONE short sentence that hints at the word without saying it.`

const undercoverPrompt = `You are the undercover whose secret word is "orange". When
asked to describe it, give ONE short sentence that loosely mimics what a civilian
holding the word "apple" might say. Don't reveal "orange".`

const judgePrompt = `You are a neutral judge. Given each player's description below,
identify who is most likely the undercover and explain in one paragraph. Be decisive.`

func main() {
	resolved, err := llmenv.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	e := env.New()
	defer e.Close()
	go logEvents(e.Bus.Subscribe(128))

	make := func(name, sys string, temp float32) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name:          name,
			LLM:           resolved.Client,
			Model:         resolved.Model,
			System:        agent.StaticSystemPrompt(sys),
			MaxIterations: 4,
			Temperature:   temp,
			Bus:           e.Bus,
		})
	}

	agents := []agent.Agent{
		make("GameMaster", gameMasterPrompt, 0.3),
		make("CivilianA", civilianPrompt, 0.7),
		make("CivilianB", civilianPrompt, 0.7),
		make("CivilianC", civilianPrompt, 0.7),
		make("Undercover", undercoverPrompt, 0.7),
		make("Judge", judgePrompt, 0.2),
	}
	sched := scheduler.NewMultiAgent(agents, e)

	// Hand-built SOP. Each player describes after the GameMaster announces;
	// then the Judge weighs in. {{prev}} is substituted with the prior
	// node's output to give each player the running context.
	graph := &scheduler.Graph{
		Start: "intro",
		Nodes: map[string]scheduler.Node{
			"intro":   {ID: "intro", Agent: "GameMaster", Input: "Start the game and ask each player.", Next: []string{"pa"}},
			"pa":      {ID: "pa", Agent: "CivilianA", Input: "Describe your word now.\n\nContext: {{prev}}", Next: []string{"pb"}},
			"pb":      {ID: "pb", Agent: "CivilianB", Input: "Describe your word now.\n\nContext: {{prev}}", Next: []string{"pc"}},
			"pc":      {ID: "pc", Agent: "CivilianC", Input: "Describe your word now.\n\nContext: {{prev}}", Next: []string{"under"}},
			"under":   {ID: "under", Agent: "Undercover", Input: "Describe your word now.\n\nContext: {{prev}}", Next: []string{"verdict"}},
			"verdict": {ID: "verdict", Agent: "Judge", Input: "Players' descriptions:\n{{prev}}\n\nWho is the undercover?"},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	res, err := sched.Run(ctx, graph, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== VERDICT ===")
	fmt.Println(res.Final)
	fmt.Println("\n=== TRACE ===")
	for _, id := range res.Visited {
		fmt.Printf("[%s]\n%s\n\n", id, res.Outputs[id])
	}
}

func logEvents(ch <-chan event.Event) {
	for ev := range ch {
		if m, ok := ev.(event.MessageEvent); ok {
			fmt.Printf("  · %s: %.80s\n", m.Sender, m.Content)
		}
	}
}
