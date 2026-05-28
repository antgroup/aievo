// Package env provides a shared environment for multiple agents: a message
// bus, a memory buffer with optional persistence backends, and per-agent
// inboxes.
//
// Design notes:
//   - Memory is an interface, not a struct. aievo's memory/buffer.go is a
//     concrete in-process slice; persistent backends (file / SQLite) plug
//     in via the same interface.
//   - We separate "inbox" from "memory": Inbox is a queue of unconsumed
//     messages targeted at one agent, Memory is the full historical log.
//   - Read/write is goroutine-safe — many agents may consume in parallel.
package env

import (
	"context"
	"sync"

	"github.com/samson-samson/aievo-next/pkg/event"
)

// Environment is the shared scratch-space for a multi-agent run.
type Environment struct {
	Bus    *event.Bus
	Memory Memory
}

// New constructs an Environment with the default in-memory Memory.
func New() *Environment {
	return &Environment{
		Bus:    event.NewBus(),
		Memory: NewInMemory(),
	}
}

// Close releases all resources. Safe to call multiple times.
func (e *Environment) Close() {
	e.Bus.Close()
	if c, ok := e.Memory.(interface{ Close() error }); ok {
		_ = c.Close()
	}
}

// Memory is the persistence contract for inter-agent message history.
type Memory interface {
	// Append stores a new message and returns its monotonically increasing index.
	Append(ctx context.Context, m event.MessageEvent) (int, error)
	// Load returns messages matching filter. nil filter = all.
	Load(ctx context.Context, f Filter) ([]event.MessageEvent, error)
	// Len returns the number of stored messages.
	Len(ctx context.Context) (int, error)
}

// Filter narrows a Memory.Load query.
type Filter struct {
	Sender    string   // empty = any
	Receiver  string   // empty = any. Matches messages where Receivers is empty
	                   // (broadcast) or contains this name.
	After     int      // index strictly greater than this; -1 = no bound
}

// InMemory is the default Memory: an in-process slice protected by a mutex.
// Sufficient for short-lived runs; for long-running production use a
// persistent backend (file, SQLite, BadgerDB).
type InMemory struct {
	mu   sync.RWMutex
	msgs []event.MessageEvent
}

func NewInMemory() *InMemory { return &InMemory{} }

func (m *InMemory) Append(ctx context.Context, msg event.MessageEvent) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, msg)
	return len(m.msgs) - 1, nil
}

func (m *InMemory) Load(ctx context.Context, f Filter) ([]event.MessageEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]event.MessageEvent, 0, len(m.msgs))
	for i, msg := range m.msgs {
		if f.After >= 0 && i <= f.After {
			continue
		}
		if f.Sender != "" && msg.Sender != f.Sender {
			continue
		}
		if f.Receiver != "" {
			match := len(msg.Receivers) == 0 // broadcast
			for _, r := range msg.Receivers {
				if r == f.Receiver {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, msg)
	}
	return out, nil
}

func (m *InMemory) Len(ctx context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.msgs), nil
}
