// Example: paper-write — 5-agent academic-paper SOP (RTA / LROA / OGA / CGA / PPA).
//
// Port of aievo's examples/multi-agent-example/paper_write. The original
// chains five role agents:
//
//	RTA   Research Topic Analyser
//	LROA  Literature Review & Outline Agent (uses WebSearch)
//	OGA   Outline Generator Agent
//	CGA   Content Generator Agent
//	PPA   Polish & Proofread Agent
//
// We map each role to a ReAct agent and link them in a linear scheduler.Graph.
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
	"github.com/samson-samson/aievo-next/pkg/tool"
	"github.com/samson-samson/aievo-next/pkg/tool/builtin"
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

	mk := func(name, role string, tools []tool.Tool) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name:          name,
			LLM:           client,
			Model:         model,
			System:        agent.StaticSystemPrompt(role),
			Tools:         tools,
			MaxIterations: 6,
			Temperature:   0.3,
			Bus:           e.Bus,
		})
	}

	agents := []agent.Agent{
		mk("RTA", "You analyse the user's research topic and propose 3-5 key questions.", nil),
		mk("LROA", "Given key questions, search literature (use WebFetch / WebSearch if needed) and produce a one-paragraph literature review.",
			[]tool.Tool{builtin.WebFetch{}, builtin.WebSearch{}}),
		mk("OGA", "Convert the topic + literature review into a section outline (5-7 sections).", nil),
		mk("CGA", "Write one paragraph per outline section. Be concrete; cite no sources.", nil),
		mk("PPA", "Polish the draft for clarity, fix grammar, and produce the final paper text.", nil),
	}

	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "rta",
		Nodes: map[string]scheduler.Node{
			"rta":  {ID: "rta", Agent: "RTA", Input: "Topic: {{prev}}", Next: []string{"lroa"}},
			"lroa": {ID: "lroa", Agent: "LROA", Input: "Questions to research:\n{{prev}}", Next: []string{"oga"}},
			"oga":  {ID: "oga", Agent: "OGA", Input: "Literature review:\n{{prev}}", Next: []string{"cga"}},
			"cga":  {ID: "cga", Agent: "CGA", Input: "Outline:\n{{prev}}", Next: []string{"ppa"}},
			"ppa":  {ID: "ppa", Agent: "PPA", Input: "Draft:\n{{prev}}"},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	res, err := sched.Run(ctx, g, "Efficient multi-agent collaboration via shared message queues")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== FINAL PAPER ===\n", res.Final)
}
