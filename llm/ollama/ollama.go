package ollama

import (
	"context"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/ollama/internal"
)

type LLM struct {
	client  *internal.Client
	options options
}

// New creates a new ollama LLM implementation.
func New(opts ...Option) (*LLM, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	client, err := internal.NewClient(o.ollamaServerURL, o.httpClient)
	if err != nil {
		return nil, err
	}

	return &LLM{client: client, options: o}, nil
}

// GenerateContent implements the Model interface.
func (l *LLM) GenerateContent(ctx context.Context, messages []llm.Message, options ...llm.GenerateOption) (*llm.Generation, error) {
	opts := llm.DefaultGenerateOption()
	for _, opt := range options {
		opt(opts)
	}

	model := l.options.model
	if opts.Model != "" {
		model = opts.Model
	}

	msgs := make([]*internal.Message, 0, len(messages))

	for _, mc := range messages {
		msgs = append(msgs, &internal.Message{
			Role:    string(mc.Role),
			Content: mc.Content,
		})
	}

	format := l.options.format
	if opts.JSONMode {
		format = "json"
	}

	ollamaOptions := makeOllamaOptionsFromOptions(l.options.ollamaOptions, *opts)
	req := &internal.ChatRequest{
		Model:    model,
		Format:   format,
		Messages: msgs,
		Options:  ollamaOptions,
		Stream:   opts.StreamingFunc != nil,
	}

	keepAlive := l.options.keepAlive
	if keepAlive != "" {
		req.KeepAlive = keepAlive
	}

	var fn internal.ChatResponseFunc
	streamedResponse := ""
	var resp internal.ChatResponse
	fn = func(response internal.ChatResponse) error {
		if opts.StreamingFunc != nil && response.Message != nil {
			if err := opts.StreamingFunc(ctx, []byte(response.Message.Content)); err != nil {
				return err
			}
		}
		if response.Message != nil {
			streamedResponse += response.Message.Content
		}
		if !req.Stream || response.Done {
			resp = response
			resp.Message = &internal.Message{
				Role:    "assistant",
				Content: streamedResponse,
			}
		}
		return nil
	}

	err := l.client.GenerateChat(ctx, req, fn)
	if err != nil {
		return nil, err
	}

	response := llm.Generation{
		Usage: &llm.Usage{},
	}

	response.Role = resp.Message.Role
	response.Content = resp.Message.Content
	response.Usage.CompletionTokens = resp.EvalCount
	response.Usage.PromptTokens = resp.PromptEvalCount
	response.Usage.TotalTokens = resp.EvalCount + resp.PromptEvalCount

	return &response, nil
}

func (l *LLM) Generate(ctx context.Context, prompt string, options ...llm.GenerateOption) (*llm.Generation, error) {
	message := llm.NewUserMessage("", prompt)
	return l.GenerateContent(ctx, []llm.Message{*message}, options...)
}

func makeOllamaOptionsFromOptions(ollamaOptions internal.Options, opts llm.GenerateOptions) internal.Options {
	ollamaOptions.NumPredict = opts.MaxTokens
	ollamaOptions.Temperature = float32(opts.Temperature)
	ollamaOptions.Stop = opts.StopWords
	ollamaOptions.TopK = opts.TopK
	ollamaOptions.TopP = float32(opts.TopP)
	ollamaOptions.Seed = opts.Seed
	ollamaOptions.RepeatPenalty = float32(opts.RepetitionPenalty)
	ollamaOptions.FrequencyPenalty = float32(opts.FrequencyPenalty)
	ollamaOptions.PresencePenalty = float32(opts.PresencePenalty)
	return ollamaOptions
}
