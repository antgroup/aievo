package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/antgroup/aievo/llm"
	goopenai "github.com/sashabaranov/go-openai"
)

type LLM struct {
	client         *goopenai.Client
	model          string
	ResponseFormat *goopenai.ChatCompletionResponseFormat
}

var (
	_ llm.LLM = (*LLM)(nil)

	_defaultModel = "gpt-4o"
)

// newClient creates an instance of the internal client.
func newClient(opt *options) (*goopenai.Client, error) {

	if len(opt.token) == 0 {
		return nil, errors.New("missing the OpenAI API key, set it in the OPENAI_API_KEY environment variable")
	}

	config := goopenai.DefaultConfig(opt.token)
	if opt.apiType == goopenai.APITypeAzure {
		config = goopenai.DefaultAzureConfig(
			opt.token, opt.baseURL)
	}
	config.BaseURL = opt.baseURL
	config.OrgID = opt.organization

	if opt.httpClient != nil {
		config.HTTPClient = opt.httpClient
	}
	if opt.apiVersion != "" {
		config.APIVersion = opt.apiVersion
	}

	client := goopenai.NewClientWithConfig(config)

	return client, nil
}

// New returns a new OpenAI LLM.
func New(opts ...Option) (*LLM, error) {
	option := &options{
		apiType:    goopenai.APITypeOpenAI,
		httpClient: http.DefaultClient,
		model:      _defaultModel,
	}

	for _, opt := range opts {
		opt(option)
	}
	c, err := newClient(option)
	if err != nil {
		return nil, err
	}
	return &LLM{
		client: c,
		model:  option.model,
	}, err
}

// GenerateContent implements the Model interface.
func (l *LLM) GenerateContent(ctx context.Context, messages []llm.Message, options ...llm.GenerateOption) (*llm.Generation, error) {
	opts := llm.DefaultGenerateOption()
	for _, opt := range options {
		opt(opts)
	}

	msgs := make([]goopenai.ChatCompletionMessage, 0, len(messages))
	for _, mc := range messages {
		msgs = append(msgs, goopenai.ChatCompletionMessage{
			Role:       string(mc.Role),
			Name:       mc.Name,
			Content:    mc.Content,
			ToolCallID: mc.ToolCallId,
			ToolCalls:  llmToolCall2ToolCall(mc.ToolCalls),
		})
	}
	req := goopenai.ChatCompletionRequest{
		Model:    l.model,
		Stop:     opts.StopWords,
		Messages: msgs,
		Stream:   true,
		StreamOptions: &goopenai.StreamOptions{
			IncludeUsage: true,
		},
		Temperature:      opts.Temperature,
		N:                opts.N,
		FrequencyPenalty: opts.FrequencyPenalty,
		PresencePenalty:  opts.PresencePenalty,

		MaxCompletionTokens: opts.MaxTokens,

		ToolChoice:  opts.ToolChoice,
		LogProbs:    opts.LogProbs,
		TopLogProbs: opts.TopLogProbs,
		Seed:        &opts.Seed,
		Metadata:    opts.Metadata,
	}

	if opts.JSONMode {
		req.ResponseFormat = &goopenai.ChatCompletionResponseFormat{Type: "json_object"}
	}

	// if opts.Tools is not empty, append them to req.Tools
	for _, tool := range opts.Tools {
		t, err := toolFromTool(&tool)
		if err != nil {
			return nil, fmt.Errorf("failed to convert llms tool to openai tool: %w", err)
		}
		req.Tools = append(req.Tools, t)
	}

	// fix for some platform dont support ParallelToolCalls when tool not set
	if opts.ParallelToolCalls != nil && len(req.Tools) > 0 {
		req.ParallelToolCalls = opts.ParallelToolCalls
	}

	// if o.client.ResponseFormat is set, use it for the request
	if l.ResponseFormat != nil {
		req.ResponseFormat = l.ResponseFormat
	}

	streamer, err := l.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}

	var response = &llm.Generation{
		Usage:    &llm.Usage{},
		LogProbs: &goopenai.ChatCompletionStreamChoiceLogprobs{},
	}

	for {
		recv, err := streamer.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(recv.Choices) > 0 {
			if recv.Choices[0].Delta.ToolCalls != nil {
				toolCall2LLMToolCall(response, recv.Choices[0].Delta.ToolCalls)
			}
			if recv.Choices[0].FinishReason != "" {
				response.StopReason = fmt.Sprint(recv.Choices[0].FinishReason)
			}
			if recv.Choices[0].Delta.Role != "" {
				response.Role = recv.Choices[0].Delta.Role
			}
			if recv.Choices[0].Logprobs != nil {
				response.LogProbs.Content = append(response.LogProbs.Content,
					recv.Choices[0].Logprobs.Content...)
			}
			response.Content += recv.Choices[0].Delta.Content
			response.ReasoningContent += recv.Choices[0].Delta.ReasoningContent
			if opts.ReasoningStreamingFunc != nil && recv.Choices[0].Delta.ReasoningContent != "" {
				_ = opts.ReasoningStreamingFunc(ctx, []byte(recv.Choices[0].Delta.Content))
			}
			if opts.StreamingFunc != nil && recv.Choices[0].Delta.Content != "" {
				_ = opts.StreamingFunc(ctx, []byte(recv.Choices[0].Delta.Content))
			}
		}
		if recv.Usage != nil {
			response.Usage.PromptTokens = recv.Usage.PromptTokens
			response.Usage.TotalTokens = recv.Usage.TotalTokens
			response.Usage.CompletionTokens = recv.Usage.CompletionTokens
		}
	}

	return response, nil
}

