package schema

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/tool"
)

// StepFeedback is the agent's action to take.
type StepFeedback struct {
	Feedback string `json:"feedback"`
	Log      string
}

// StepAction is the agent's action to take.
type StepAction struct {
	Id          string `json:"id"`
	Action      string `json:"action"`
	Thought     string `json:"thought"`
	Input       string `json:"input"`
	Node        string `json:"node"`
	Feedback    string `json:"feedback"`
	Log         string `json:"log"`
	Observation string `json:"observation"`
}

type StepActionInput struct {
	Input any `json:"input"`
}

type StepType struct {
	Type string `json:"type"`
}

// Generation is the output of a single generation.
type Generation struct {
	// Generated text output.
	Messages []Message
	// Raw generation info response from the provider.
	// May include things like reason for finishing (e.g. in OpenAI).
	TotalTokens int
}

// Agent is the interface all agents must implement.
type Agent interface {
	Run(ctx context.Context, messages []Message, opts ...llm.GenerateOption) (*Generation, error)

	Name() string

	Description() string

	WithEnv(env Environment)

	Env() Environment

	Tools() []tool.Tool
}

var (
	ErrMissingLLM          = errors.New("missing field LLM")
	ErrMissingEnv          = errors.New("missing field Env")
	ErrMissingPrompt       = errors.New("missing fill in prompt")
	ErrMissingName         = errors.New("missing agent name")
	ErrMissingDesc         = errors.New("missing agent desc")
	ErrMissingGraph        = errors.New("missing sop graph")
	ErrAgentNoReturn       = errors.New("no actions or finish was returned by the agent")
	ErrNotFinished         = errors.New("agent not finished before max iterations")
	ErrParsePromptTemplate = errors.New("parse prompt template error")
)

func ConvertConstructScratchPad(name, self string, messages []Message, steps []StepAction) string {
	var scratchPad string
	for _, message := range messages {
		receiver := message.Receiver
		sender := message.Sender
		if strings.EqualFold(receiver, name) {
			receiver = self
		}
		if strings.EqualFold(sender, name) {
			sender = self
		}
		if message.IsMsg() {
			if message.Condition != "" {
				scratchPad += fmt.Sprintf("(%s -> %s)(%s): %s\n",
					sender, receiver, message.Condition, message.Content)
			} else {
				if sender == "Watcher" {
					scratchPad += fmt.Sprintf("(Hint from Global Watcher): %s\n", message.Content)
				}else {
					scratchPad += fmt.Sprintf("(%s -> %s): %s\n",
						sender, receiver, message.Content)
				}
			}

		}
	}
	for _, step := range steps {
		if step.Feedback == "" {
			scratchPad += fmt.Sprintf(
				"(%s)Thought: %s\nAction: %s\nAction Input: %s\nObservation: %s\n",
				self, step.Thought, step.Action, step.Input, step.Observation)
			continue
		}
		scratchPad += fmt.Sprintf("(%s)Output: %s\nFeedback: %s\n",
			self, step.Log, step.Feedback)

	}
	return scratchPad
}

func ConvertToolNames(actions []tool.Tool) string {
	var tn strings.Builder
	for i, a := range actions {
		if i > 0 {
			tn.WriteString(", ")
		}
		tn.WriteString(a.Name())
	}

	return tn.String()
}

func ConvertToolDescriptions(actions []tool.Tool) string {
	var ts strings.Builder
	for _, a := range actions {
		ts.WriteString(fmt.Sprintf("- %s: %s\n",
			a.Name(), a.Description()))
	}

	return ts.String()
}

func ConvertAgentNames(agents []Agent) string {
	var tn strings.Builder
	for i, a := range agents {
		if i > 0 {
			tn.WriteString(", ")
		}
		tn.WriteString(a.Name())
	}

	return tn.String()
}

func ConvertAgentDescriptions(agents []Agent) string {
	var ts strings.Builder
	for _, a := range agents {
		ts.WriteString(fmt.Sprintf("- %s: %s\n",
			a.Name(), a.Description()))
	}

	return strings.TrimSpace(ts.String())
}
