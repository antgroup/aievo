// Package anthropic adapts the Anthropic Messages API onto aievo-next's
// llm.LLM interface.
//
// We talk to the API directly (no SDK) — the wire format is stable and
// keeping the dependency surface narrow matters for an open-source library.
//
// Supports:
//   - text-only and tool-use turns
//   - server-sent-events streaming
//   - the four message types Anthropic emits in a stream
//     (message_start, content_block_*, message_delta, message_stop)
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/llm"
)

type Config struct {
	APIKey  string
	BaseURL string // optional; default https://api.anthropic.com
	Version string // optional; default "2023-06-01"
	Client  *http.Client
}

type Client struct{ cfg Config }

func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Version == "" {
		cfg.Version = "2023-06-01"
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Client{cfg: cfg}
}

// Stream implements llm.LLM. It converts our typed Request into Anthropic
// Messages format, opens an SSE stream, and forwards events.
func (c *Client) Stream(ctx context.Context, req llm.Request) (<-chan llm.StreamEvent, error) {
	body, err := json.Marshal(toAnthropicRequest(req))
	if err != nil {
		return nil, aierrs.NewFatal(err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, aierrs.NewFatal(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", c.cfg.Version)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.cfg.Client.Do(httpReq)
	if err != nil {
		return nil, aierrs.NewTransient(err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, aierrs.NewLLMError("anthropic", resp.StatusCode, fmt.Errorf("%s", string(b)))
	}
	ch := make(chan llm.StreamEvent, 16)
	go pump(ctx, resp, ch)
	return ch, nil
}

func pump(ctx context.Context, resp *http.Response, out chan<- llm.StreamEvent) {
	defer close(out)
	defer resp.Body.Close()

	// Build-up state: tool_use blocks arrive as multiple deltas keyed by index.
	type toolBuf struct {
		id, name string
		args     strings.Builder
	}
	tools := map[int]*toolBuf{}
	var usage llm.Usage
	var stopReason string

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		// SSE spec allows "data: x" or "data:x"; some proxies omit the space.
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var ev struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
				Thinking    string `json:"thinking"`
			} `json:"delta"`
			ContentBlock struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Name  string `json:"name"`
			} `json:"content_block"`
			Message struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
				StopReason string `json:"stop_reason"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "message_start":
			usage.PromptTokens = ev.Message.Usage.InputTokens
			usage.CompletionTokens = ev.Message.Usage.OutputTokens
		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				tools[ev.Index] = &toolBuf{id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" {
					send(ctx, out, llm.DeltaEvent{Content: ev.Delta.Text})
				}
			case "thinking_delta":
				if ev.Delta.Thinking != "" {
					send(ctx, out, llm.ReasoningDeltaEvent{Content: ev.Delta.Thinking})
				}
			case "input_json_delta":
				if t, ok := tools[ev.Index]; ok {
					t.args.WriteString(ev.Delta.PartialJSON)
				}
			}
		case "content_block_stop":
			if t, ok := tools[ev.Index]; ok {
				send(ctx, out, llm.ToolUseEvent{Call: llm.ToolCall{
					ID: t.id, Name: t.name, Input: t.args.String(),
				}})
				delete(tools, ev.Index)
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				stopReason = ev.Delta.StopReason
			}
			if ev.Usage.OutputTokens > 0 {
				usage.CompletionTokens = ev.Usage.OutputTokens
			}
		case "message_stop":
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			send(ctx, out, llm.FinishEvent{Reason: toFinishReason(stopReason), Usage: usage})
			return
		case "error":
			send(ctx, out, llm.ErrorEvent{Err: aierrs.NewLLMError("anthropic", 0,
				fmt.Errorf("%s: %s", ev.Error.Type, ev.Error.Message))})
			return
		}
	}
	if err := scanner.Err(); err != nil {
		send(ctx, out, llm.ErrorEvent{Err: aierrs.NewTransient(err)})
	}
}

func send(ctx context.Context, out chan<- llm.StreamEvent, e llm.StreamEvent) {
	select {
	case <-ctx.Done():
	case out <- e:
	}
}

func toFinishReason(r string) llm.FinishReason {
	switch r {
	case "end_turn":
		return llm.FinishStop
	case "max_tokens":
		return llm.FinishLength
	case "tool_use":
		return llm.FinishToolCalls
	case "stop_sequence":
		return llm.FinishStop
	}
	return llm.FinishStop
}

// ----- Request conversion -----

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []map[string]any `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []map[string]any   `json:"tools,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
	Temp      *float32           `json:"temperature,omitempty"`
}

func toAnthropicRequest(req llm.Request) anthropicRequest {
	out := anthropicRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    true,
	}
	if out.MaxTokens == 0 {
		out.MaxTokens = 4096
	}
	if req.Temperature > 0 {
		t := req.Temperature
		out.Temp = &t
	}

	// Extract system messages — Anthropic uses a separate top-level field.
	var sys []string
	for _, m := range req.Messages {
		if m.Role == "system" {
			sys = append(sys, m.Content)
			continue
		}
		out.Messages = append(out.Messages, toAnthropicMessage(m))
	}
	out.System = strings.Join(sys, "\n\n")

	// Tools.
	for _, t := range req.Tools {
		schema := map[string]any{}
		_ = json.Unmarshal(t.Schema, &schema)
		out.Tools = append(out.Tools, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": schema,
		})
	}
	return out
}

func toAnthropicMessage(m llm.Message) anthropicMessage {
	out := anthropicMessage{Role: m.Role}
	if m.Role == "tool" {
		// Anthropic represents tool results inside a "user" role with a
		// tool_result content block.
		out.Role = "user"
		out.Content = []map[string]any{{
			"type":        "tool_result",
			"tool_use_id": m.ToolCallID,
			"content":     m.Content,
		}}
		return out
	}
	if m.Content != "" {
		out.Content = append(out.Content, map[string]any{"type": "text", "text": m.Content})
	}
	for _, tc := range m.ToolCalls {
		var inp any
		_ = json.Unmarshal([]byte(tc.Input), &inp)
		out.Content = append(out.Content, map[string]any{
			"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": inp,
		})
	}
	return out
}