func (l *LLM) Generate(ctx context.Context, prompt string, options ...llm.GenerateOption) (*llm.Generation, error) {
	message := llm.NewUserMessage("", prompt)
	return l.GenerateContent(ctx, []llm.Message{message}, options...)
}

// toolFromTool converts an llms.Tool to a Tool.
func toolFromTool(t *llm.Tool) (goopenai.Tool, error) {
	tool := goopenai.Tool{
		Type: goopenai.ToolType(t.Type),
	}
	switch t.Type {
	case string(goopenai.ToolTypeFunction):
		tool.Function = &goopenai.FunctionDefinition{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
			Strict:      t.Function.Strict,
		}
	default:
		return goopenai.Tool{}, fmt.Errorf("tool type %v not supported", t.Type)
	}
	return tool, nil
}

func llmToolCall2ToolCall(toolCalls []llm.ToolCall) []goopenai.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	calls := make([]goopenai.ToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		calls = append(calls, goopenai.ToolCall{
			ID:   call.ID,
			Type: goopenai.ToolType(call.Type),
			Function: goopenai.FunctionCall{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}
	return calls
}

func toolCall2LLMToolCall(generation *llm.Generation, toolCalls []goopenai.ToolCall) {
	if len(toolCalls) == 0 {
		return
	}
	if len(generation.ToolCalls) == 0 {
		generation.ToolCalls = make([]llm.ToolCall, 0, len(toolCalls))
	}

	for idx, call := range toolCalls {
		if call.Index != nil {
			idx = *call.Index
		}
		for i := len(generation.ToolCalls); i <= idx; i++ {
			generation.ToolCalls = append(generation.ToolCalls, llm.ToolCall{
				ID:       "",
				Type:     "",
				Function: &llm.FunctionCall{},
			})
		}
		if call.ID != "" {
			generation.ToolCalls[idx].ID = call.ID
		}
		if call.Type != "" {
			generation.ToolCalls[idx].Type = string(call.Type)
		}
		generation.ToolCalls[idx].Function.Arguments += call.Function.Arguments
		generation.ToolCalls[idx].Function.Name += call.Function.Name
	}
}
