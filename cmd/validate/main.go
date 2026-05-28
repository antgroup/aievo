// validate is a no-API-key smoke test that exercises every core system
// end-to-end with mock LLMs. It's not a benchmark — it's the proof that
// the architecture pieces fit together correctly.
//
// Run with:  go run ./cmd/validate
//
// The output is organised as numbered checks. Any "FAIL" line is a real
// problem; "OK" lines are validated invariants.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/samson-samson/aievo-next/pkg/agent"
	"github.com/samson-samson/aievo-next/pkg/env"
	"github.com/samson-samson/aievo-next/pkg/event"
	"github.com/samson-samson/aievo-next/pkg/featureflag"
	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/mock"
	"github.com/samson-samson/aievo-next/pkg/permission"
	"github.com/samson-samson/aievo-next/pkg/queryloop"
	"github.com/samson-samson/aievo-next/pkg/scheduler"
	"github.com/samson-samson/aievo-next/pkg/skill"
	"github.com/samson-samson/aievo-next/pkg/subagent"
	"github.com/samson-samson/aievo-next/pkg/tool"
	"github.com/samson-samson/aievo-next/pkg/tool/builtin"
)

var (
	passed int32
	failed int32
)

func check(name string, ok bool, detail string) {
	if ok {
		atomic.AddInt32(&passed, 1)
		fmt.Printf("  \033[32m✓\033[0m %s\n", name)
	} else {
		atomic.AddInt32(&failed, 1)
		fmt.Printf("  \033[31m✗\033[0m %s — %s\n", name, detail)
	}
}

