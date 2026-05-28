// Example: engineer — single-agent code generator with file + shell tools.
//
// Port of aievo's examples/single-agent-example/engineer/main.go (~70 lines).
// This port is ~50 lines and adds:
//   - Parallel safe tools (FileRead) when the model fetches multiple files
//   - Explicit ctx with Ctrl-C cancellation
//   - Event bus subscriber that streams progress to stdout
//
// Run:
//
//	OPENAI_API_KEY=sk-... OPENAI_MODEL=gpt-4o-mini \
//	  go run ./examples/engineer
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/event"
	"github.com/samson-samson/aievo-next/pkg/llmenv"
	"github.com/samson-samson/aievo-next/pkg/tool"
	"github.com/samson-samson/aievo-next/pkg/tool/builtin"
)

const systemPrompt = `You are a senior Python engineer. The user will give you a coding task.

Use the available tools to:
  - FileWrite to create source files
  - FileRead to inspect existing files (you may call FileRead in parallel; the runtime executes safe tools concurrently)
  - Bash to install dependencies, run tests, etc.

Always finish by summarizing what you built and how to run it. Be concise.`

func main() {
	resolved, err := llmenv.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Workspace lives next to the example so the model can write into it
	// without disturbing the rest of the repo.
	cwd, _ := os.Getwd()
	workspace := filepath.Join(cwd, "examples", "engineer", "workspace")
	_ = os.MkdirAll(workspace, 0o755)

	e := env.New()
	defer e.Close()
	go logEvents(e.Bus.Subscribe(64))

	a := agent.NewReAct(agent.ReActConfig{
		Name:   "engineer",
		LLM:    resolved.Client,
		Model:  resolved.Model,
		System: agent.StaticSystemPrompt(systemPrompt),
		Tools: []tool.Tool{
			builtin.FileRead{},
			builtin.FileWrite{},
			builtin.Bash{Workdir: workspace},
		},
		MaxIterations: 20,
		Temperature:   0.1,
		Bus:           e.Bus,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	res, err := a.Run(ctx, "Write a snake game in Python using pygame. Save to snake.py.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== FINAL ===")
	fmt.Println(res.Final)
	fmt.Printf("\nUsage: %d prompt / %d completion tokens · %d tool calls\n",
		res.Usage.PromptTokens, res.Usage.CompletionTokens, len(res.Calls))
}

func logEvents(ch <-chan event.Event) {
	for ev := range ch {
		switch e := ev.(type) {
		case event.ToolCallEvent:
			for _, c := range e.Calls {
				fmt.Printf("→ tool %s: %s\n", c.Name, c.Input)
			}
		case event.TerminalEvent:
			fmt.Printf("⏹ %s: %s\n", e.Agent, e.Reason)
		}
	}
}
