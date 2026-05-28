package permission

import (
	"context"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/tool"
)

type fake struct {
	name string
	safe bool
}

func (f fake) Name() string             { return f.name }
func (f fake) Description() string      { return f.name }
func (f fake) Schema() []byte           { return nil }
func (f fake) IsConcurrencySafe() bool  { return f.safe }
func (fake) Call(context.Context, string) (string, error) { return "", nil }

func TestGate_Bypass(t *testing.T) {
	if Gate(ModeBypass, fake{name: "Bash", safe: false}) != Allow {
		t.Fatal("bypass must allow everything")
	}
}

func TestGate_Plan_DeniesMutating(t *testing.T) {
	if Gate(ModePlan, fake{name: "Bash", safe: false}) != Deny {
		t.Fatal("plan must deny mutating")
	}
	if Gate(ModePlan, fake{name: "FileRead", safe: true}) != Allow {
		t.Fatal("plan must allow read")
	}
}

func TestGate_AcceptEdits(t *testing.T) {
	if Gate(ModeAcceptEdits, fake{name: "FileWrite", safe: false}) != Allow {
		t.Fatal("acceptEdits should auto-approve writes")
	}
	if Gate(ModeAcceptEdits, fake{name: "Bash", safe: false}) != AskUser {
		t.Fatal("acceptEdits must still ask for Bash")
	}
}

func TestGate_Default(t *testing.T) {
	if Gate(ModeDefault, fake{name: "FileRead", safe: true}) != Allow {
		t.Fatal("default must allow read")
	}
	if Gate(ModeDefault, fake{name: "Bash", safe: false}) != AskUser {
		t.Fatal("default must ask for Bash")
	}
}

func TestCheck_AskUserRoutes(t *testing.T) {
	called := false
	decider := func(context.Context, tool.Tool, string) (Decision, error) {
		called = true
		return Allow, nil
	}
	if err := Check(context.Background(), ModeDefault, decider, fake{name: "Bash"}, "x"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("decider was not invoked")
	}
}

func TestCheck_NoDecider(t *testing.T) {
	if err := Check(context.Background(), ModeDefault, nil, fake{name: "Bash"}, "x"); err == nil {
		t.Fatal("expected error when AskUser has no decider")
	}
}
