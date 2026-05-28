// Package mcp is a minimal client for Model Context Protocol stdio servers.
//
// MCP (https://modelcontextprotocol.io) is JSON-RPC 2.0 over stdio. We
// implement just enough of it to:
//
//	1. spawn a server child process
//	2. handshake (initialize)
//	3. list its tools
//	4. invoke tools and surface results as tool.Tool implementations
//
// Anything beyond that — resources, prompts, sampling — is left for future
// work; the wire scaffolding here is the load-bearing piece.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// Server represents one running MCP child process.
type Server struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan rpcResp
	closed  atomic.Bool
}

// ServerConfig configures a stdio-launched MCP server.
type ServerConfig struct {
	Name    string   // display name; used as tool-name prefix
	Command string   // executable, e.g. "uvx"
	Args    []string // args, e.g. ["mcp-server-fetch"]
	Env     []string // extra env vars in KEY=VALUE form
}

// Connect launches the server and performs the MCP initialize handshake.
func Connect(ctx context.Context, cfg ServerConfig) (*Server, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = append(cmd.Env, cfg.Env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, aierrs.NewFatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, aierrs.NewFatal(err)
	}
	if err := cmd.Start(); err != nil {
		return nil, aierrs.NewTransient(err)
	}
	s := &Server{
		name:    cfg.Name,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: map[int64]chan rpcResp{},
	}
	go s.readLoop()

	// Handshake.
	_, err = s.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "aievo-next", "version": "0.1"},
	})
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("mcp init %s: %w", cfg.Name, err)
	}
	// Notification — no response expected.
	_ = s.notify("notifications/initialized", nil)
	return s, nil
}

// Tools lists the server's tools and returns them as tool.Tool wrappers.
func (s *Server) Tools(ctx context.Context) ([]tool.Tool, error) {
	raw, err := s.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var r struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, aierrs.NewFatal(err)
	}
	out := make([]tool.Tool, 0, len(r.Tools))
	for _, t := range r.Tools {
		out = append(out, &mcpTool{
			server: s,
			name:   s.name + "__" + t.Name,
			remote: t.Name,
			desc:   t.Description,
			schema: []byte(t.InputSchema),
		})
	}
	return out, nil
}

// Close terminates the child. Safe to call multiple times.
func (s *Server) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	_ = s.stdin.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.cmd.Wait()
}

// ----- JSON-RPC plumbing -----

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}
type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}
type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcErr) Error() string { return fmt.Sprintf("rpc %d: %s", e.Code, e.Message) }

func (s *Server) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&s.nextID, 1)
	ch := make(chan rpcResp, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	if err := s.write(rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, aierrs.NewToolError(s.name, resp.Error, false)
		}
		return resp.Result, nil
	}
}

func (s *Server) notify(method string, params any) error {
	return s.write(rpcReq{JSONRPC: "2.0", Method: method, Params: params})
}

func (s *Server) write(r rpcReq) error {
	b, err := json.Marshal(r)
	if err != nil {
		return aierrs.NewFatal(err)
	}
	b = append(b, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.stdin.Write(b); err != nil {
		return aierrs.NewTransient(err)
	}
	return nil
}

func (s *Server) readLoop() {
	for {
		line, err := s.stdout.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			return
		}
		var resp rpcResp
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		s.mu.Lock()
		ch, ok := s.pending[resp.ID]
		s.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

// ----- mcpTool: tool.Tool wrapping an MCP tool -----

type mcpTool struct {
	server *Server
	name   string // exposed name (prefixed)
	remote string // server-side name
	desc   string
	schema []byte
}

func (t *mcpTool) Name() string             { return t.name }
func (t *mcpTool) Description() string      { return t.desc }
func (t *mcpTool) Schema() []byte           { return t.schema }
func (t *mcpTool) IsConcurrencySafe() bool  { return false /* unknown — assume unsafe */ }
func (t *mcpTool) Call(ctx context.Context, input string) (string, error) {
	var args any
	_ = json.Unmarshal([]byte(input), &args)
	raw, err := t.server.call(ctx, "tools/call", map[string]any{
		"name":      t.remote,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	// MCP returns a structured response; flatten to a single text blob.
	var r struct {
		Content []struct {
			Type, Text string
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return string(raw), nil
	}
	var out string
	for _, c := range r.Content {
		out += c.Text
	}
	if r.IsError {
		return out, aierrs.NewToolError(t.name, fmt.Errorf("server returned isError=true"), false)
	}
	return out, nil
}
