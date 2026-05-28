// Package openai adapts the github.com/sashabaranov/go-openai streaming API
// onto aievo-next's provider-agnostic llm.LLM interface.
//
// Design notes:
//   - Tool call arguments arrive incrementally in OpenAI's stream; we buffer
//     them and emit a single ToolUseEvent per call when the delta finalises.
//   - Reasoning / thinking deltas (used by DeepSeek and similar) are surfaced
//     via the ReasoningContent field on go-openai. We forward them as
//     llm.ReasoningDeltaEvent so the UI / loop can decide whether to display.
//   - Errors are wrapped in *errors.LLMError so the retry policy can decide.
package openai

import (
	"context"
	"errors"
	"io"
	"strings"

	goopenai "github.com/sashabaranov/go-openai"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/llm"
)

// Config configures the OpenAI adapter. BaseURL is optional (default:
// https://api.openai.com/v1). Any OpenAI-compatible endpoint works
// (Ollama, vLLM, DeepSeek, etc.).
type Config struct {
	APIKey  string
	BaseURL string
}

// Client is an llm.LLM backed by go-openai.
type Client struct {
	api *goopenai.Client
}

// New constructs a client. APIKey is required; BaseURL may be empty.
func New(cfg Config) *Client {
	c := goopenai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		c.BaseURL = cfg.BaseURL
	}
	return &Client{api: goopenai.NewClientWithConfig(c)}
}

// Stream implements llm.LLM.
func (c *Client) Stream(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	chatReq := toChatRequest(req)
	stream, err := c.api.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		return nil, aierrs.NewLLMError("openai", statusCodeFromErr(err), err)
	}
	ch := make(chan llm.StreamEvent, 8)
	go func() {
		defer close(ch)
		defer stream.Close()
		// Per-call argument buffer keyed by tool call index.
		argBuf := map[int]*pendingCall{}
		var usage llm.Usage
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				flushPendingCalls(ctx, ch, argBuf)
				select {
				case <-ctx.Done():
				case ch <- llm.FinishEvent{Reason: llm.FinishStop, Usage: usage}:
				}
				return
			}
			if err != nil {
				select {
				case <-ctx.Done():
				case ch <- llm.ErrorEvent{Err: aierrs.NewLLMError("openai", statusCodeFromErr(err), err)}:
				}
				return
			}
			if len(resp.Choices) == 0 {
				continue
			}
			choice := resp.Choices[0]
			if choice.Delta.Content != "" {
				select {
				case <-ctx.Done():
					return
				case ch <- llm.DeltaEvent{Content: choice.Delta.Content}:
				}
			}
			if choice.Delta.ReasoningContent != "" {
				select {
				case <-ctx.Done():
					return
				case ch <- llm.ReasoningDeltaEvent{Content: choice.Delta.ReasoningContent}:
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				p, ok := argBuf[idx]
				if !ok {
					p = &pendingCall{}
					argBuf[idx] = p
				}
				if tc.ID != "" {
					p.id = tc.ID
				}
				if tc.Function.Name != "" {
					p.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					p.args.WriteString(tc.Function.Arguments)
				}
			}
			if choice.FinishReason != "" {
				flushPendingCalls(ctx, ch, argBuf)
				select {
				case <-ctx.Done():
				case ch <- llm.FinishEvent{Reason: toFinishReason(choice.FinishReason), Usage: usage}:
				}
				return
			}
			if resp.Usage != nil {
				usage.PromptTokens = resp.Usage.PromptTokens
				usage.CompletionTokens = resp.Usage.CompletionTokens
				usage.TotalTokens = resp.Usage.TotalTokens
			}
		}
	}()
	return ch, nil
}

type pendingCall struct {
	id   string
	name string
	args strings.Builder
}

func flushPendingCalls(ctx context.Context, ch chan<- llm.StreamEvent, buf map[int]*pendingCall) {
	for _, p := range buf {
		if p.name == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case ch <- llm.ToolUseEvent{Call: llm.ToolCall{
			ID:    p.id,
			Name:  p.name,
			Input: p.args.String(),
		}}:
		}
	}
}

func toChatRequest(req llm.Request) goopenai.ChatCompletionRequest {
	out := goopenai.ChatCompletionRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}
	for _, m := range req.Messages {
		om := goopenai.ChatCompletionMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, goopenai.ToolCall{
				ID:   tc.ID,
				Type: goopenai.ToolTypeFunction,
				Function: goopenai.FunctionCall{
					Name:      tc.Name,
					Arguments: tc.Input,
				},
			})
		}
		out.Messages = append(out.Messages, om)
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, goopenai.Tool{
			Type: goopenai.ToolTypeFunction,
			Function: &goopenai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  rawJSON(t.Schema),
			},
		})
	}
	return out
}

// rawJSON wraps raw bytes so go-openai's Parameters field accepts them
// without re-marshalling. Reading the field via json.Marshal returns the
// underlying bytes verbatim.
type rawJSON []byte

func (r rawJSON) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return r, nil
}

func toFinishReason(r goopenai.FinishReason) llm.FinishReason {
	switch r {
	case goopenai.FinishReasonStop:
		return llm.FinishStop
	case goopenai.FinishReasonLength:
		return llm.FinishLength
	case goopenai.FinishReasonToolCalls, goopenai.FinishReasonFunctionCall:
		return llm.FinishToolCalls
	case goopenai.FinishReasonContentFilter:
		return llm.FinishContentFilter
	}
	return llm.FinishStop
}

// statusCodeFromErr extracts HTTP status from go-openai's APIError so we
// can let *errors.LLMError decide whether to retry.
func statusCodeFromErr(err error) int {
	var apiErr *goopenai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode
	}
	var reqErr *goopenai.RequestError
	if errors.As(err, &reqErr) {
		return reqErr.HTTPStatusCode
	}
	return 0
}