func main() {
	fmt.Println("aievo-next architecture validation (no API key required)")

	check1_ReActWithMockLLM()
	check2_ToolParallelDispatch()
	check3_ContextCancellation()
	check4_PermissionGate()
	check5_StateMachineTransitions()
	check6_TypedEventFanout()
	check7_SubagentIsolation()
	check8_SchedulerSOP()
	check9_SkillLoader()
	check10_TaskTrackerAndPlanMode()

	fmt.Printf("\n=========================================\n")
	fmt.Printf("Result: \033[32m%d passed\033[0m  \033[31m%d failed\033[0m\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// ---------- Checks ----------

func check1_ReActWithMockLLM() {
	fmt.Println("\n[1] ReAct: mock LLM → tool call → reflect → final")
	mockLLM := mock.NewMock().
		ExpectAndRespond(mock.Script{
			ToolCall: &llm.ToolCall{ID: "c1", Name: "FileRead", Input: `{"path":"/tmp/nothing"}`},
			Finish:   llm.FinishToolCalls,
		}).
		ExpectAndRespond(mock.Script{
			Delta:  "I read the file successfully.",
			Finish: llm.FinishStop,
		})
	tmp := filepath.Join(os.TempDir(), "aievo-validate")
	_ = os.MkdirAll(tmp, 0o755)
	path := filepath.Join(tmp, "x.txt")
	_ = os.WriteFile(path, []byte("hello"), 0o644)

	a := agent.NewReAct(agent.ReActConfig{
		Name:  "tester",
		LLM:   mockLLM,
		Model: "mock",
		Tools: []tool.Tool{builtin.FileRead{}},
	})
	res, err := a.Run(context.Background(), "read it")
	check("agent runs end-to-end", err == nil, fmt.Sprintf("%v", err))
	check("final message present", strings.Contains(res.Final, "read the file"), res.Final)
	check("tool call recorded", len(res.Calls) == 1, fmt.Sprintf("got %d calls", len(res.Calls)))
	check("LLM called twice (plan + reflect→plan→finish)", len(mockLLM.Calls()) == 2, fmt.Sprintf("got %d", len(mockLLM.Calls())))
}

func check2_ToolParallelDispatch() {
	fmt.Println("\n[2] Tool scheduler: 4 safe calls run concurrently")
	featureflag.Set(featureflag.FlagParallelTools, true)
	defer featureflag.Reload()

	slow := &slowTool{name: "Read", d: 80 * time.Millisecond, safe: true}
	calls := []tool.Call{{Name: "Read", ID: "1"}, {Name: "Read", ID: "2"}, {Name: "Read", ID: "3"}, {Name: "Read", ID: "4"}}
	start := time.Now()
	results, _ := tool.ExecuteCalls(context.Background(), calls, []tool.Tool{slow}, tool.SchedulerConfig{MaxParallel: 4})
	elapsed := time.Since(start)

	check("4 results returned in model order",
		len(results) == 4 && results[0].CallID == "1" && results[3].CallID == "4",
		fmt.Sprintf("got %d in order %v", len(results), idsOf(results)))
	check("parallel speedup observed (4×80ms expected ~80ms, serial ~320ms)",
		elapsed < 200*time.Millisecond,
		fmt.Sprintf("elapsed %v", elapsed))

	// And the reverse — flag off should serialize.
	featureflag.Set(featureflag.FlagParallelTools, false)
	start = time.Now()
	_, _ = tool.ExecuteCalls(context.Background(), calls, []tool.Tool{slow}, tool.SchedulerConfig{MaxParallel: 4})
	elapsedSerial := time.Since(start)
	check("flag-off serialises (>= 4×80ms)", elapsedSerial > 280*time.Millisecond, fmt.Sprintf("elapsed %v", elapsedSerial))
}

func check3_ContextCancellation() {
	fmt.Println("\n[3] Context cancellation: agent stops within budget")
	// Mock returns nothing; the loop will spin until ctx times out.
	a := agent.NewReAct(agent.ReActConfig{
		Name:          "blocker",
		LLM:           &blockingLLM{d: 10 * time.Second},
		Model:         "mock",
		MaxIterations: 20,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := a.Run(ctx, "hang please")
	elapsed := time.Since(start)
	check("ctx cancellation surfaces as error", err != nil, "no error")
	check("agent exited within 500ms of deadline", elapsed < 500*time.Millisecond, fmt.Sprintf("took %v", elapsed))
}

func check4_PermissionGate() {
	fmt.Println("\n[4] Permission modes: plan denies writes, bypass allows all")
	fw := builtin.FileWrite{}
	fr := builtin.FileRead{}

	check("plan denies FileWrite", permission.Gate(permission.ModePlan, fw) == permission.Deny, "")
	check("plan allows FileRead", permission.Gate(permission.ModePlan, fr) == permission.Allow, "")
	check("acceptEdits approves FileWrite", permission.Gate(permission.ModeAcceptEdits, fw) == permission.Allow, "")
	check("default asks for Bash", permission.Gate(permission.ModeDefault, builtin.Bash{}) == permission.AskUser, "")
	check("bypass allows Bash", permission.Gate(permission.ModeBypass, builtin.Bash{}) == permission.Allow, "")
}

func check5_StateMachineTransitions() {
	fmt.Println("\n[5] QueryLoop state machine: legal vs illegal transitions")
	legal := []struct{ a, b queryloop.State }{
		{queryloop.StatePending, queryloop.StatePlanning},
		{queryloop.StatePlanning, queryloop.StateActing},
		{queryloop.StateActing, queryloop.StateReflecting},
		{queryloop.StateReflecting, queryloop.StatePlanning},
		{queryloop.StateReflecting, queryloop.StateTerminalOK},
	}
	illegal := []struct{ a, b queryloop.State }{
		{queryloop.StatePending, queryloop.StateActing},
		{queryloop.StatePlanning, queryloop.StateReflecting},
		{queryloop.StateActing, queryloop.StatePlanning},
	}
	for _, p := range legal {
		check(fmt.Sprintf("legal: %s → %s", p.a, p.b), queryloop.IsLegal(p.a, p.b), "")
	}
	for _, p := range illegal {
		check(fmt.Sprintf("illegal: %s → %s rejected", p.a, p.b), !queryloop.IsLegal(p.a, p.b), "")
	}
}

func check6_TypedEventFanout() {
	fmt.Println("\n[6] Event bus: fan-out with slow consumer protection")
	bus := event.NewBus()
	defer bus.Close()
	a := bus.Subscribe(8)
	_ = bus.Subscribe(1) // slow / never read

	for i := 0; i < 100; i++ {
		bus.Publish(context.Background(), event.NewAgentStartedEvent("x", i))
	}
	got := 0
	deadline := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case _, ok := <-a:
			if !ok {
				break loop
			}
			got++
		case <-deadline:
			break loop
		}
	}
	check("active subscriber received events", got > 0, fmt.Sprintf("got %d", got))
	check("slow subscriber didn't block publisher (no deadlock — we got here)", true, "")
}

func check7_SubagentIsolation() {
	fmt.Println("\n[7] Subagent: parent invokes child, child has own context")
	childCalls := 0
	build := func(task string) (agent.Agent, error) {
		childCalls++
		mockLLM := mock.NewMock().ExpectAndRespond(mock.Script{
			Delta:  fmt.Sprintf("child reply to: %s", task),
			Finish: llm.FinishStop,
		})
		return agent.NewReAct(agent.ReActConfig{Name: "child", LLM: mockLLM, Model: "mock"}), nil
	}
	sub := subagent.NewTool(subagent.Spec{
		ToolName: "AgentExplore", Description: "explore", Build: build,
	})
	out, err := sub.Call(context.Background(), `{"task":"find X"}`)
	check("subagent invoked", err == nil && childCalls == 1, fmt.Sprintf("err=%v calls=%d", err, childCalls))
	check("child output flows back", strings.Contains(out, "find X"), out)
	check("subagent is concurrency-safe", sub.IsConcurrencySafe(), "")
}

func check8_SchedulerSOP() {
	fmt.Println("\n[8] Multi-agent SOP: 5-node chain with {{prev}} substitution")
	e := env.New()
	defer e.Close()
	mk := func(name, resp string) agent.Agent {
		return agent.NewReAct(agent.ReActConfig{
			Name:  name,
			LLM:   mock.NewMock().ExpectAndRespond(mock.Script{Delta: resp, Finish: llm.FinishStop}),
			Model: "mock",
		})
	}
	sched := scheduler.NewMultiAgent([]agent.Agent{
		mk("A", "[A:result]"),
		mk("B", "[B:result]"),
		mk("C", "[C:result]"),
	}, e)
	g := &scheduler.Graph{
		Start: "n1",
		Nodes: map[string]scheduler.Node{
			"n1": {ID: "n1", Agent: "A", Input: "start", Next: []string{"n2"}},
			"n2": {ID: "n2", Agent: "B", Input: "{{prev}}", Next: []string{"n3"}},
			"n3": {ID: "n3", Agent: "C", Input: "{{prev}}"},
		},
	}
	res, err := sched.Run(context.Background(), g, "")
	check("3-node chain completes", err == nil && len(res.Visited) == 3, fmt.Sprintf("visited=%v err=%v", res.Visited, err))
	check("visitation order preserved",
		len(res.Visited) == 3 && res.Visited[0] == "n1" && res.Visited[2] == "n3",
		fmt.Sprintf("%v", res.Visited))
	check("{{prev}} substitution propagated",
		strings.Contains(res.Outputs["n3"], "C:result"),
		res.Outputs["n3"])

	// Invalid graph (cycle) rejected.
	bad := &scheduler.Graph{
		Start: "x",
		Nodes: map[string]scheduler.Node{
			"x": {ID: "x", Agent: "A", Next: []string{"y"}},
			"y": {ID: "y", Agent: "B", Next: []string{"x"}},
		},
	}
	_, err = sched.Run(context.Background(), bad, "")
	check("cycle rejected by Graph.Validate", err != nil && strings.Contains(err.Error(), "cycle"), fmt.Sprintf("%v", err))
}

func check9_SkillLoader() {
	fmt.Println("\n[9] Skill loader: SKILL.md frontmatter + body")
	root, _ := os.MkdirTemp("", "aievo-skill-")
	defer os.RemoveAll(root)
	dir := filepath.Join(root, "greet")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
description: greet the user warmly
trigger: hello
---

When asked to greet, respond enthusiastically.`), 0o644)

	skills, err := skill.LoadDir(root)
	check("skill discovered", err == nil && len(skills) == 1, fmt.Sprintf("%d, err=%v", len(skills), err))
	if len(skills) == 1 {
		s := skills[0]
		check("frontmatter description parsed", s.Description == "greet the user warmly", s.Description)
		check("body extracted", strings.Contains(s.Body, "enthusiastically"), s.Body)
		check("trigger parsed", s.Trigger == "hello", s.Trigger)
	}
}

func check10_TaskTrackerAndPlanMode() {
	fmt.Println("\n[10] Task tracker + Plan mode: stateful tools")
	store := builtin.NewTaskStore()
	tools := store.Tools()
	create, update, list := tools[0], tools[1], tools[2]
	_, _ = create.Call(context.Background(), `{"subject":"compile"}`)
	_, _ = create.Call(context.Background(), `{"subject":"test"}`)
	_, _ = update.Call(context.Background(), `{"id":"1","status":"completed"}`)
	out, _ := list.Call(context.Background(), `{}`)
	check("two tasks recorded", strings.Count(out, `"id"`) == 2, out)
	check("status update visible", strings.Contains(out, `"completed"`), out)

	plan := builtin.NewPlanState()
	pt := plan.Tools()
	enter, exit := pt[0], pt[1]
	_, _ = enter.Call(context.Background(), `{}`)
	check("plan mode active", plan.Active(), "")
	_, _ = exit.Call(context.Background(), `{"plan":"build a tree"}`)
	check("plan mode exited", !plan.Active(), "")
	check("plan body captured", plan.Plan() == "build a tree", plan.Plan())
}

// ---------- helpers ----------

type slowTool struct {
	name string
	d    time.Duration
	safe bool
}

func (s *slowTool) Name() string             { return s.name }
func (s *slowTool) Description() string      { return "slow" }
func (s *slowTool) Schema() []byte           { return nil }
func (s *slowTool) IsConcurrencySafe() bool  { return s.safe }
func (s *slowTool) Call(ctx context.Context, _ string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(s.d):
		return "done", nil
	}
}

func idsOf(results []tool.Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.CallID
	}
	return out
}

// blockingLLM returns a stream that never produces an event — exercises
// ctx cancellation paths.
type blockingLLM struct{ d time.Duration }

func (b *blockingLLM) Stream(ctx context.Context, _ llm.Request) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
		case <-time.After(b.d):
		}
	}()
	return ch, nil
}
