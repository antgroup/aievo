package llm

import "context"

// GenerateOption is a function that configures a GenerateOptions.
type GenerateOption func(*GenerateOptions)

// GenerateOptions is a set of options for calling models. Not all models support
// all options.
type GenerateOptions struct {
	// Model is the model to use.
	Model string `json:"model"`
	// CandidateCount is the number of response candidates to generate.
	CandidateCount int `json:"candidate_count"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens"`
	// Temperature is the temperature for sampling, between 0 and 1.
	Temperature float32 `json:"temperature"`
	// StopWords is a list of words to stop on.
	StopWords []string `json:"stop_words"`
	// StreamingFunc is a function to be called for each chunk of a streaming response.
	// Return an error to stop streaming early.
	StreamingFunc          func(ctx context.Context, chunk []byte) error `json:"-"`
	ReasoningStreamingFunc func(ctx context.Context, chunk []byte) error `json:"-"`
	// TopK is the number of tokens to consider for top-k sampling.
	TopK int `json:"top_k"`
	// TopP is the cumulative probability for top-p sampling.
	TopP float64 `json:"top_p"`
	// Seed is a seed for deterministic sampling.
	Seed int `json:"seed"`
	// MinLength is the minimum length of the generated text.
	MinLength int `json:"min_length"`
	// MaxLength is the maximum length of the generated text.
	MaxLength int `json:"max_length"`
	// N is how many chat completion choices to generate for each input message.
	N int `json:"n"`
	// RepetitionPenalty is the repetition penalty for sampling.
	RepetitionPenalty float32 `json:"repetition_penalty"`
	// FrequencyPenalty is the frequency penalty for sampling.
	FrequencyPenalty float32 `json:"frequency_penalty"`
	// PresencePenalty is the presence penalty for sampling.
	PresencePenalty float32 `json:"presence_penalty"`

	// JSONMode is a flag to enable JSON mode.
	JSONMode bool `json:"json"`

	// Tools is a list of tools to use. Each tool can be a specific tool or a function.
	Tools []Tool `json:"tools,omitempty"`
	// ParallelToolCalls Whether to enable parallel function calling during tool use.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
	// ToolChoice is the choice of tool to use, it can either be "none", "auto" (the default behavior), or a specific tool as described in the ToolChoice type.
	ToolChoice any `json:"tool_choice"`

	// Metadata is a map of metadata to include in the request.
	// The meaning of this field is specific to the backend in use.
	Metadata map[string]string `json:"metadata,omitempty"`

	// ResponseMIMEType MIME type of the generated candidate text.
	// Supported MIME types are: text/plain: (default) Text output.
	// application/json: JSON response in the response candidates.
	ResponseMIMEType string `json:"response_mime_type,omitempty"`

	LogProbs bool `json:"logprobs,omitempty"`
	// TopLogProbs is an integer between 0 and 5 specifying the number of most likely tokens to return at each
	// token position, each with an associated log probability.
	// logprobs must be set to true if this parameter is used.
	TopLogProbs int `json:"top_logprobs,omitempty"`
}

// Tool is a tool that can be used by the model.
type Tool struct {
	// Type is the type of the tool.
	Type string `json:"type"`
	// Function is the function to call.
	Function *FunctionDefinition `json:"function,omitempty"`
}

// FunctionDefinition is a definition of a function that can be called by the model.
type FunctionDefinition struct {
	// Name is the name of the function.
	Name string `json:"name"`
	// Description is a description of the function.
	Description string `json:"description"`
	// Parameters is a list of parameters for the function.
	Parameters any `json:"parameters,omitempty"`
	// Strict is a flag to indicate if the function should be called strictly. Only used for openai llm structured output.
	Strict bool `json:"strict,omitempty"`
}

// ToolChoice is a specific tool to use.
type ToolChoice struct {
	// Type is the type of the tool.
	Type string `json:"type"`
	// Function is the function to call (if the tool is a function).
	Function *FunctionReference `json:"function,omitempty"`
}

// FunctionReference is a reference to a function.
type FunctionReference struct {
	// Name is the name of the function.
	Name string `json:"name"`
}

// FunctionCallBehavior is the behavior to use when calling functions.
type FunctionCallBehavior string

const (
	// FunctionCallBehaviorNone will not call any functions.
	FunctionCallBehaviorNone FunctionCallBehavior = "none"
	// FunctionCallBehaviorAuto will call functions automatically.
	FunctionCallBehaviorAuto FunctionCallBehavior = "auto"
)

// WithModel specifies which model name to use.
func WithModel(model string) GenerateOption {
	return func(o *GenerateOptions) {
		o.Model = model
	}
}

// WithMaxTokens specifies the max number of tokens to generate.
func WithMaxTokens(maxTokens int) GenerateOption {
	return func(o *GenerateOptions) {
		o.MaxTokens = maxTokens
	}
}

// WithCandidateCount specifies the number of response candidates to generate.
func WithCandidateCount(c int) GenerateOption {
	return func(o *GenerateOptions) {
		o.CandidateCount = c
	}
}

