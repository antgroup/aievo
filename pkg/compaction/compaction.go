// Package compaction shrinks a conversation when its token count would
// otherwise overflow the model's context. Loosely modelled on Claude Code's
// QueryEngine.compactIfNeeded path (book ch. 3).
//
// Strategy:
//
//	1. EstimateTokens — rough char/4 heuristic; replace with a real tokenizer
//	   if precision matters.
//	2. Compactor.Compact — if estimate exceeds budget, summarise everything
//	   but the system prompt and the most-recent K messages into one
//	   "[compacted]" assistant note.
//
// Summarisation requires an LLM call; the caller injects one via the Summariser
// function so compaction can be cheap-modelled (e.g. gpt-4o-mini) while the
// host agent uses a stronger model.
package compaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/samson-samson/aievo-next/pkg/llm"
)

// EstimateTokens is the cheap fallback estimator. Returns char/4 — within
// 25% for English. Override at the call site if better precision is needed.
func EstimateTokens(messages []llm.Message) int {
	n := 0
	for _, m := range messages {
		n += len(m.Content) / 4
		for _, tc := range m.ToolCalls {
			n += (len(tc.Name) + len(tc.Input)) / 4
		}
	}
	return n
}

// Summariser produces a 1-paragraph distillation of older messages.
type Summariser func(ctx context.Context, msgs []llm.Message) (string, error)

// Compactor configures one compaction policy.
type Compactor struct {
	// MaxTokens is the budget. When EstimateTokens exceeds this, compaction
	// triggers. Choose ~70% of model context to leave room for the response.
	MaxTokens int

	// KeepRecent messages are kept verbatim at the tail (so the model sees
	// recent tool calls + reflections). Default 6.
	KeepRecent int

	// Summarise distils everything past KeepRecent into a single note.
	Summarise Summariser
}

// Compact returns a (potentially) shorter Messages slice. If under budget,
// returns msgs unchanged. The system prompt (index 0 with Role=="system")
// is always preserved verbatim.
func (c Compactor) Compact(ctx context.Context, msgs []llm.Message) ([]llm.Message, error) {
	if c.MaxTokens == 0 {
		c.MaxTokens = 32000
	}
	if c.KeepRecent == 0 {
		c.KeepRecent = 6
	}
	if EstimateTokens(msgs) <= c.MaxTokens {
		return msgs, nil
	}
	if c.Summarise == nil {
		return msgs, fmt.Errorf("compaction: over budget but no Summariser configured")
	}

	// Identify the system prefix (zero or more leading system messages).
	systemEnd := 0
	for systemEnd < len(msgs) && msgs[systemEnd].Role == "system" {
		systemEnd++
	}
	// Everything between systemEnd and tail is candidate for summary.
	tailStart := len(msgs) - c.KeepRecent
	if tailStart <= systemEnd {
		return msgs, nil // nothing to summarise
	}
	toSummarise := msgs[systemEnd:tailStart]
	summary, err := c.Summarise(ctx, toSummarise)
	if err != nil {
		return msgs, err
	}

	out := make([]llm.Message, 0, systemEnd+1+c.KeepRecent)
	out = append(out, msgs[:systemEnd]...)
	out = append(out, llm.Message{
		Role:    "assistant",
		Content: "[compacted earlier turns]\n\n" + strings.TrimSpace(summary),
	})
	out = append(out, msgs[tailStart:]...)
	return out, nil
}

// DefaultSummariser builds a Summariser from an LLM client. The summary
// prompt is fixed and the model is told to be brief.
func DefaultSummariser(client llm.LLM, model string) Summariser {
	return func(ctx context.Context, msgs []llm.Message) (string, error) {
		var b strings.Builder
		for _, m := range msgs {
			fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
		}
		req := llm.Request{
			Model: model,
			Messages: []llm.Message{
				{Role: "system", Content: "Summarise the conversation below in 1-2 paragraphs. Keep concrete facts, file paths, names, decisions. Drop conversational filler."},
				{Role: "user", Content: b.String()},
			},
			MaxTokens:   600,
			Temperature: 0,
		}
		ch, err := client.Stream(ctx, req)
		if err != nil {
			return "", err
		}
		var out strings.Builder
		for ev := range ch {
			if d, ok := ev.(llm.DeltaEvent); ok {
				out.WriteString(d.Content)
			}
			if e, ok := ev.(llm.ErrorEvent); ok {
				return "", e.Err
			}
		}
		return out.String(), nil
	}
}
