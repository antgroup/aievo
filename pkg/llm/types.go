// Package llm defines the provider-agnostic LLM interface and a typed
// streaming event hierarchy.
//
// Why a typed stream (vs. aievo's openai.go where the OpenAI SDK leaks into
// callers)? Three reasons:
//
//  1. Provider isolation. Adding Anthropic / Bedrock / Gemini means writing
//     a new adapter that emits the same StreamEvent variants — call sites
//     never change.
//  2. Compiler-enforced exhaustiveness. Consumers use a type switch over
//     StreamEvent and the compiler ensures every variant is handled.
//  3. Independent evolution. The wire format of one provider can change
//     without rippling through the framework.
package llm

// Message is the canonical chat message. Role values: "system" | "user" |
// "assistant" | "tool".
type Message struct {
	Role       string
	Content    string
	ToolCallID string     // set when Role == "tool": the call this response answers
	ToolCalls  []ToolCall // set when Role == "assistant" and the model called tools
}

// ToolCall represents a model-requested tool invocation surfaced through
// the streaming API. ID is provider-issued and must be echoed back in the
// "tool" message that delivers the result.
type ToolCall struct {
	ID    string
	Name  string
	Input string // raw JSON; caller validates against the tool's Schema
}

// ToolSpec is the schema a provider needs to advertise a tool to the model.
type ToolSpec struct {
	Name        string
	Description string
	Schema      []byte // JSON Schema, as raw bytes; opaque to llm package
}

// Request bundles everything an LLM provider needs to fulfil one call.
type Request struct {
	Model       string
	Messages    []Message
	Tools       []ToolSpec
	Temperature float32
	MaxTokens   int
	// Stop, TopP, etc. omitted for v0.1 — add as providers need them.
}

// Usage reports token counts at the end of a stream.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// FinishReason mirrors OpenAI's enum but is provider-agnostic.
type FinishReason int

const (
	FinishStop FinishReason = iota
	FinishLength
	FinishToolCalls
	FinishContentFilter
	FinishError
)

func (f FinishReason) String() string {
	switch f {
	case FinishStop:
		return "stop"
	case FinishLength:
		return "length"
	case FinishToolCalls:
		return "tool_calls"
	case FinishContentFilter:
		return "content_filter"
	case FinishError:
		return "error"
	}
	return "unknown"
}
