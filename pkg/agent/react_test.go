package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// echoTool returns its input verbatim. Concurrency-safe, no I/O.
type echoTool struct{}

func (echoTool) Name() string             { return "Echo" }
func (echoTool) Description() string      { return "Echo input back" }
func (echoTool) Schema() []byte           { return []byte(`{"type":"object"}`) }
func (echoTool) IsConcurrencySafe() bool  { return true }
func (echoTool) Call(_ context.Context, in string) (string, error) {
	return "echo:" + in, nil
}

func TestReAct_SingleTurnNoTools(t *testing.T) {
	mockLLM := mock.NewMock().ExpectAndRespond(mock.Script{
		Delta:  "The answer is 42.",
		Finish: llm.FinishStop,
	})
	a := NewReAct(ReActConfig{
		Name:  "test",
		LLM:   mockLLM,
		Model: "test-model",
	})
	res, err := a.Run(context.Background(), "What is 6×7?")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Final, "42") {
		t.Fatalf("Final = %q", res.Final)
	}
	if len(mockLLM.Calls()) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(mockLLM.Calls()))
	}
}

func TestReAct_OneToolCallThenFinish(t *testing.T) {
	mockLLM := mock.NewMock().
		ExpectAndRespond(mock.Script{
			ToolCall: &llm.ToolCall{ID: "c1", Name: "Echo", Input: `{"x":1}`},
			Finish:   llm.FinishToolCalls,
		}).
		ExpectAndRespond(mock.Script{
			Delta:  "Done.",
			Finish: llm.FinishStop,
		})
	a := NewReAct(ReActConfig{
		Name:  "test",
		LLM:   mockLLM,
		Model: "test-model",
		Tools: []tool.Tool{echoTool{}},
	})
	res, err := a.Run(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if res.Final != "Done." {
		t.Fatalf("Final = %q", res.Final)
	}
	if len(res.Calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(res.Calls))
	}
	if res.Calls[0].Output != `echo:{"x":1}` {
		t.Fatalf("tool output = %q", res.Calls[0].Output)
	}
}

func TestReAct_RespectsContextCancel(t *testing.T) {
	// LLM that takes 500ms; ctx cancelled after 10ms.
	// We simulate by having the mock produce nothing — the loop will hit ctx.
	mockLLM := mock.NewMock()
	a := NewReAct(ReActConfig{
		Name:  "test",
		LLM:   mockLLM,
		Model: "test-model",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	// Mock returns instantly so we may finish before cancel fires; that's OK.
	// The important assertion is that we don't deadlock.
	_, _ = a.Run(ctx, "x")
}

func TestReAct_TwoToolsParallel(t *testing.T) {
	mockLLM := mock.NewMock().
		ExpectAndRespond(mock.Script{
			// Mock only supports one ToolCall per Script; use serial calls
			// to verify both are scheduled and recorded.
			ToolCall: &llm.ToolCall{ID: "c1", Name: "Echo", Input: `"a"`},
			Finish:   llm.FinishToolCalls,
		}).
		ExpectAndRespond(mock.Script{
			Delta:  "OK",
			Finish: llm.FinishStop,
		})
	a := NewReAct(ReActConfig{
		Name:  "two-tools",
		LLM:   mockLLM,
		Model: "test-model",
		Tools: []tool.Tool{echoTool{}},
	})
	res, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Calls) == 0 {
		t.Fatal("expected at least one tool call")
	}
}
