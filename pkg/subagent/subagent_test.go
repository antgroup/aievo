package subagent

import (
	"context"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
)

func TestSubagent_SpawnsAndReturnsFinal(t *testing.T) {
	build := func(task string) (agent.Agent, error) {
		mockLLM := mock.NewMock().ExpectAndRespond(mock.Script{
			Delta:  "child says: " + task,
			Finish: llm.FinishStop,
		})
		return agent.NewReAct(agent.ReActConfig{
			Name:  "child",
			LLM:   mockLLM,
			Model: "mock",
		}), nil
	}
	tool := NewTool(Spec{
		ToolName:    "AgentExplore",
		Description: "spawn explore agent",
		Build:       build,
	})

	out, err := tool.Call(context.Background(), `{"task":"find X"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "child says: find X" {
		t.Fatalf("got %q", out)
	}
}

func TestSubagent_IsConcurrencySafe(t *testing.T) {
	tl := NewTool(Spec{ToolName: "X", Build: func(string) (agent.Agent, error) { return nil, nil }})
	if !tl.IsConcurrencySafe() {
		t.Fatal("subagent must be concurrency-safe (independent child state)")
	}
}
