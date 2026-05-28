// Example: werewolf — 6 players + God game master, simplified 2-phase round.
//
// Port of examples/multi-agent-example/werewolf. The original uses
// SopAgent + WatcherAgent and conditional message subscription to emulate
// night/day rounds. v0.1 of aievo-next models this as an explicit Graph;
// a SopAgent-style auto generator is on the v0.2 roadmap.
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

type role struct {
	name, kind, prompt string
}

var roles = []role{
	{"Alex", "Werewolf", "You are Alex, a werewolf. During day votes, blend in; coordinate subtly with Bella."},
	{"Bella", "Werewolf", "You are Bella, a werewolf. During day votes, blend in; coordinate subtly with Alex."},
	{"Diana", "Villager", "You are Diana, a villager. Identify werewolves through discussion."},
	{"Ethan", "Villager", "You are Ethan, a villager. Vote out the most suspicious player."},
	{"George", "Seer", "You are George, the seer. Reveal information cautiously to guide villagers."},
	{"Hannah", "Witch", "You are Hannah, the witch. You can save or poison once. Hint without confirming."},
}

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
		mk("God", "You are the Werewolf game master. Announce the current phase and ask the listed player to act/speak. Be brief."),
	}
	for _, r := range roles {
		agents = append(agents, mk(r.name, r.prompt+"\n\nRoles in game: 2 Werewolves, 2 Villagers, 1 Seer, 1 Witch."))
	}
	sched := scheduler.NewMultiAgent(agents, e)

	// Night: God → Werewolves (Alex, Bella) → Seer (George) → Witch (Hannah)
	// Day:   God → all 6 players take turns sharing → God announces vote.
	g := &scheduler.Graph{
		Start: "night_intro",
		Nodes: map[string]scheduler.Node{
			"night_intro":    {ID: "night_intro", Agent: "God", Input: "Night phase begins. Werewolves, choose a target.", Next: []string{"alex_night"}},
			"alex_night":     {ID: "alex_night", Agent: "Alex", Input: "Coordinate the kill with Bella. Pick one villager.", Next: []string{"bella_night"}},
			"bella_night":    {ID: "bella_night", Agent: "Bella", Input: "Alex proposed:\n{{prev}}\nConfirm or counter.", Next: []string{"george_night"}},
			"george_night":   {ID: "george_night", Agent: "George", Input: "Seer's turn. Choose one player to check.", Next: []string{"hannah_night"}},
			"hannah_night":   {ID: "hannah_night", Agent: "Hannah", Input: "Witch's turn. Decide whether to save the night's victim.", Next: []string{"day_intro"}},
			"day_intro":      {ID: "day_intro", Agent: "God", Input: "Day phase. Each player gives a short statement.\nNight summary:\n{{prev}}", Next: []string{"diana_day"}},
			"diana_day":      {ID: "diana_day", Agent: "Diana", Input: "Your statement (1 sentence).", Next: []string{"ethan_day"}},
			"ethan_day":      {ID: "ethan_day", Agent: "Ethan", Input: "Diana said:\n{{prev}}\nYour statement.", Next: []string{"alex_day"}},
			"alex_day":       {ID: "alex_day", Agent: "Alex", Input: "Players so far:\n{{prev}}\nYour statement (blend in!).", Next: []string{"bella_day"}},
			"bella_day":      {ID: "bella_day", Agent: "Bella", Input: "{{prev}}\nYour statement.", Next: []string{"god_vote"}},
			"god_vote":       {ID: "god_vote", Agent: "God", Input: "Tally suspicion based on:\n{{prev}}\nAnnounce who is voted out and the apparent winner."},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	res, err := sched.Run(ctx, g, "")
	if err != nil {
		log.Fatal(err)
	}
	for _, id := range res.Visited {
		fmt.Printf("[%s] %s\n\n", id, res.Outputs[id])
	}
}
