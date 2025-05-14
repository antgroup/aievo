package llm

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

type Generation struct {
	// Text is the generated text.
	Role             string `json:"role"`
	Content          string `json:"content"`
	StopReason       string `json:"stop_reason"`
	ReasoningContent string `json:"reasoning_content"`
	// GenerationInfo prepared field
	GenerationInfo map[string]any `json:"generation_info"`
	// ToolCalls is a list of tool calls the model asks to invoke.
	ToolCalls []ToolCall
	Usage     *Usage
	LogProbs  *openai.ChatCompletionStreamChoiceLogprobs
}

type Usage struct {
	CompletionTokens int `json:"completion_tokens,omitempty"`
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type LLM interface {
	Generate(ctx context.Context, prompt string, options ...GenerateOption) (*Generation, error)
	GenerateContent(ctx context.Context, messages []Message, options ...GenerateOption) (*Generation, error)
}
