package agent

import (
	"context"
	stderrors "errors"
	"strings"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/event"
	"github.com/samson-samson/aievo-next/pkg/featureflag"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/queryloop"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// ReActConfig holds wiring for a ReActAgent.
type ReActConfig struct {
	Name          string
	LLM           llm.LLM
	Model         string
	Tools         []tool.Tool
	System        SystemPromptFn
	MaxIterations int
	Bus           *event.Bus           // optional; if nil no events are published
	ToolScheduler tool.SchedulerConfig // limits / parallelism for tool calls
	Temperature   float32
}

// ReActAgent is the canonical Plan/Act/Reflect driver. It implements both
// Agent (consumer-facing) and queryloop.Driver (loop internals).
type ReActAgent struct {
	cfg      ReActConfig
	messages []llm.Message
	usage    llm.Usage
	calls    []toolLog
}

// NewReAct constructs an agent. The configuration is validated lazily on
// the first Run; callers can build agents during program startup without
// needing API keys.
func NewReAct(cfg ReActConfig) *ReActAgent {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 16
	}
	if cfg.System == nil {
		cfg.System = StaticSystemPrompt("You are a helpful assistant.")
	}
	return &ReActAgent{cfg: cfg}
}

func (a *ReActAgent) Name() string { return a.cfg.Name }

// Run resets per-call state and drives the queryloop to completion.
func (a *ReActAgent) Run(ctx context.Context, input string) (Result, error) {
	if a.cfg.LLM == nil {
		return Result{}, aierrs.NewFatal(stderrors.New("ReActAgent: LLM is nil"))
	}
	a.messages = []llm.Message{
		{Role: "system", Content: a.cfg.System(ctx)},
		{Role: "user", Content: input},
	}
	a.usage = llm.Usage{}
	a.calls = nil

	loop := queryloop.New(a, queryloop.Config{
		MaxIterations: a.cfg.MaxIterations,
		AgentName:     a.cfg.Name,
		Bus:           a.cfg.Bus,
	})
	step, err := loop.Run(ctx)
	res := Result{
		Final: step.Final,
		Usage: a.usage,
		Steps: int(loop.State()),
		Calls: a.calls,
	}
	if err != nil {
		return res, err
	}
	if res.Final == "" {
		// Some terminations (e.g. max_iter) carry no final string;
		// surface the tail assistant message instead.
		res.Final = lastAssistant(a.messages)
	}
	return res, nil
}

// ----- queryloop.Driver implementation -----

// Plan invokes the LLM once and converts the stream into a Step.
func (a *ReActAgent) Plan(ctx context.Context) queryloop.Step {
	req := llm.Request{
		Model:       a.cfg.Model,
		Messages:    a.messages,
		Tools:       toolSpecsOf(a.cfg.Tools),
		Temperature: a.cfg.Temperature,
	}

	var content strings.Builder
	var calls []llm.ToolCall

	doStream := func(ctx context.Context) error {
		ch, err := a.cfg.LLM.Stream(ctx, req)
		if err != nil {
			return err
		}
		for ev := range ch {
			switch e := ev.(type) {
			case llm.DeltaEvent:
				content.WriteString(e.Content)
				a.emit(ctx, event.NewLLMChunkEvent(a.cfg.Name, e.Content))
			case llm.ReasoningDeltaEvent:
				// Reasoning is not appended to the assistant message (the
				// model already integrated it). Emit for observers.
				a.emit(ctx, event.NewLLMChunkEvent(a.cfg.Name, "[reasoning] "+e.Content))
			case llm.ToolUseEvent:
				calls = append(calls, e.Call)
			case llm.FinishEvent:
				a.usage = e.Usage
			case llm.ErrorEvent:
				return e.Err
			}
		}
		return nil
	}

	var err error
	if featureflag.Enabled(featureflag.FlagAgentRetry) {
		err = aierrs.Retry(ctx, aierrs.DefaultPolicy, doStream)
	} else {
		err = doStream(ctx)
	}
	if err != nil {
		return queryloop.DecideError(err)
	}
	// If ctx was cancelled mid-stream the provider may close the channel
	// silently — surface that instead of treating an empty stream as
	// "model produced no tool calls".
	if err := ctx.Err(); err != nil {
		return queryloop.DecideError(err)
	}

	// Persist assistant turn for the next iteration.
	assistant := llm.Message{
		Role:    "assistant",
		Content: content.String(),
	}
	for _, c := range calls {
		assistant.ToolCalls = append(assistant.ToolCalls, c)
	}
	a.messages = append(a.messages, assistant)

	if len(calls) == 0 {
		return queryloop.DecidePlanToFinish(content.String(), "model produced no tool calls")
	}
	tcs := make([]tool.Call, len(calls))
	for i, c := range calls {
		tcs[i] = tool.Call{ID: c.ID, Name: c.Name, Input: c.Input}
	}
	return queryloop.DecidePlanToAct(tcs)
}

// Act runs the tool calls via the concurrent scheduler.
func (a *ReActAgent) Act(ctx context.Context, calls []tool.Call) queryloop.Step {
	results, err := tool.ExecuteCalls(ctx, calls, a.cfg.Tools, a.cfg.ToolScheduler)
	if err != nil {
		return queryloop.DecideError(err)
	}
	a.emit(ctx, event.NewToolCallEvent(a.cfg.Name, toEventCalls(calls)))
	a.emit(ctx, event.NewToolResultEvent(a.cfg.Name, toEventResults(results)))
	for _, r := range results {
		a.calls = append(a.calls, toolLog{
			Name: r.Name, Input: callInput(calls, r.CallID), Output: r.Output, Err: r.Err,
		})
	}
	return queryloop.DecideActToReflect(results)
}

// Reflect appends tool results as messages and loops.
func (a *ReActAgent) Reflect(ctx context.Context, results []tool.Result) queryloop.Step {
	for _, r := range results {
		content := r.Output
		if r.Err != nil {
			content = "ERROR: " + r.Err.Error()
		}
		a.messages = append(a.messages, llm.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: r.CallID,
		})
	}
	return queryloop.DecideReflectToPlan()
}

// ----- helpers -----

func (a *ReActAgent) emit(ctx context.Context, e event.Event) {
	if a.cfg.Bus != nil {
		a.cfg.Bus.Publish(ctx, e)
	}
}

func toEventCalls(calls []tool.Call) []event.ToolCall {
	out := make([]event.ToolCall, len(calls))
	for i, c := range calls {
		out[i] = event.ToolCall{ID: c.ID, Name: c.Name, Input: c.Input}
	}
	return out
}

func toEventResults(results []tool.Result) []event.ToolResult {
	out := make([]event.ToolResult, len(results))
	for i, r := range results {
		out[i] = event.ToolResult{CallID: r.CallID, Name: r.Name, Output: r.Output, Err: r.Err}
	}
	return out
}

func callInput(calls []tool.Call, id string) string {
	for _, c := range calls {
		if c.ID == id {
			return c.Input
		}
	}
	return ""
}

func lastAssistant(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Content != "" {
			return msgs[i].Content
		}
	}
	return ""
}

// Compile-time guarantees.
var (
	_ Agent             = (*ReActAgent)(nil)
	_ queryloop.Driver  = (*ReActAgent)(nil)
)
