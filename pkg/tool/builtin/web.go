package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
)

// ===== WebFetch =====
// Fetch a URL, optionally extract a CSS-like text snippet, return body.
// Concurrency-safe by default — but the host is a side-effect target,
// so for high-rate use, callers should add their own rate limiter.

type WebFetch struct {
	Client *http.Client // nil → default 30s
}

func (WebFetch) Name() string             { return "WebFetch" }
func (WebFetch) Description() string      { return `Fetch a URL and return the response body (text). Input: {"url":"https://...","max_bytes":50000}.` }
func (WebFetch) IsConcurrencySafe() bool  { return true }
func (WebFetch) Schema() []byte {
	return []byte(`{"type":"object","properties":{"url":{"type":"string"},"max_bytes":{"type":"integer"}},"required":["url"]}`)
}
func (w WebFetch) Call(ctx context.Context, input string) (string, error) {
	var in struct {
		URL      string
		MaxBytes int `json:"max_bytes"`
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("WebFetch: %w", err))
	}
	if _, err := url.Parse(in.URL); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("WebFetch: bad url: %w", err))
	}
	if in.MaxBytes == 0 {
		in.MaxBytes = 200 * 1024 // 200 KB default cap
	}
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return "", aierrs.NewFatal(err)
	}
	req.Header.Set("User-Agent", "aievo-next/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return "", aierrs.NewTransient(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", aierrs.NewLLMError("webfetch", resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(in.MaxBytes)))
	if err != nil {
		return "", aierrs.NewTransient(err)
	}
	return string(body), nil
}

// ===== WebSearch (stub) =====
// Real implementation requires a search backend. We expose the tool so
// agents can plan around it; calls return a friendly stub message.

type WebSearch struct {
	// If Backend is non-nil it is used to fulfil the query; otherwise the
	// tool returns a stub. Keeping Backend pluggable lets callers wire a
	// concrete search API without changing the framework.
	Backend func(ctx context.Context, query string) (string, error)
}

func (WebSearch) Name() string             { return "WebSearch" }
func (WebSearch) Description() string      { return `Search the web. Input: {"query":"..."}. Returns top results as a text list.` }
func (WebSearch) IsConcurrencySafe() bool  { return true }
func (WebSearch) Schema() []byte {
	return []byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
}
func (w WebSearch) Call(ctx context.Context, input string) (string, error) {
	var in struct{ Query string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(err)
	}
	if w.Backend != nil {
		return w.Backend(ctx, in.Query)
	}
	return fmt.Sprintf("(stub) no WebSearch backend configured. Query was: %q", strings.TrimSpace(in.Query)), nil
}