// WithTemperature specifies the model temperature, a hyperparameter that
// regulates the randomness, or creativity, of the AI's responses.
func WithTemperature(temperature float32) GenerateOption {
	return func(o *GenerateOptions) {
		o.Temperature = temperature
	}
}

// WithStopWords specifies a list of words to stop generation on.
func WithStopWords(stopWords []string) GenerateOption {
	return func(o *GenerateOptions) {
		o.StopWords = stopWords
	}
}

// WithOptions specifies options.
func WithOptions(options GenerateOptions) GenerateOption {
	return func(o *GenerateOptions) {
		(*o) = options
	}
}

// WithStreamingFunc specifies the streaming function to use.
func WithStreamingFunc(streamingFunc func(ctx context.Context, chunk []byte) error) GenerateOption {
	return func(o *GenerateOptions) {
		o.StreamingFunc = streamingFunc
	}
}

// WithReasoningStreamingFunc specifies the streaming function for reasoning to use.
func WithReasoningStreamingFunc(streamingFunc func(ctx context.Context, chunk []byte) error) GenerateOption {
	return func(o *GenerateOptions) {
		o.ReasoningStreamingFunc = streamingFunc
	}
}

// WithTopK will add an option to use top-k sampling.
func WithTopK(topK int) GenerateOption {
	return func(o *GenerateOptions) {
		o.TopK = topK
	}
}

// WithTopP	will add an option to use top-p sampling.
func WithTopP(topP float64) GenerateOption {
	return func(o *GenerateOptions) {
		o.TopP = topP
	}
}

// WithSeed will add an option to use deterministic sampling.
func WithSeed(seed int) GenerateOption {
	return func(o *GenerateOptions) {
		o.Seed = seed
	}
}

// WithMinLength will add an option to set the minimum length of the generated text.
func WithMinLength(minLength int) GenerateOption {
	return func(o *GenerateOptions) {
		o.MinLength = minLength
	}
}

// WithMaxLength will add an option to set the maximum length of the generated text.
func WithMaxLength(maxLength int) GenerateOption {
	return func(o *GenerateOptions) {
		o.MaxLength = maxLength
	}
}

// WithN will add an option to set how many chat completion choices to generate for each input message.
func WithN(n int) GenerateOption {
	return func(o *GenerateOptions) {
		o.N = n
	}
}

// WithRepetitionPenalty will add an option to set the repetition penalty for sampling.
func WithRepetitionPenalty(repetitionPenalty float32) GenerateOption {
	return func(o *GenerateOptions) {
		o.RepetitionPenalty = repetitionPenalty
	}
}

// WithFrequencyPenalty will add an option to set the frequency penalty for sampling.
func WithFrequencyPenalty(frequencyPenalty float32) GenerateOption {
	return func(o *GenerateOptions) {
		o.FrequencyPenalty = frequencyPenalty
	}
}

// WithPresencePenalty will add an option to set the presence penalty for sampling.
func WithPresencePenalty(presencePenalty float32) GenerateOption {
	return func(o *GenerateOptions) {
		o.PresencePenalty = presencePenalty
	}
}

// WithToolChoice will add an option to set the choice of tool to use.
// It can either be "none", "auto" (the default behavior), or a specific tool as described in the ToolChoice type.
func WithToolChoice(choice any) GenerateOption {
	// TODO: Add type validation for choice.
	return func(o *GenerateOptions) {
		o.ToolChoice = choice
	}
}

// WithTools will add an option to set the tools to use.
func WithTools(tools []Tool) GenerateOption {
	return func(o *GenerateOptions) {
		o.Tools = tools
	}
}

// WithJSONMode will add an option to set the response format to JSON.
// This is useful for models that return structured data.
func WithJSONMode() GenerateOption {
	return func(o *GenerateOptions) {
		o.JSONMode = true
	}
}

// WithMetadata will add an option to set metadata to include in the request.
// The meaning of this field is specific to the backend in use.
func WithMetadata(metadata map[string]string) GenerateOption {
	return func(o *GenerateOptions) {
		o.Metadata = metadata
	}
}

// WithResponseMIMEType will add an option to set the ResponseMIMEType
// Currently only supported by googleai llms.
func WithResponseMIMEType(responseMIMEType string) GenerateOption {
	return func(o *GenerateOptions) {
		o.ResponseMIMEType = responseMIMEType
	}
}

func WithParallelToolCalls(parallel bool) GenerateOption {
	return func(o *GenerateOptions) {
		o.ParallelToolCalls = &parallel
	}
}

func WithLogProbes(probe bool) GenerateOption {
	return func(o *GenerateOptions) {
		o.LogProbs = probe
	}
}

func WithTopLogProbs(top int) GenerateOption {
	return func(o *GenerateOptions) {
		o.TopLogProbs = top
	}
}

func DefaultGenerateOption() *GenerateOptions {
	// v := true
	return &GenerateOptions{
		ParallelToolCalls: nil,
		// ToolChoice:        "none",
	}
}
