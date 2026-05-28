package compaction

import (
	"context"
	"strings"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
)

func TestEstimateTokens(t *testing.T) {
	got := EstimateTokens([]llm.Message{
		{Role: "user", Content: strings.Repeat("a", 400)}, // ~100 tokens
		{Role: "assistant", Content: strings.Repeat("b", 800)}, // ~200 tokens
	})
	if got < 250 || got > 350 {
		t.Fatalf("got=%d outside [250,350]", got)
	}
}

func TestCompact_UnderBudget(t *testing.T) {
	c := Compactor{MaxTokens: 10000}
	msgs := []llm.Message{
		{Role: "system", Content: "you are X"},
		{Role: "user", Content: "hi"},
	}
	out, err := c.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatal("should be unchanged when under budget")
	}
}

func TestCompact_TriggersWhenOver(t *testing.T) {
	mockLLM := mock.NewMock().ExpectAndRespond(mock.Script{
		Delta:  "earlier we discussed X and Y",
		Finish: llm.FinishStop,
	})
	c := Compactor{
		MaxTokens:  20,
		KeepRecent: 1,
		Summarise:  DefaultSummariser(mockLLM, "mock"),
	}
	msgs := []llm.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: strings.Repeat("x", 400)},  // ~100 tokens
		{Role: "assistant", Content: strings.Repeat("y", 400)},
		{Role: "user", Content: strings.Repeat("z", 400)},
		{Role: "assistant", Content: "tail"},
	}
	out, err := c.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 { // system + summary + tail
		t.Fatalf("got %d messages: %+v", len(out), out)
	}
	if !strings.Contains(out[1].Content, "[compacted earlier turns]") {
		t.Fatalf("summary marker missing: %q", out[1].Content)
	}
	if out[2].Content != "tail" {
		t.Fatalf("tail wrong: %q", out[2].Content)
	}
}

func TestCompact_OverButNoSummariser(t *testing.T) {
	c := Compactor{MaxTokens: 1}
	_, err := c.Compact(context.Background(), []llm.Message{
		{Role: "user", Content: strings.Repeat("a", 100)},
	})
	if err == nil {
		t.Fatal("expected error when over budget with no summariser")
	}
}
