// smoke is a minimal end-to-end check against a real Anthropic-compatible
// endpoint. Reads ANTHROPIC_BASE_URL / ANTHROPIC_AUTH_TOKEN / ANTHROPIC_MODEL.
//
//	go run ./cmd/smoke
//
// Prints the streaming output of one short prompt.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/anthropic"
)

func main() {
	tok := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if tok == "" {
		tok = os.Getenv("ANTHROPIC_API_KEY")
	}
	if tok == "" {
		log.Fatal("ANTHROPIC_AUTH_TOKEN (or ANTHROPIC_API_KEY) required")
	}
	base := os.Getenv("ANTHROPIC_BASE_URL")
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-opus-4-5"
	}

	fmt.Printf("→ POST %s/v1/messages   model=%s\n\n", base, model)

	client := anthropic.New(anthropic.Config{
		APIKey:  tok,
		BaseURL: base,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	ch, err := client.Stream(ctx, llm.Request{
		Model:     model,
		MaxTokens: 100,
		Messages: []llm.Message{
			{Role: "user", Content: "Reply in exactly five words."},
		},
	})
	if err != nil {
		log.Fatalf("stream open failed: %v", err)
	}

	var content string
	var usage llm.Usage
	for ev := range ch {
		switch e := ev.(type) {
		case llm.DeltaEvent:
			fmt.Print(e.Content)
			content += e.Content
		case llm.ReasoningDeltaEvent:
			fmt.Printf("[thinking: %s]", e.Content)
		case llm.ToolUseEvent:
			fmt.Printf("[tool_use: %s]", e.Call.Name)
		case llm.FinishEvent:
			usage = e.Usage
		case llm.ErrorEvent:
			fmt.Printf("\nERROR: %v\n", e.Err)
			os.Exit(1)
		}
	}
	fmt.Printf("\n\n✓ stream OK · %v · %d/%d tokens (prompt/completion)\n",
		time.Since(start).Round(time.Millisecond), usage.PromptTokens, usage.CompletionTokens)
}
