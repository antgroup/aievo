package llm

import "context"

// LLM is the provider-agnostic streaming chat interface. There is only one
// method — providers expose richer behaviour by interpreting Request fields
// and emitting StreamEvent variants. The channel is closed by the provider
// after the final FinishEvent.
type LLM interface {
	Stream(ctx context.Context, req Request) (<-chan StreamEvent, error)
}

// StreamEvent is a sealed interface — only this package may declare variants.
// Consumers use a type switch; the compiler enforces exhaustiveness.
type StreamEvent interface {
	isStreamEvent()
}

type streamBase struct{}

func (streamBase) isStreamEvent() {}

// DeltaEvent carries one streamed chunk of assistant text content.
type DeltaEvent struct {
	streamBase
	Content string
}

// ToolUseEvent indicates the model called one or more tools. Some providers
// stream these incrementally (one event per tool); the framework treats
// each as a complete call once delivered.
type ToolUseEvent struct {
	streamBase
	Call ToolCall
}

// ReasoningDeltaEvent carries reasoning / thinking text (DeepSeek, Anthropic
// extended thinking, OpenAI o1-style). Separated from DeltaEvent so the UI
// can render it differently. Optional — providers may omit.
type ReasoningDeltaEvent struct {
	streamBase
	Content string
}

// FinishEvent is the terminal event of every successful stream.
type FinishEvent struct {
	streamBase
	Reason FinishReason
	Usage  Usage
}

// ErrorEvent is emitted instead of FinishEvent when the provider fails
// mid-stream. The channel is closed immediately after.
type ErrorEvent struct {
	streamBase
	Err error
}
