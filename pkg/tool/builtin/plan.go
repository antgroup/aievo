package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
)

// ===== EnterPlanMode / ExitPlanMode =====
// Stateful pair: EnterPlanMode flips a flag so a permission Gate can refuse
// mutating tools; ExitPlanMode flips it back and reports the plan body.
// Inspired by Claude Code's EnterPlanMode / ExitPlanMode pair (book ch. 4).

type PlanState struct {
	active int32 // 0=normal, 1=in plan mode
	plan   atomic.Value // string
}

func NewPlanState() *PlanState {
	s := &PlanState{}
	s.plan.Store("")
	return s
}

func (s *PlanState) Active() bool {
	return atomic.LoadInt32(&s.active) == 1
}

func (s *PlanState) Plan() string {
	v, _ := s.plan.Load().(string)
	return v
}

// Tools returns the two plan-mode tools backed by s.
func (s *PlanState) Tools() []planTool {
	return []planTool{
		{name: "EnterPlanMode", state: s, enter: true,
			desc:   `Switch the agent into read-only plan mode. No further mutating tools allowed until ExitPlanMode. Input: {}.`,
			schema: []byte(`{"type":"object","properties":{}}`)},
		{name: "ExitPlanMode", state: s, enter: false,
			desc:   `Exit plan mode and present the final plan. Input: {"plan":"..."}.`,
			schema: []byte(`{"type":"object","properties":{"plan":{"type":"string"}},"required":["plan"]}`)},
	}
}

type planTool struct {
	name, desc string
	schema     []byte
	state      *PlanState
	enter      bool
}

func (p planTool) Name() string             { return p.name }
func (p planTool) Description() string      { return p.desc }
func (p planTool) Schema() []byte           { return p.schema }
func (p planTool) IsConcurrencySafe() bool  { return true }
func (p planTool) Call(_ context.Context, input string) (string, error) {
	if p.enter {
		atomic.StoreInt32(&p.state.active, 1)
		return "plan mode active; only read-only tools may be used", nil
	}
	var in struct{ Plan string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(err)
	}
	p.state.plan.Store(in.Plan)
	atomic.StoreInt32(&p.state.active, 0)
	return fmt.Sprintf("plan recorded (%d bytes); back to normal mode", len(in.Plan)), nil
}
