// Package permission implements the four Claude-Code permission modes.
//
//	default            — ask via Decider for each call
//	acceptEdits        — auto-approve file writes / edits; ask for the rest
//	plan               — read-only; refuse anything mutating
//	bypassPermissions  — approve everything (sandbox / CI only)
package permission

import (
	"context"
	"strings"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

type Mode int

const (
	ModeDefault Mode = iota
	ModeAcceptEdits
	ModePlan
	ModeBypass
)

func (m Mode) String() string {
	switch m {
	case ModeDefault:
		return "default"
	case ModeAcceptEdits:
		return "acceptEdits"
	case ModePlan:
		return "plan"
	case ModeBypass:
		return "bypassPermissions"
	}
	return "unknown"
}

// Decision is the outcome of one permission check.
type Decision int

const (
	Allow Decision = iota
	Deny
	AskUser
)

// Decider returns a decision for a single tool call. Used in ModeDefault
// and as the fallback for "ask" branches in ModeAcceptEdits.
//
// Implementations may block on stdin, GUI, ACP prompt, etc. — the framework
// honours ctx.
type Decider func(ctx context.Context, t tool.Tool, input string) (Decision, error)

// AlwaysAllow is a Decider that approves every call. Suitable for tests.
func AlwaysAllow(context.Context, tool.Tool, string) (Decision, error) { return Allow, nil }

// AlwaysDeny denies everything. Useful in restricted ModePlan tests.
func AlwaysDeny(context.Context, tool.Tool, string) (Decision, error) { return Deny, nil }

// Gate decides whether a call is permitted under the given mode.
//
// Mutating tools are detected by the Tool.IsConcurrencySafe() == false flag,
// plus a small allow-list of write-y names that are concurrency-safe by
// chance but still mutate state. Add more there as the tool set grows.
var writeyNames = map[string]struct{}{
	"FileWrite":   {},
	"FileEdit":    {},
	"FileMultiEdit": {},
	"Bash":        {},
	"NotebookEdit": {},
}

// IsMutating reports whether t can change state outside the agent.
func IsMutating(t tool.Tool) bool {
	if _, ok := writeyNames[t.Name()]; ok {
		return true
	}
	// Concurrency-unsafe is a conservative proxy: anything you can't run
	// twice in parallel is suspected of mutating something.
	return !t.IsConcurrencySafe()
}

// Gate applies the mode's policy. Returns Allow/Deny/AskUser (callers
// route AskUser through Decider if they have one).
func Gate(mode Mode, t tool.Tool) Decision {
	if mode == ModeBypass {
		return Allow
	}
	mutating := IsMutating(t)
	switch mode {
	case ModePlan:
		if mutating {
			return Deny
		}
		return Allow
	case ModeAcceptEdits:
		// Edit-y tools auto-approve; bash and friends still ask.
		if isEditish(t.Name()) {
			return Allow
		}
		if !mutating {
			return Allow
		}
		return AskUser
	case ModeDefault:
		if !mutating {
			return Allow
		}
		return AskUser
	}
	return Deny
}

func isEditish(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "edit") || strings.Contains(n, "write")
}

// Check is the high-level entry: returns nil if allowed, a *errors.ToolError
// if denied. Use this from agent code.
func Check(ctx context.Context, mode Mode, decider Decider, t tool.Tool, input string) error {
	switch Gate(mode, t) {
	case Allow:
		return nil
	case Deny:
		return aierrs.NewFatal(&deniedErr{tool: t.Name(), mode: mode})
	case AskUser:
		if decider == nil {
			return aierrs.NewFatal(&deniedErr{tool: t.Name(), mode: mode, reason: "no decider for AskUser"})
		}
		d, err := decider(ctx, t, input)
		if err != nil {
			return err
		}
		if d == Allow {
			return nil
		}
		return aierrs.NewFatal(&deniedErr{tool: t.Name(), mode: mode, reason: "user denied"})
	}
	return aierrs.NewFatal(&deniedErr{tool: t.Name(), mode: mode})
}

type deniedErr struct {
	tool, reason string
	mode         Mode
}

func (e *deniedErr) Error() string {
	if e.reason != "" {
		return "permission denied for " + e.tool + " (mode=" + e.mode.String() + "): " + e.reason
	}
	return "permission denied for " + e.tool + " (mode=" + e.mode.String() + ")"
}
