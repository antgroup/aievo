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

func canned(text string) llm.LLM {
	return mock.NewMock().ExpectAndRespond(mock.Script{Delta: text, Finish: llm.FinishStop})
}

func TestPaperWriteSOP_WithMockLLM(t *testing.T) {
	e := env.New()
	defer e.Close()
	mk := func(name, response string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name: name, LLM: canned(response), Model: "mock",
			MaxIterations: 2,
		})
	}
	agents := []agent.Agent{
		mk("RTA", "[questions Q1 Q2 Q3]"),
		mk("LROA", "[lit review found A, B, C]"),
		mk("OGA", "[outline S1 S2 S3]"),
		mk("CGA", "[draft body]"),
		mk("PPA", "[polished paper]"),
	}
	sched := scheduler.NewMultiAgent(agents, e)
	g := &scheduler.Graph{
		Start: "rta",
		Nodes: map[string]scheduler.Node{
			"rta":  {ID: "rta", Agent: "RTA", Input: "topic", Next: []string{"lroa"}},
			"lroa": {ID: "lroa", Agent: "LROA", Input: "{{prev}}", Next: []string{"oga"}},
			"oga":  {ID: "oga", Agent: "OGA", Input: "{{prev}}", Next: []string{"cga"}},
			"cga":  {ID: "cga", Agent: "CGA", Input: "{{prev}}", Next: []string{"ppa"}},
			"ppa":  {ID: "ppa", Agent: "PPA", Input: "{{prev}}"},
		},
	}
	res, err := sched.Run(context.Background(), g, "topic")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visited) != 5 || !strings.Contains(res.Final, "polished") {
		t.Fatalf("visited=%v final=%q", res.Visited, res.Final)
	}
}
