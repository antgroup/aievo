// Integration test: drives the engineer example pipeline with a mock LLM,
// without invoking the real main() (which needs OPENAI_API_KEY).
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
	"github.com/samson-samson/aievo-next/pkg/tool"
	"github.com/samson-samson/aievo-next/pkg/tool/builtin"
)

func TestEngineerExample_WithMockLLM(t *testing.T) {
	workspace := t.TempDir()
	out := filepath.Join(workspace, "snake.py")

	// 1) Model decides to FileWrite -> 2) Model summarises.
	mockLLM := mock.NewMock().
		ExpectAndRespond(mock.Script{
			ToolCall: &llm.ToolCall{
				ID:   "c1",
				Name: "FileWrite",
				Input: `{"path":"` + out + `","content":"print('snake')\n"}`,
			},
			Finish: llm.FinishToolCalls,
		}).
		ExpectAndRespond(mock.Script{
			Delta:  "Created snake.py.",
			Finish: llm.FinishStop,
		})

	e := env.New()
	defer e.Close()

	a := agent.NewReAct(agent.ReActConfig{
		Name:   "engineer",
		LLM:    mockLLM,
		Model:  "mock",
		System: agent.StaticSystemPrompt(systemPrompt),
		Tools: []tool.Tool{
			builtin.FileRead{},
			builtin.FileWrite{},
			builtin.Bash{Workdir: workspace},
		},
		MaxIterations: 4,
		Bus:           e.Bus,
	})

	res, err := a.Run(context.Background(), "Write a snake game.")
	if err != nil {
		t.Fatal(err)
	}
	if res.Final != "Created snake.py." {
		t.Fatalf("Final = %q", res.Final)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected file written: %v", err)
	}
}
